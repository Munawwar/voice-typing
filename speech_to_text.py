#!/usr/bin/env python3
"""
Groq Whisper Large v3 Turbo Speech-to-Text
Supports both X11 and Wayland with automatic detection
"""

import argparse
import soundfile as sf
import tempfile
import subprocess
import threading
import time
import pyaudio
import wave
import os
import sys
from pathlib import Path
from groq import Groq

class SpeechToTextService:
    def __init__(self):
        print("🔄 Initializing Groq Whisper Large v3 Turbo...")
        
        # Get API key from environment
        api_key = os.getenv('GROQ_API_KEY')
        if not api_key:
            print("❌ GROQ_API_KEY environment variable not set!")
            print("Please set your Groq API key:")
            print("export GROQ_API_KEY='your_api_key_here'")
            sys.exit(1)
        
        try:
            self.client = Groq(api_key=api_key)
            print("✅ Groq client initialized successfully")
        except Exception as e:
            print(f"❌ Error initializing Groq client: {e}")
            sys.exit(1)
        
        # Audio settings
        self.chunk = 1024
        self.format = pyaudio.paInt16
        self.channels = 1
        self.preferred_rate = 16000  # Whisper models prefer 16kHz
        self.recording = False
        
    def find_input_device(self, audio):
        """Find the best input device"""
        print("🔍 Looking for microphone...")
        
        # Get device count
        device_count = audio.get_device_count()
        input_devices = []
        
        for i in range(device_count):
            try:
                device_info = audio.get_device_info_by_index(i)
                if device_info['maxInputChannels'] > 0:
                    input_devices.append((i, device_info))
                    print(f"  📱 Device {i}: {device_info['name']} ({device_info['maxInputChannels']} channels)")
            except:
                continue
        
        if not input_devices:
            print("❌ No input devices found")
            return None
        
        # Prefer USB/headset devices, then default
        for device_id, device_info in input_devices:
            name = device_info['name'].lower()
            if 'usb' in name or 'headset' in name or 'jabra' in name or 'evolve' in name:
                print(f"✅ Selected: {device_info['name']}")
                return device_id, device_info
        
        # Try the default device
        try:
            default_device = audio.get_default_input_device_info()
            print(f"✅ Using default: {default_device['name']}")
            return default_device['index'], default_device
        except:
            # Use first available device
            device_id, device_info = input_devices[0]
            print(f"✅ Using first available: {device_info['name']}")
            return device_id, device_info

    def record_audio(self, duration=None):
        """Record audio from microphone"""
        try:
            audio = pyaudio.PyAudio()
        except Exception as e:
            print(f"❌ Error initializing audio: {e}")
            print("Make sure pulseaudio/pipewire is running and microphone is available")
            return None
        
        # Find the best input device
        device_result = self.find_input_device(audio)
        if device_result is None:
            audio.terminate()
            return None
        
        device_id, device_info = device_result
        
        # Use device's native sample rate
        device_rate = int(device_info['defaultSampleRate'])
        if device_rate != self.preferred_rate:
            print(f"ℹ️  Device supports {device_rate}Hz, will resample to {self.preferred_rate}Hz")
        
        try:
            stream = audio.open(
                format=self.format,
                channels=self.channels,
                rate=device_rate,
                input=True,
                input_device_index=device_id,
                frames_per_buffer=self.chunk
            )
            
            self.actual_rate = device_rate
            
        except Exception as e:
            print(f"❌ Error opening audio stream: {e}")
            print("Check microphone permissions and availability")
            audio.terminate()
            return None
        
        if duration is None:
            print("🎤 Recording... (Press Ctrl+C to stop)")
        else:
            print(f"🎤 Recording for {duration} seconds...")
        
        frames = []
        start_time = time.time()
        
        try:
            while self.recording or (duration and time.time() - start_time < duration):
                data = stream.read(self.chunk, exception_on_overflow=False)
                frames.append(data)
                
                if duration and time.time() - start_time >= duration:
                    break
                    
        except KeyboardInterrupt:
            print("\n⏹️  Stopping recording...")
        except Exception as e:
            print(f"❌ Error during recording: {e}")
        
        stream.stop_stream()
        stream.close()
        audio.terminate()
        
        if not frames:
            print("❌ No audio recorded")
            return None
        
        # Save to temporary file
        temp_file = tempfile.NamedTemporaryFile(suffix='.wav', delete=False)
        try:
            with wave.open(temp_file.name, 'wb') as wf:
                wf.setnchannels(self.channels)
                wf.setsampwidth(pyaudio.get_sample_size(self.format))
                wf.setframerate(self.actual_rate)
                wf.writeframes(b''.join(frames))
            
            return temp_file.name
        except Exception as e:
            print(f"❌ Error saving audio: {e}")
            return None
    
    def transcribe_audio(self, audio_file):
        """Transcribe audio using Groq Whisper Large v3 Turbo"""
        if not audio_file or not os.path.exists(audio_file):
            print("❌ Invalid audio file")
            return ""
        
        try:
            print("🔄 Transcribing audio with Groq Whisper Large v3 Turbo...")
            
            # Open and transcribe the audio file
            with open(audio_file, "rb") as file:
                transcription = self.client.audio.transcriptions.create(
                    file=file,
                    model="whisper-large-v3-turbo",
                    response_format="json",
                    temperature=0.0  # Deterministic output
                )
            
            # Extract text from response
            if hasattr(transcription, 'text'):
                text = transcription.text.strip()
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
    
    def start_continuous_mode(self, copy_to_clipboard=False, no_type=False):
        """Start continuous speech recognition"""
        print("🎤 Starting continuous mode...")
        print("Press Enter to start recording, then Enter again to stop and transcribe")
        print("Type 'q' and Enter to quit")
        
        while True:
            user_input = input("\nPress Enter to record (or 'q' to quit): ").strip().lower()
            if user_input == 'q':
                break
            
            # Start recording
            self.recording = True
            print("🎤 Recording... Press Enter to stop")
            
            # Start recording in background
            record_thread = threading.Thread(target=self._background_record)
            record_thread.start()
            
            # Wait for user to stop
            input()  # Wait for Enter
            self.recording = False
            record_thread.join()
            
            if hasattr(self, '_last_audio_file') and self._last_audio_file:
                transcription = self.transcribe_audio(self._last_audio_file)
                if transcription:
                    print(f"📝 Transcribed: {transcription}")
                    if copy_to_clipboard:
                        self.copy_to_clipboard(transcription)
                    if not no_type:
                        self.type_text(transcription)
                else:
                    print("❌ No speech detected")
                
                # Clean up
                try:
                    os.unlink(self._last_audio_file)
                except:
                    pass
    
    def _background_record(self):
        """Background recording for continuous mode"""
        self._last_audio_file = self.record_audio()

