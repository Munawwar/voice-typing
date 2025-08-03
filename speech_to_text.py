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
        
        # Track last complete sentence for correction commands
        self.last_sentence = ""
        
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
                keyterm=["UNDO", "THAT", "WORD", "WORDS", "LAST", "CORRECT", "WITH", "NEWLINE", "NEW", "LINE", "PARA", "PARAGRAPH", "LITERAL", "LITERALLY", "END", "VOICE", "RECORDING", "STOP"],
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
        
        # Check for undo word commands (undo last X words, undo word)
        if self.is_undo_words_command(sentence_lower):
            word_count = self.extract_word_count_from_undo(sentence_lower)
            self.handle_undo_words_command(word_count)
            return None
        
        # Check for correct X with Y command
        if self.is_correct_command(sentence_lower):
            old_word, new_word = self.extract_correction_words(sentence_lower)
            if old_word and new_word:
                self.handle_correct_command(old_word, new_word)
                return None
        
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
        
        # No command detected, store as last sentence and return as-is
        self.last_sentence = sentence
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
    
    def is_undo_words_command(self, sentence_lower):
        """Check if sentence contains undo words command"""
        return ("undo word" in sentence_lower or 
                "undo last" in sentence_lower and "word" in sentence_lower)
    
    def extract_word_count_from_undo(self, sentence_lower):
        """Extract word count from undo command"""
        if "undo word" in sentence_lower and "undo last" not in sentence_lower:
            return 1
        
        # Look for patterns like "undo last 3 words", "undo last three words"
        import re
        
        # Number patterns
        number_match = re.search(r'undo last (\d+) word', sentence_lower)
        if number_match:
            return int(number_match.group(1))
        
        # Written number patterns
        word_numbers = {
            'one': 1, 'two': 2, 'three': 3, 'four': 4, 'five': 5,
            'six': 6, 'seven': 7, 'eight': 8, 'nine': 9, 'ten': 10
        }
        
        for word_num, count in word_numbers.items():
            if f"undo last {word_num} word" in sentence_lower:
                return count
        
        return 1  # Default to 1 word
    
    def is_correct_command(self, sentence_lower):
        """Check if sentence contains correct X with Y command"""
        return "correct" in sentence_lower and "with" in sentence_lower
    
    def extract_correction_words(self, sentence_lower):
        """Extract old and new words from correct command"""
        import re
        
        # Pattern: "correct X with Y" - capture everything after "with" up to end or punctuation
        match = re.search(r'correct\s+(.+?)\s+with\s+(.+?)(?:\s*[.!?]*\s*$|$)', sentence_lower)
        if match:
            old_word = match.group(1).strip()
            new_word = match.group(2).strip()
            
            # Remove trailing punctuation from the new word (in case "correct." was heard with a trailing period)
            new_word = re.sub(r'[.!?]+$', '', new_word).strip()
            
            return old_word, new_word
        
        return None, None
    
    def handle_undo_words_command(self, word_count):
        """Handle undo words command using cursor navigation"""
        print(f"🗑️ Undo {word_count} word(s) command detected")
        
        if self.real_time_typing:
            # Use Ctrl+Shift+Left Arrow to select words, then Backspace
            for _ in range(word_count):
                self.type_key_combination(['ctrl', 'shift', 'Left'])
            self.type_key_combination(['BackSpace'])
    
    def handle_correct_command(self, old_word, new_word):
        """Handle correct X with Y command by replacing last transcription part"""
        print(f"🔄 Correct '{old_word}' with '{new_word}' command detected")
        
        if not self.transcription_parts:
            print("❌ No previous transcription to correct")
            return
        
        # Get the last transcription part (which should be the last sentence/text)
        last_part = self.transcription_parts[-1]
        
        # Skip if it's just formatting (newlines)
        if last_part in ["\n", "\n\n"]:
            print("❌ Cannot correct formatting commands")
            return
        
        # Create corrected text by replacing in the last transcription part
        corrected_text = last_part.replace(old_word, new_word)
        corrected_text = corrected_text.replace(old_word.capitalize(), new_word.capitalize())
        
        if self.real_time_typing:
            # Remove the last transcription part by typing backspaces
            self.type_backspaces(len(last_part))
            # Type the corrected text
            self.type_text(corrected_text)
        
        # Update the transcription_parts with corrected text
        self.transcription_parts[-1] = corrected_text
        
        # Rebuild current text
        text_parts = [part for part in self.transcription_parts if part not in ["\n", "\n\n"]]
        self.current_text = "".join(text_parts)
        
        # Update last sentence for potential future corrections
        self.last_sentence = corrected_text.strip()
    
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
        
        # Define proper key mappings for each tool based on official documentation
        KEY_MAPPINGS = {
            'xdotool': {
                # xdotool uses X11 keysym names
                'BackSpace': 'BackSpace',
                'Return': 'Return', 
                'shift': 'shift',  # alias for Shift_L
            },
            'ydotool': {
                # ydotool uses Linux kernel input event codes
                'BackSpace': 'Backspace',  # note lowercase 's'
                'Return': 'enter',         # ydotool uses 'enter' not 'return'
                'shift': 'shift',
            },
            'wtype': {
                # wtype has limited key combination support
                'BackSpace': 'backspace',  # named key in libxkbcommon
                'Return': 'enter',
                'shift': 'shift',
            }
        }
        
        def build_command_for_tool(keys, tool):
            """Build the appropriate command for each tool"""
            mapping = KEY_MAPPINGS.get(tool, {})
            
            if tool == 'xdotool':
                # xdotool: key combination with +
                mapped_keys = [mapping.get(k, k) for k in keys]
                return ['xdotool', 'key', '+'.join(mapped_keys)]
                
            elif tool == 'ydotool':
                # ydotool: key combination with +
                mapped_keys = [mapping.get(k, k) for k in keys]
                return ['ydotool', 'key', '+'.join(mapped_keys)]
                
            elif tool == 'wtype':
                # wtype: special handling required
                return self._build_wtype_command(keys, mapping)
            
            return None
        
        if display_server == 'wayland':
            # Wayland: try ydotool first, then wtype, then xdotool as fallback
            tools = ['ydotool', 'wtype', 'xdotool']
        else:
            # X11: try xdotool first, then others as fallback
            tools = ['xdotool', 'ydotool', 'wtype']
        
        methods = []
        for tool in tools:
            cmd = build_command_for_tool(keys, tool)
            if cmd:
                if callable(cmd):
                    # wtype returns a function
                    methods.append((tool, cmd))
                else:
                    # other tools return command arrays
                    methods.append((tool, lambda c=cmd: subprocess.run(c, check=True, capture_output=True)))
        
        # Try each method until one works
        key_combo_str = '+'.join(keys)
        for method_name, method_func in methods:
            try:
                method_func()
                print(f"✅ Typed key combo via {method_name}: {key_combo_str}")
                return
            except (subprocess.CalledProcessError, FileNotFoundError):
                continue
        
        print(f"❌ Failed to type key combination: {key_combo_str}")
    
    def _build_wtype_command(self, keys, mapping):
        """Build wtype command - has limited key combination support"""
        if len(keys) == 1:
            key = keys[0]
            if key == 'BackSpace':
                # wtype can send backspace character
                return lambda: subprocess.run(['wtype', '-P', 'backspace', '-p', 'backspace'], check=True, capture_output=True)
            elif key == 'Return':
                # wtype can send enter
                return lambda: subprocess.run(['wtype', '-P', 'enter', '-p', 'enter'], check=True, capture_output=True)
        elif keys == ['shift', 'Return']:
            # wtype doesn't support shift+enter well, just send enter
            return lambda: subprocess.run(['wtype', '-P', 'enter', '-p', 'enter'], check=True, capture_output=True)
        
        # wtype doesn't support this combination
        return None
        
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
            
            # Show ready notification (for hotkey mode)
            if real_time_typing:
                try:
                    import subprocess
                    subprocess.run([
                        'notify-send', 
                        'Voice Typing Ready!', 
                        'Focus on a text field and start talking. Say "stop voice" or press Super+] again to stop.',
                        '--icon=audio-input-microphone',
                        '--expire-time=5000'
                    ], capture_output=True)
                except:
                    pass
            
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
            
            # Show stop notification
            try:
                import subprocess
                subprocess.run([
                    'notify-send', 
                    'Voice Typing Stopped', 
                    'Recording ended.',
                    '--icon=audio-input-microphone-muted',
                    '--expire-time=3000'
                ], capture_output=True)
            except:
                pass
            
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
                ('xdotool', lambda: subprocess.run(['xdotool', 'type', '--delay', '50', text], check=True, capture_output=True))
            ]
        else:
            methods = [
                ('xdotool', lambda: subprocess.run(['xdotool', 'type', '--delay', '50', text], check=True, capture_output=True)),
                ('wtype', lambda: subprocess.run(['wtype', text], check=True, capture_output=True)),
                ('ydotool', lambda: subprocess.run(['ydotool', 'type', text], check=True, capture_output=True))
            ]
        
        # Try each method until one works
        for method_name, method_func in methods:
            try:
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
    
    
    def _suggest_typing_tools(self):
        """Suggest installation of typing tools based on display server"""
        display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
        
        print(f"\n🔧 Install typing tools for {display_server}:")
        
        if display_server == 'wayland':
            print("Primary (Wayland):")
            print("  sudo apt install wtype ydotool")
            print("  sudo systemctl enable --now ydotoold")
            print("  sudo usermod -a -G input $USER")
            print("\nFallback (X11 compatibility):")
            print("  sudo apt install xdotool")
        else:
            print("Primary (X11):")
            print("  sudo apt install xdotool")
            print("\nFallback (Wayland compatibility):")
            print("  sudo apt install wtype ydotool")
    
    async def start_continuous_mode(self, no_type=False):
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
            if not args.no_type:
                stt_service.type_text(transcription)
        else:
            print("❌ Transcription failed")
        
    elif args.continuous:
        # Start continuous streaming mode
        try:
            await stt_service.start_continuous_mode(
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
            if not args.no_type:
                stt_service.type_text(transcription)
        else:
            print("❌ No speech detected")

if __name__ == "__main__":
    asyncio.run(main())