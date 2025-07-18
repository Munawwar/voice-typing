#!/usr/bin/env python3
"""
NVIDIA Canary Qwen 2.5B Speech-to-Text
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

class SpeechToTextService:
    def __init__(self):
        print("🔄 Loading NVIDIA Canary Qwen 2.5B model...")
        try:
            from nemo.collections.asr.models import ASRModel
            import torch
            
            self.model = ASRModel.from_pretrained('nvidia/canary-qwen-2.5b')
            self.model.eval()
            
            # Check if CUDA is available
            if torch.cuda.is_available():
                device = torch.cuda.get_device_name(0)
                print(f"✅ Model loaded successfully on GPU: {device}")
            else:
                print("✅ Model loaded successfully on CPU")
                
        except ImportError as e:
            print(f"❌ Error importing NeMo: {e}")
            print("Make sure you've run the installation script")
            sys.exit(1)
        except Exception as e:
            print(f"❌ Error loading model: {e}")
            sys.exit(1)
        
        # Audio settings for 16kHz mono (required by model)
        self.chunk = 1024
        self.format = pyaudio.paInt16
        self.channels = 1
        self.rate = 16000
        self.recording = False
        
    def record_audio(self, duration=None):
        """Record audio from microphone"""
        try:
            audio = pyaudio.PyAudio()
        except Exception as e:
            print(f"❌ Error initializing audio: {e}")
            print("Make sure pulseaudio is running and microphone is available")
            return None
        
        try:
            stream = audio.open(
                format=self.format,
                channels=self.channels,
                rate=self.rate,
                input=True,
                frames_per_buffer=self.chunk
            )
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
                wf.setframerate(self.rate)
                wf.writeframes(b''.join(frames))
            
            return temp_file.name
        except Exception as e:
            print(f"❌ Error saving audio: {e}")
            return None
    
    def transcribe_audio(self, audio_file):
        """Transcribe audio using Canary model"""
        if not audio_file or not os.path.exists(audio_file):
            print("❌ Invalid audio file")
            return ""
        
        try:
            print("🔄 Transcribing audio...")
            
            # Generate transcription using the model
            answer_ids = self.model.generate(
                prompts=[
                    [{
                        "role": "user", 
                        "content": f"Transcribe the following: {self.model.audio_locator_tag}", 
                        "audio": [audio_file]
                    }]
                ],
                max_new_tokens=128,
            )
            
            # Convert to text
            transcription = self.model.tokenizer.ids_to_text(answer_ids[0].cpu())
            return transcription.strip()
            
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
    
    def start_continuous_mode(self):
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
    parser = argparse.ArgumentParser(description='NVIDIA Canary Speech-to-Text')
    parser.add_argument('--file', '-f', help='Transcribe audio file')
    parser.add_argument('--continuous', '-c', action='store_true', 
                       help='Start continuous speech recognition')
    parser.add_argument('--duration', '-d', type=int, default=5,
                       help='Recording duration in seconds (default: 5)')
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
        # Start continuous mode
        try:
            stt_service.start_continuous_mode()
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