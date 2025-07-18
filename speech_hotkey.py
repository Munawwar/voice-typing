#!/usr/bin/env python3
"""
System-wide hotkey service for speech-to-text
Listens for Super+Space and toggles speech recognition
"""

import subprocess
import time
import threading
import tempfile
import os
import sys
import signal
from pathlib import Path

# Import the speech service
from speech_to_text import SpeechToTextService

class HotkeyService:
    def __init__(self):
        print("🔄 Initializing speech-to-text service...")
        try:
            self.stt_service = SpeechToTextService()
        except Exception as e:
            print(f"❌ Failed to initialize speech service: {e}")
            sys.exit(1)
        
        self.recording = False
        self.current_keys = set()
        self.audio_file = None
        
        # Setup signal handlers for clean shutdown
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
        
    def _signal_handler(self, signum, frame):
        """Handle shutdown signals"""
        print("\n🛑 Shutting down...")
        if self.audio_file and os.path.exists(self.audio_file):
            try:
                os.unlink(self.audio_file)
            except:
                pass
        sys.exit(0)
        
    def on_press(self, key):
        """Handle key press events"""
        try:
            # Import here to avoid issues if pynput not available
            from pynput import keyboard
            
            self.current_keys.add(key)
            
            # Check for Super+Space combination
            super_pressed = (keyboard.Key.cmd in self.current_keys or 
                           keyboard.Key.cmd_l in self.current_keys or 
                           keyboard.Key.cmd_r in self.current_keys)
            space_pressed = keyboard.Key.space in self.current_keys
            
            if super_pressed and space_pressed:
                self.toggle_recording()
                    
        except AttributeError:
            # Special keys might not have char representation
            pass
        except Exception as e:
            print(f"⚠️ Key press error: {e}")
    
    def on_release(self, key):
        """Handle key release events"""
        try:
            from pynput import keyboard
            
            self.current_keys.discard(key)
                
        except AttributeError:
            pass
        except Exception as e:
            print(f"⚠️ Key release error: {e}")
    
    def toggle_recording(self):
        """Toggle recording state"""
        if self.recording:
            self.stop_recording()
        else:
            self.start_recording()
    
    def start_recording(self):
        """Start recording audio"""
        if self.recording:
            return
            
        self.recording = True
        print("🎤 Recording started...")
        
        # Show desktop notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                'Recording started... Press Super+Space again to stop and transcribe',
                '--icon=audio-input-microphone',
                '--expire-time=2000'
            ], capture_output=True)
        except:
            pass
        
        # Start recording in a separate thread
        self.record_thread = threading.Thread(target=self._record_audio)
        self.record_thread.daemon = True
        self.record_thread.start()
    
    def stop_recording(self):
        """Stop recording and transcribe"""
        if not self.recording:
            return
            
        self.recording = False
        print("🛑 Recording stopped, processing...")
        
        # Show processing notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                'Processing audio...',
                '--icon=view-refresh',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
    
    def _record_audio(self):
        """Record audio in background thread"""
        import pyaudio
        import wave
        
        # Audio settings (16kHz mono as required by model)
        chunk = 1024
        format = pyaudio.paInt16
        channels = 1
        rate = 16000
        
        try:
            audio = pyaudio.PyAudio()
            
            # Find best input device (same logic as main service)
            device_result = self.stt_service.find_input_device(audio)
            if device_result is None:
                audio.terminate()
                self.recording = False
                return
            
            device_id, device_info = device_result
            device_rate = int(device_info['defaultSampleRate'])
            
            stream = audio.open(
                format=format,
                channels=channels,
                rate=device_rate,  # Use device's native rate
                input=True,
                input_device_index=device_id,
                frames_per_buffer=chunk
            )
        except Exception as e:
            print(f"❌ Audio initialization failed: {e}")
            self.recording = False
            return
        
        frames = []
        
        # Record while self.recording is True
        try:
            while self.recording:
                data = stream.read(chunk, exception_on_overflow=False)
                frames.append(data)
        except Exception as e:
            print(f"❌ Recording error: {e}")
        finally:
            stream.stop_stream()
            stream.close()
            audio.terminate()
        
        # Save and transcribe if we have audio
        if frames and len(frames) > 10:  # At least some audio
            try:
                # Save to temporary file
                temp_file = tempfile.NamedTemporaryFile(suffix='.wav', delete=False)
                self.audio_file = temp_file.name
                
                with wave.open(temp_file.name, 'wb') as wf:
                    wf.setnchannels(channels)
                    wf.setsampwidth(audio.get_sample_size(format))
                    wf.setframerate(device_rate)  # Use actual recording rate
                    wf.writeframes(b''.join(frames))
                
                # Transcribe in separate thread to avoid blocking
                transcribe_thread = threading.Thread(target=self._process_transcription)
                transcribe_thread.daemon = True
                transcribe_thread.start()
                
            except Exception as e:
                print(f"❌ Error saving audio: {e}")
                self._show_error_notification("Failed to save audio")
        else:
            print("⚠️ No audio recorded")
            self._show_error_notification("No audio detected")
    
    def _process_transcription(self):
        """Process transcription in background"""
        if not self.audio_file or not os.path.exists(self.audio_file):
            return
        
        try:
            # Transcribe
            transcription = self.stt_service.transcribe_audio(self.audio_file)
            
            if transcription and transcription.strip():
                print(f"📝 Transcribed: {transcription}")
                
                # Small delay to ensure hotkey is fully released
                time.sleep(0.2)
                
                # Copy to clipboard first
                self.stt_service.copy_to_clipboard(transcription)
                
                # Type the text
                self.stt_service.type_text(transcription)
                
                # Show success notification
                self._show_success_notification(transcription)
            else:
                print("❌ No speech detected")
                self._show_error_notification("No speech detected")
                
        except Exception as e:
            print(f"❌ Transcription error: {e}")
            self._show_error_notification(f"Transcription failed: {str(e)[:50]}")
        finally:
            # Clean up audio file
            if self.audio_file and os.path.exists(self.audio_file):
                try:
                    os.unlink(self.audio_file)
                except:
                    pass
                self.audio_file = None
    
    def _show_success_notification(self, text):
        """Show success notification"""
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                'Transcription complete',
                '--icon=dialog-information',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
    
    def _show_error_notification(self, message):
        """Show error notification"""
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                message,
                '--icon=dialog-error',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
    
    
    def start_service(self):
        """Start the hotkey listener service"""
        try:
            from pynput import keyboard
        except ImportError:
            print("❌ pynput not installed!")
            print("Install with: pip install pynput")
            sys.exit(1)
        
        print("🎧 Speech-to-text hotkey service started")
        print("📱 Press Super+Space to start recording, press again to stop and transcribe")
        print("🛑 Press Ctrl+C to quit")
        print("")
        
        # Show startup notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech-to-Text Service', 
                'Service started! Use Super+Space to record',
                '--icon=audio-input-microphone',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
        
        try:
            with keyboard.Listener(
                on_press=self.on_press,
                on_release=self.on_release) as listener:
                listener.join()
        except KeyboardInterrupt:
            print("\n👋 Service stopped")
        except Exception as e:
            print(f"❌ Service error: {e}")

def main():
    # Check if we're in the right directory
    if not os.path.exists('speech_to_text.py'):
        print("❌ speech_to_text.py not found!")
        print("Make sure you're running this from the speech-to-text directory")
        sys.exit(1)
    
    try:
        service = HotkeyService()
        service.start_service()
    except KeyboardInterrupt:
        print("\n👋 Service stopped")
    except Exception as e:
        print(f"❌ Failed to start service: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
