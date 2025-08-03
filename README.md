# Speech-to-Text with Deepgram Streaming API

Real-time voice transcription into any text field in Ubuntu via hotkey (Super + ])

## Quick Start

```bash
# First time run
DEEPGRAM_API_KEY='your_api_key_here' ./install.sh

# Future runs
DEEPGRAM_API_KEY='your_api_key_here' ./run.sh
```

Press `Super+]` to start streaming transcription, press again to stop and type.

## Voice Commands

| Command | Description |
|---------|-------------|
| `undo that` | Remove last transcribed segment |
| `undo word` | Remove last word using cursor selection |
| `undo last X words` | Remove last X words (e.g., "undo last 3 words") |
| `correct X with Y` | Replace word X with Y in last sentence |
| `newline` / `new line` | Insert line break |
| `new para` / `next paragraph` | Insert paragraph break |
| `end voice` / `stop recording` | Stop transcription |
| `literal` / `literally` | Treat following text as literal (no commands) |

Text is automatically typed to the active window in real-time.

## Other Uses Cases

1. **Single streaming session:**
   ```bash
   source venv/bin/activate
   python speech_to_text.py -d 5  # Stream for 5 seconds
   ```

2. **Continuous streaming mode:**
   ```bash
   python speech_to_text.py -c  # Interactive streaming mode
   ```


## Why Deepgram Streaming API?

- Real-time: Live streaming transcription with instant results
- Keywords: Built-in support for detecting special commands like "delete"
- High accuracy: Nova-3 model with smart formatting
- Low latency: WebSocket-based streaming for minimal delay
- Flexible: Supports both streaming and file-based transcription

## Code structure

- `speech_to_text.py` - Main speech recognition script
- `speech_hotkey.py` - System-wide hotkey service  
- `run.sh` - Easy launcher
- `test_installation.py` - Test your setup

## How it Works: Wayland vs X11

This application works differently depending on your display server:

### **Wayland (Ubuntu 24 default)**
- **Hotkey Detection**: Uses GNOME's custom keyboard shortcuts (automatic setup)
- **How**: Install script configures `Super+]` in GNOME Settings → Keyboard → Custom Shortcuts
- **Typing**: Uses `ydotool` (primary) or `xdotool` (fallback)
- **Pros**: Secure, integrates with desktop environment, reliable
- **Setup**: Fully automatic during installation

### **X11 (legacy/manual selection)**
- **Hotkey Detection**: Uses `pynput` library for global key listening
- **How**: Python script runs continuously and monitors all keyboard input
- **Typing**: Uses `xdotool` (primary) or Wayland tools (fallback)
- **Pros**: Direct hotkey detection, works on any X11 desktop environment
- **Setup**: Automatic, no desktop environment configuration needed

### **Manual Usage (both)**
- Run `python speech_to_text.py` directly for single recording sessions
- Use `python speech_to_text.py -c` for continuous mode without hotkeys

## Troubleshooting

- Test installation: `python test_installation.py`
- Check API key: `echo $DEEPGRAM_API_KEY`
- Check audio: `pactl list sources short`

### **Wayland Issues**
- If hotkey doesn't work: Check GNOME Settings → Keyboard → Custom Shortcuts
- Re-run install script if you change API key: `./install.sh`
- Manual setup: Add `Super+]` → `/path/to/speech_hotkey.py --hotkey`

### **X11 Issues**
- If hotkey doesn't work: Try running `./run.sh` manually
- Permission errors: Check if user is in input group
- Display issues: Verify `$DISPLAY` environment variable