def main():
    parser = argparse.ArgumentParser(description='Groq Whisper Large v3 Turbo Speech-to-Text')
    parser.add_argument('--file', '-f', help='Transcribe audio file')
    parser.add_argument('--continuous', '-c', action='store_true', 
                       help='Start continuous speech recognition')
    parser.add_argument('--duration', '-d', type=int, default=5,
                       help='Recording duration in seconds (default: 5)')
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
        # Start continuous mode
        try:
            stt_service.start_continuous_mode(
                copy_to_clipboard=args.copy_to_clipboard,
                no_type=args.no_type
            )
        except KeyboardInterrupt:
            print("\n👋 Goodbye!")
        
    else:
        # Single recording
        print(f"🎤 Recording for {args.duration} seconds...")
        time.sleep(1)  # Give user time to prepare
        
        stt_service.recording = True
        audio_file = stt_service.record_audio(duration=args.duration)
        stt_service.recording = False
        
        if audio_file:
            transcription = stt_service.transcribe_audio(audio_file)
            if transcription:
                print(f"📝 Transcription: {transcription}")
                if args.copy_to_clipboard:
                    stt_service.copy_to_clipboard(transcription)
                if not args.no_type:
                    stt_service.type_text(transcription)
            else:
                print("❌ No speech detected")
            
            # Clean up
            try:
                os.unlink(audio_file)
            except:
                pass
        else:
            print("❌ Recording failed")

if __name__ == "__main__":
    main()