#!/usr/bin/env python3
"""
System-wide hotkey service for speech-to-text
Listens for Super+] and toggles speech recognition
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
        
        # Setup signal handlers for clean shutdown and toggle
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
        signal.signal(signal.SIGUSR1, self._toggle_handler)
        
    def _signal_handler(self, signum, frame):
        """Handle shutdown signals"""
        print("\n🛑 Shutting down...")
        if self.streaming_task:
            self.streaming_task.cancel()
        sys.exit(0)
    
    def _toggle_handler(self, signum, frame):
        """Handle toggle signal from hotkey"""
        print("🎯 Toggle signal received")
        self.toggle_recording()
        
    def on_press(self, key):
        """Handle key press events"""
        try:
            # Import here to avoid issues if pynput not available
            from pynput import keyboard
            
            self.current_keys.add(key)
            
            # Check for Super+] combination
            # On Linux/Wayland, Super key is often mapped to cmd
            super_pressed = (keyboard.Key.cmd in self.current_keys or 
                           keyboard.Key.cmd_r in self.current_keys)
            # Check for super_l/super_r if they exist in this pynput version
            try:
                super_pressed = super_pressed or (keyboard.Key.super_l in self.current_keys or 
                                                keyboard.Key.super_r in self.current_keys)
            except AttributeError:
                # super_l/super_r not available in this pynput version
                pass
            
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
                'Streaming started... Press Super+] again to stop and transcribe',
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
    
    
    def _test_keyboard_access(self):
        """Test if we can access keyboard events"""
        try:
            from pynput import keyboard
            import time
            
            print("🔍 Testing keyboard access...")
            
            test_keys = []
            def test_press(key):
                test_keys.append(key)
            
            # Try to create a listener and test for 1 second
            try:
                with keyboard.Listener(on_press=test_press, suppress=False) as listener:
                    # Test very briefly
                    time.sleep(0.1)
                    listener.stop()
                
                return True
            except Exception as e:
                print(f"   Keyboard access test failed: {e}")
                return False
        except Exception:
            return False
    
    def start_service(self):
        """Start the hotkey listener service"""
        try:
            from pynput import keyboard
        except ImportError:
            print("❌ pynput not installed!")
            print("Install with: pip install pynput")
            sys.exit(1)
        
        print("🎧 Speech-to-text hotkey service started")
        print("📱 Press Super+] to start recording, press again to stop and transcribe")
        print("🛑 Press Ctrl+C to quit")
        
        # Check display server
        display_server = os.environ.get('XDG_SESSION_TYPE', 'unknown')
        wayland_display = os.environ.get('WAYLAND_DISPLAY')
        x11_display = os.environ.get('DISPLAY')
        
        print(f"🖥️  Display server: {display_server}")
        
        if display_server == 'wayland' or wayland_display:
            print("⚠️ Wayland detected - testing global hotkey access...")
            if not self._test_keyboard_access():
                print("❌ Global hotkeys not accessible on this Wayland session")
                print("💡 Solutions for Wayland:")
                print("   1. Use desktop environment hotkey settings:")
                print(f"      - Add custom shortcut: Super+] → {os.path.abspath(__file__)}")
                print("      - GNOME: Settings > Keyboard > Custom Shortcuts")
                print("      - KDE: System Settings > Shortcuts")
                print("   2. Switch to X11 session (better hotkey support)")
                print("   3. Run manually: 'python speech_to_text.py' when needed")
                return
            else:
                print("✅ Global hotkey access works on this Wayland session")
        elif display_server == 'x11' or x11_display:
            print("✅ X11 detected - global hotkeys should work normally")
        else:
            print("⚠️ Unknown display server - attempting to start anyway")
        
        print("")
        
        # Show startup notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech-to-Text Service', 
                'Service started! Use Super+] to record',
                '--icon=audio-input-microphone',
                '--expire-time=2000'
            ], capture_output=True)
        except:
            pass
        
        try:
            with keyboard.Listener(
                on_press=self.on_press,
                on_release=self.on_release,
                suppress=False) as listener:  # Don't suppress keys for better compatibility
                listener.join()
        except KeyboardInterrupt:
            print("\n👋 Service stopped")
        except Exception as e:
            print(f"❌ Service error: {e}")
            
            # Provide specific guidance based on the error and environment
            if display_server == 'wayland' or wayland_display:
                print("💡 Wayland security restrictions prevent global hotkeys.")
                print("💡 Use desktop environment hotkey settings instead.")
            elif "could not open display" in str(e).lower():
                print("💡 Display server connection failed.")
                print("💡 Try running in a GUI session or check DISPLAY variable.")
            else:
                print("💡 Try running in an X11 session for better compatibility.")

def is_hotkey_mode():
    """Detect if we're being called as a hotkey (via --hotkey flag)"""
    return '--hotkey' in sys.argv

def toggle_service():
    """Single toggle for hotkey mode - start or stop recording"""
    # Check if there's already an active voice recording process
    lock_file = "/tmp/voice_recording.lock"
    
    if os.path.exists(lock_file):
        # Voice recording is active, stop it
        try:
            with open(lock_file, 'r') as f:
                pid = int(f.read().strip())
            
            print("🛑 Stopping active voice recording...")
            
            # Kill the recording process
            try:
                os.kill(pid, signal.SIGTERM)
                time.sleep(0.5)  # Give it time to cleanup
                
                # Show stop notification
                subprocess.run([
                    'notify-send', 
                    'Voice Typing Stopped', 
                    'Recording interrupted by hotkey.',
                    '--icon=audio-input-microphone-muted',
                    '--expire-time=3000'
                ], capture_output=True)
                
            except ProcessLookupError:
                # Process already dead
                pass
            
            # Remove lock file
            if os.path.exists(lock_file):
                os.remove(lock_file)
                
        except Exception as e:
            print(f"⚠️ Error stopping recording: {e}")
            # Remove stale lock file
            if os.path.exists(lock_file):
                os.remove(lock_file)
        
        return
    
    # No active recording, start new one
    try:
        print("🎯 Hotkey triggered - starting speech recognition...")
        
        # Create lock file with current process PID
        with open(lock_file, 'w') as f:
            f.write(str(os.getpid()))
        
        # Show initialization notification
        try:
            subprocess.run([
                'notify-send', 
                'Voice Typing Service', 
                'Initializing... Please wait.',
                '--icon=audio-input-microphone',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
        
        # Import and run speech-to-text directly
        from speech_to_text import SpeechToTextService
        
        # Start a single recording session
        stt_service = SpeechToTextService()
        
        # Use continuous mode with automatic stopping (similar to hotkey behavior)
        # We'll simulate a short recording session
        import asyncio
        
        async def single_recording():
            await stt_service.start_streaming(real_time_typing=True)
        
        # Run the recording
        asyncio.run(single_recording())
        
    except Exception as e:
        print(f"❌ Hotkey toggle failed: {e}")
        # Show error notification
        try:
            subprocess.run([
                'notify-send', 
                'Speech Recognition', 
                f'Failed: {str(e)[:50]}',
                '--icon=dialog-error',
                '--expire-time=3000'
            ], capture_output=True)
        except:
            pass
    finally:
        # Always cleanup lock file when done
        lock_file = "/tmp/voice_recording.lock"
        if os.path.exists(lock_file):
            try:
                os.remove(lock_file)
            except:
                pass

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
    
    # Detect if we're being called as a hotkey or as a persistent service
    if is_hotkey_mode():
        print("🎯 Hotkey mode detected - performing single toggle")
        toggle_service()
    else:
        print("🎧 Starting persistent hotkey listener service")
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
