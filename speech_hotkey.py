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
import asyncio
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
        self.streaming_task = None
        self.loop = None
        self.initializing = False  # Flag to prevent race conditions
        
        # Setup signal handlers for clean shutdown
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
        
    def _signal_handler(self, signum, frame):
        """Handle shutdown signals"""
        print("\n🛑 Shutting down...")
        if self.streaming_task:
            self.streaming_task.cancel()
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
        """Start streaming audio"""
        if self.recording or self.initializing:
            print("⚠️ Already recording or initializing, ignoring hotkey")
            return
            
        self.initializing = True
        self.recording = True
        print("🎤 Streaming started...")
        
        # Show desktop notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                'Streaming started... Press Super+Space again to stop and transcribe',
                '--icon=audio-input-microphone',
                '--expire-time=2000'
            ], capture_output=True)
        except:
            pass
        
        # Reset transcription state
        self.stt_service.transcription_parts = []
        self.stt_service.current_text = ""
        
        # Start streaming in a separate thread
        self.record_thread = threading.Thread(target=self._start_streaming)
        self.record_thread.daemon = True
        self.record_thread.start()
    
    def stop_recording(self):
        """Stop streaming and transcribe"""
        if not self.recording:
            return
            
        self.recording = False
        self.initializing = False  # Clear both flags
        print("🛑 Streaming stopped, processing...")
        
        # Cancel streaming task
        if self.streaming_task:
            self.streaming_task.cancel()
        
        # Show processing notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                'Processing transcription...',
                '--icon=view-refresh',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
        
        # Process the final transcription
        self._process_final_transcription()
    
    def _start_streaming(self):
        """Start streaming in background thread"""
        try:
            # Create new event loop for this thread
            self.loop = asyncio.new_event_loop()
            asyncio.set_event_loop(self.loop)
            
            # Clear initializing flag once we start the actual streaming
            self.initializing = False
            
            # Start streaming with real-time typing enabled
            self.streaming_task = self.loop.create_task(self.stt_service.start_streaming(real_time_typing=True))
            self.loop.run_until_complete(self.streaming_task)
            
        except asyncio.CancelledError:
            print("🛑 Streaming cancelled")
        except Exception as e:
            print(f"❌ Streaming error: {e}")
            self._show_error_notification(f"Streaming failed: {str(e)[:50]}")
        finally:
            self.initializing = False
            if self.loop:
                self.loop.close()
    
    def _process_final_transcription(self):
        """Process final transcription from streaming"""
        try:
            # Get the final transcription from the service
            if self.stt_service.current_text:
                transcription = self.stt_service.current_text.strip()
                print(f"📝 Final transcription: {transcription}")
                
                # Small delay to ensure hotkey is fully released
                time.sleep(0.2)
                
                
                # Note: Text has already been typed in real-time, so no need to type again
                # Just show success notification
                self._show_success_notification()
            else:
                print("❌ No speech detected")
                self._show_error_notification("No speech detected")
                
        except Exception as e:
            print(f"❌ Transcription processing error: {e}")
            self._show_error_notification(f"Processing failed: {str(e)[:50]}")
    
    def _show_success_notification(self):
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
                '--expire-time=2000'
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
    
    # I had bugs somehow that this speech hotkey was running multiple times.
    # And that is messy as it will type multiple times into the same text field.
    # Also it will eat your API credits.
    # So I added this check to kill any existing instances.
    import subprocess
    try:
        result = subprocess.run(['pgrep', '-f', 'speech_hotkey.py'], capture_output=True, text=True)
        existing_pids = result.stdout.strip().split('\n') if result.stdout.strip() else []
        current_pid = str(os.getpid())
        
        # Filter out current process
        other_pids = [pid for pid in existing_pids if pid != current_pid and pid]
        
        if other_pids:
            print(f"⚠️ Found {len(other_pids)} existing speech-to-text instance(s)")
            print("Killing existing instances to prevent conflicts...")
            for pid in other_pids:
                try:
                    subprocess.run(['kill', pid], check=True)
                    print(f"✅ Killed process {pid}")
                except subprocess.CalledProcessError:
                    print(f"❌ Failed to kill process {pid}")
    except:
        # If pgrep fails, continue anyway
        pass
    
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
