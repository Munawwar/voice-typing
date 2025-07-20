#!/usr/bin/env python3
"""
Deepgram Streaming Speech-to-Text
Supports both X11 and Wayland with automatic detection
Real-time streaming transcription with keyword detection
"""

import argparse
import tempfile
import subprocess
import threading
import time
import os
import sys
import asyncio
from pathlib import Path
from dotenv import load_dotenv
from deepgram import (
    DeepgramClient,
    LiveTranscriptionEvents,
    LiveOptions,
    Microphone
)

load_dotenv()

class SpeechToTextService:
    def __init__(self):
        print("🔄 Initializing Deepgram Streaming Speech-to-Text...")
        
        # Get API key from environment
        api_key = os.getenv('DEEPGRAM_API_KEY')
        if not api_key:
            print("❌ DEEPGRAM_API_KEY environment variable not set!")
            print("Please set your Deepgram API key:")
            print("export DEEPGRAM_API_KEY='your_api_key_here'")
            sys.exit(1)
        
        try:
            self.deepgram = DeepgramClient(api_key)
            print("✅ Deepgram client initialized successfully")
        except Exception as e:
            print(f"❌ Error initializing Deepgram client: {e}")
            sys.exit(1)
        
        # Streaming state
        self.recording = False
        self.transcription_parts = []
        self.current_text = ""
        self.microphone = None
        self.dg_connection = None
        self.real_time_typing = False  # Flag to control real-time typing
        self.stop_requested = False  # Flag to stop recording via voice command
        self.last_was_line_break = False  # Track if last action was any line break
        
        # Keywords for special commands
        self.delete_keywords = ["undo that"]
        self.newline_keywords = ["newline", "new line"]
        self.paragraph_keywords = ["next para", "new para", "next paragraph", "new paragraph"]
        self.stop_keywords = ["end voice", "end recording", "stop recording", "stop voice"]
        self.escape_keywords = ["literal", "literally"]
        
    def setup_live_transcription(self):
        """Setup live transcription connection"""
        try:
            # Create connection
            self.dg_connection = self.deepgram.listen.websocket.v("1")
            
            # Define event handlers (using service_self to avoid conflict)
            service_self = self
            
            def on_open(dg_self, open, **kwargs):
                print("🔗 Connection opened")

            def on_message(dg_self, result, **kwargs):
                sentence = result.channel.alternatives[0].transcript
                if sentence:
                    if result.is_final:
                        print(f"📝 Final: {sentence}")
                        
                        # Check for voice commands BEFORE adding to transcription_parts
                        processed_sentence = service_self.process_voice_commands(sentence)
                        if processed_sentence is not None:
                            # Handle different types of processed sentences
                            if processed_sentence in ["\n", "\n\n"]:
                                # Formatting commands - add as-is to transcription_parts
                                service_self.transcription_parts.append(processed_sentence)
                                service_self.last_was_line_break = True
                            else:
                                # Regular text - add appropriate spacing/formatting
                                space_prefix = " " if service_self.current_text and not service_self.last_was_line_break else ""
                                text_with_spacing = space_prefix + processed_sentence
                                
                                # Add the text WITH spacing to transcription_parts for proper undo
                                service_self.transcription_parts.append(text_with_spacing)
                                service_self.current_text += text_with_spacing
                                service_self.last_was_line_break = False
                                
                                # Type the final sentence immediately for real-time feedback
                                if hasattr(service_self, 'real_time_typing') and service_self.real_time_typing:
                                    service_self.type_text(text_with_spacing)
                    else:
                        print(f"💭 Interim: {sentence}")

            def on_metadata(dg_self, metadata, **kwargs):
                print(f"🔍 Metadata: {metadata}")

            def on_speech_started(dg_self, speech_started, **kwargs):
                print("🎤 Speech started")

            def on_utterance_end(dg_self, utterance_end, **kwargs):
                print("⏹️ Utterance ended")

            def on_close(dg_self, close, **kwargs):
                print("🔚 Connection closed")

            def on_error(dg_self, error, **kwargs):
                print(f"❌ Error: {error}")

            # Register event handlers
            self.dg_connection.on(LiveTranscriptionEvents.Open, on_open)
            self.dg_connection.on(LiveTranscriptionEvents.Transcript, on_message)
            self.dg_connection.on(LiveTranscriptionEvents.Metadata, on_metadata)
            self.dg_connection.on(LiveTranscriptionEvents.SpeechStarted, on_speech_started)
            self.dg_connection.on(LiveTranscriptionEvents.UtteranceEnd, on_utterance_end)
            self.dg_connection.on(LiveTranscriptionEvents.Close, on_close)
            self.dg_connection.on(LiveTranscriptionEvents.Error, on_error)

            # Configure live transcription options (using only supported parameters)
            options = LiveOptions(
                model="nova-3",
                language="en-IN",
                # improves readability
                smart_format=True,
                punctuate=True,
                encoding="linear16",
                channels=1,
                sample_rate=16000,
                interim_results=True,
                utterance_end_ms="1000",
                vad_events=True,
                endpointing=300,
                profanity_filter=True,
                # Remove words like "um" and "uh"
                filler_words=True,
                keyterm=["UNDO", "THAT", "NEWLINE", "NEW", "LINE", "PARA", "PARAGRAPH", "LITERAL", "LITERALLY", "END", "VOICE", "RECORDING", "STOP"],
                # Convert numeric words to numbers
                numerals=True,
                tag="voice-typing"
            )

            # Start connection
            if self.dg_connection.start(options) is False:
                print("❌ Failed to start connection")
                return False
                
            return True
            
        except Exception as e:
            print(f"❌ Error setting up live transcription: {e}")
            return False

    def process_voice_commands(self, sentence):
        """Process voice commands and return modified sentence or None if command executed"""
        sentence_lower = sentence.lower().strip()
        
        # Check for escape keywords - if present, treat as literal text
        if any(escape in sentence_lower for escape in self.escape_keywords):
            print(f"🔤 Literal text detected: {sentence}")
            # Remove the escape word and return the rest as literal text
            # for escape in self.escape_keywords:
            #     sentence = sentence.replace(escape, "").replace(escape.capitalize(), "").strip()
            return sentence
        
        # Check for delete command
        if any(keyword in sentence_lower for keyword in self.delete_keywords):
            self.handle_delete_command()
            return None  # Don't add this sentence to text
        
        # Check for newline command
        if any(keyword in sentence_lower for keyword in self.newline_keywords):
            self.handle_newline_command()
            return "\n"  # Add newline marker to transcription_parts for undo tracking
        
        # Check for paragraph command
        if any(keyword in sentence_lower for keyword in self.paragraph_keywords):
            self.handle_paragraph_command()
            return "\n\n"  # Add paragraph marker to transcription_parts for undo tracking
        
        # Check for stop recording command
        if any(keyword in sentence_lower for keyword in self.stop_keywords):
            self.handle_stop_command()
            return None  # Don't add this sentence to text
        
        # No command detected, return sentence as-is
        return sentence
    
    def handle_delete_command(self):
        """Handle delete commands to remove words"""
        print(f"🗑️ Delete command detected")
        
        # Simple implementation: remove the last sentence/word
        if self.transcription_parts:
            removed = self.transcription_parts.pop()
            print(f"🗑️ Removed: {removed}")
            
            # Handle different types of removals
            if removed == "\n":
                # Removing a newline command
                if self.real_time_typing:
                    self.type_key_combination(['BackSpace'])  # Remove the newline
            elif removed == "\n\n":
                # Removing a paragraph command (double newline)
                if self.real_time_typing:
                    self.type_key_combination(['BackSpace'])  # Remove first newline
                    self.type_key_combination(['BackSpace'])  # Remove second newline
            else:
                # Removing regular text (with its spacing already included)
                if self.real_time_typing:
                    self.type_backspaces(len(removed))  # No +1 needed, spacing already included
            
            # Rebuild current text (filter out newline markers)
            text_parts = [part for part in self.transcription_parts if part not in ["\n", "\n\n"]]
            self.current_text = "".join(text_parts)  # Use join without separator since spacing is already included
    
    def handle_newline_command(self):
        """Handle newline command to add a single line break"""
        print(f"📝 Newline command detected")
        if self.real_time_typing:
            self.type_newline()
    
    def handle_paragraph_command(self):
        """Handle paragraph command to add double line break"""
        print(f"📝 Paragraph command detected")
        if self.real_time_typing:
            self.type_paragraph_break()
    
    def handle_stop_command(self):
        """Handle stop recording command"""
        print(f"🛑 Stop command detected - ending recording")
        # Set a flag that can be checked by the streaming loop
        self.stop_requested = True
    
    def type_newline(self):
        """Type a single newline (Shift+Enter)"""
        self.type_key_combination(['shift', 'Return'])
    
    def type_paragraph_break(self):
        """Type double newline for paragraph break"""
        self.type_key_combination(['shift', 'Return'])
        self.type_key_combination(['shift', 'Return'])
    
    def type_backspaces(self, count):
        """Type multiple backspace characters"""
        for _ in range(count):
            self.type_key_combination(['BackSpace'])
    
    def type_key_combination(self, keys):
        """Type a key combination using available tools"""
        display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
        
        # Convert keys to appropriate format for each tool
        key_string = '+'.join(keys)
        
        if display_server == 'wayland':
            methods = [
                ('ydotool', lambda: subprocess.run(['ydotool', 'key'] + [f'{k.lower()}:{1 if k != "shift" else 1}' for k in keys], check=True, capture_output=True)),
                ('wtype', lambda: self._wtype_key_combo(keys)),
            ]
        else:
            methods = [
                ('xdotool', lambda: subprocess.run(['xdotool', 'key', key_string], check=True, capture_output=True)),
            ]
        
        # Try each method until one works
        for method_name, method_func in methods:
            try:
                method_func()
                print(f"✅ Typed key combo via {method_name}: {key_string}")
                return
            except (subprocess.CalledProcessError, FileNotFoundError):
                continue
        
        print(f"❌ Failed to type key combination: {key_string}")
    
    def _wtype_key_combo(self, keys):
        """Handle key combinations for wtype (limited support)"""
        if keys == ['shift', 'Return']:
            # wtype doesn't support shift+enter, so just send enter
            subprocess.run(['wtype', '\n'], check=True)
        elif keys == ['BackSpace']:
            subprocess.run(['wtype', '\b'], check=True)
        else:
            raise subprocess.CalledProcessError(1, 'wtype')
        
    async def start_streaming(self, duration=None, real_time_typing=False):
        """Start streaming transcription"""
        try:
            # Set real-time typing flag and reset stop flag
            self.real_time_typing = real_time_typing
            self.stop_requested = False
            
            # Setup live transcription
            if not self.setup_live_transcription():
                return None
            
            # Setup microphone
            self.microphone = Microphone(self.dg_connection.send)
            
            # Start microphone
            print("🎤 Starting microphone...")
            self.microphone.start()
            
            if duration:
                print(f"🎤 Streaming for {duration} seconds...")
                await asyncio.sleep(duration)
            else:
                print("🎤 Streaming... (Press Ctrl+C or say 'end voice' to stop)")
                try:
                    while not self.stop_requested:
                        await asyncio.sleep(0.1)
                    print("\n⏹️ Stopping streaming via voice command...")
                except KeyboardInterrupt:
                    print("\n⏹️ Stopping streaming...")
            
            # Stop microphone and connection
            self.microphone.finish()
            self.dg_connection.finish()
            
            # Return the final transcription
            final_text = self.current_text.strip()
            return final_text if final_text else None
            
        except Exception as e:
            print(f"❌ Error during streaming: {e}")
            return None
    
    def transcribe_audio(self, audio_file):
        """Transcribe audio file using Deepgram (fallback for file-based transcription)"""
        if not audio_file or not os.path.exists(audio_file):
            print("❌ Invalid audio file")
            return ""
        
        try:
            print("🔄 Transcribing audio file with Deepgram...")
            
            with open(audio_file, "rb") as file:
                buffer_data = file.read()

            response = self.deepgram.listen.prerecorded.v("1").transcribe_file(
                { "buffer": buffer_data },
                {
                    "model": "nova-3",
                    "smart_format": True,
                }
            )
            
            # Extract text from response
            if response.results and response.results.channels:
                text = response.results.channels[0].alternatives[0].transcript.strip()
                print(f"✅ Transcription completed: {len(text)} characters")
                return text
            else:
                print("❌ No text in transcription response")
                return ""
            
        except Exception as e:
            print(f"❌ Error during transcription: {e}")
            return ""
    
    def type_text(self, text):
        """Type text to the currently active window (auto-detects X11/Wayland)"""
        if not text:
            print("❌ No text to type")
            return
        
        display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
        
        # Try methods in order of preference based on display server
        if display_server == 'wayland':
            methods = [
                ('wtype', lambda: subprocess.run(['wtype', text], check=True, capture_output=True)),
                ('ydotool', lambda: subprocess.run(['ydotool', 'type', text], check=True, capture_output=True)),
                ('clipboard', lambda: self._type_via_clipboard(text)),
                ('xdotool', lambda: subprocess.run(['xdotool', 'type', '--delay', '50', text], check=True, capture_output=True))
            ]
        else:
            methods = [
                ('xdotool', lambda: subprocess.run(['xdotool', 'type', '--delay', '50', text], check=True, capture_output=True)),
                ('wtype', lambda: subprocess.run(['wtype', text], check=True, capture_output=True)),
                ('ydotool', lambda: subprocess.run(['ydotool', 'type', text], check=True, capture_output=True)),
                ('clipboard', lambda: self._type_via_clipboard(text))
            ]
        
        # Try each method until one works
        for method_name, method_func in methods:
            try:
                if method_name == 'clipboard':
                    success = method_func()
                    if success:
                        print(f"✅ Typed via {method_name}: {text}")
                        return
                else:
                    method_func()
                    print(f"✅ Typed via {method_name}: {text}")
                    return
            except (subprocess.CalledProcessError, FileNotFoundError):
                continue
        
        # If all methods failed
        print(f"❌ All typing methods failed for {display_server}")
        print("📋 Text copied to console:")
        print(f"'{text}'")
        self._suggest_typing_tools()
    
    def copy_to_clipboard(self, text):
        """Copy text to clipboard without typing it"""
        if not text:
            print("❌ No text to copy")
            return False
        
        display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
        
        # Try clipboard tools in order of preference based on display server
        if display_server == 'wayland':
            clipboard_tools = [
                ['wl-copy'],
                ['xclip', '-selection', 'clipboard'],
                ['xsel', '--clipboard', '--input']
            ]
        else:
            clipboard_tools = [
                ['xclip', '-selection', 'clipboard'],
                ['xsel', '--clipboard', '--input'],
                ['wl-copy']
            ]
        
        for tool in clipboard_tools:
            try:
                # Try to use the tool directly
                subprocess.run(tool, input=text.encode(), check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                print(f"📋 Copied to clipboard via {tool[0]}: {text}")
                return True
            except (subprocess.CalledProcessError, FileNotFoundError):
                continue
        
        print("❌ No working clipboard tool found")
        return False
    
    def _type_via_clipboard(self, text):
        """Type text via clipboard (works on both X11 and Wayland)"""
        try:
            # Detect clipboard tool
            clipboard_copy = None
            
            # Try Wayland clipboard first
            try:
                subprocess.run(['wl-copy', '--version'], check=True, capture_output=True)
                clipboard_copy = ['wl-copy']
            except (subprocess.CalledProcessError, FileNotFoundError):
                pass
            
            # Fallback to X11 clipboard
            if not clipboard_copy:
                try:
                    subprocess.run(['xclip', '-version'], check=True, capture_output=True)
                    clipboard_copy = ['xclip', '-selection', 'clipboard']
                except (subprocess.CalledProcessError, FileNotFoundError):
                    return False
            
            # Copy to clipboard
            subprocess.run(clipboard_copy, input=text.encode(), check=True)
            
            # Small delay
            time.sleep(0.1)
            
            # Paste using appropriate method
            display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
            if display_server == 'wayland':
                subprocess.run(['ydotool', 'key', 'ctrl+v'], check=True)
            else:
                subprocess.run(['xdotool', 'key', 'ctrl+v'], check=True)
            
            return True
            
        except (subprocess.CalledProcessError, FileNotFoundError):
            return False
    
    def _suggest_typing_tools(self):
        """Suggest installation of typing tools based on display server"""
        display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
        
        print(f"\n🔧 Install typing tools for {display_server}:")
        
        if display_server == 'wayland':
            print("Primary (Wayland):")
            print("  sudo apt install wtype ydotool wl-clipboard")
            print("  sudo systemctl enable --now ydotoold")
            print("  sudo usermod -a -G input $USER")
            print("\nFallback (X11 compatibility):")
            print("  sudo apt install xdotool xclip")
        else:
            print("Primary (X11):")
            print("  sudo apt install xdotool xclip")
            print("\nFallback (Wayland compatibility):")
            print("  sudo apt install wtype ydotool wl-clipboard")
    
    async def start_continuous_mode(self, copy_to_clipboard=False, no_type=False):
        """Start continuous streaming speech recognition"""
        print("🎤 Starting continuous streaming mode...")
        print("Press Enter to start streaming, then Enter again to stop and process")
        print("Type 'q' and Enter to quit")
        print("Say 'delete' to remove the last transcribed segment")
        
        while True:
            user_input = input("\nPress Enter to stream (or 'q' to quit): ").strip().lower()
            if user_input == 'q':
                break
            
            # Reset transcription state
            self.transcription_parts = []
            self.current_text = ""
            
            print("🎤 Streaming... Press Enter to stop")
            
            # Start streaming in background
            stream_task = asyncio.create_task(self.start_streaming())
            
            # Wait for user to stop (in a separate thread to not block async)
            def wait_for_input():
                input()
                
            await asyncio.get_event_loop().run_in_executor(None, wait_for_input)
            
            # Cancel streaming
            stream_task.cancel()
            
            # Process final transcription
            if self.current_text:
                transcription = self.current_text.strip()
                print(f"📝 Final transcription: {transcription}")
                if copy_to_clipboard:
                    self.copy_to_clipboard(transcription)
                if not no_type:
                    self.type_text(transcription)
            else:
                print("❌ No speech detected")

async def main():
    parser = argparse.ArgumentParser(description='Deepgram Streaming Speech-to-Text')
    parser.add_argument('--file', '-f', help='Transcribe audio file')
    parser.add_argument('--continuous', '-c', action='store_true', 
                       help='Start continuous streaming speech recognition')
    parser.add_argument('--duration', '-d', type=int, default=5,
                       help='Streaming duration in seconds (default: 5)')
    parser.add_argument('--no-type', action='store_true',
                       help='Don\'t type the result, just print it')
    parser.add_argument('--copy-to-clipboard', action='store_true',
                       help='Copy transcribed text to clipboard')
    
    args = parser.parse_args()
    
    # Initialize service
    try:
        stt_service = SpeechToTextService()
    except Exception as e:
        print(f"❌ Failed to initialize service: {e}")
        sys.exit(1)
    
    if args.file:
        # Transcribe file
        if not os.path.exists(args.file):
            print(f"❌ File not found: {args.file}")
            sys.exit(1)
        
        transcription = stt_service.transcribe_audio(args.file)
        if transcription:
            print(f"📝 Transcription: {transcription}")
            if args.copy_to_clipboard:
                stt_service.copy_to_clipboard(transcription)
            if not args.no_type:
                stt_service.type_text(transcription)
        else:
            print("❌ Transcription failed")
        
    elif args.continuous:
        # Start continuous streaming mode
        try:
            await stt_service.start_continuous_mode(
                copy_to_clipboard=args.copy_to_clipboard,
                no_type=args.no_type
            )
        except KeyboardInterrupt:
            print("\n👋 Goodbye!")
        
    else:
        # Single streaming session
        print(f"🎤 Streaming for {args.duration} seconds...")
        time.sleep(1)  # Give user time to prepare
        
        transcription = await stt_service.start_streaming(duration=args.duration, real_time_typing=not args.no_type)
        
        if transcription:
            print(f"📝 Transcription: {transcription}")
            if args.copy_to_clipboard:
                stt_service.copy_to_clipboard(transcription)
            if not args.no_type:
                stt_service.type_text(transcription)
        else:
            print("❌ No speech detected")

if __name__ == "__main__":
    asyncio.run(main())