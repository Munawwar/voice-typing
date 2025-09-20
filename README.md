# Voice Typing

Linux real-time voice typing tool using Deepgram's streaming API. Works on both Wayland and X11.

## Features

- **Real-time transcription**: Text appears immediately as you speak
- **Voice commands**: "undo that", "newline", "new paragraph", "stop voice"
- **Fallback text injection**: Direct typing → Clipboard paste → Silent mode
- **Single binary**: Not too many dependencies (Great to bind it to keyboard hotkey)
- **Desktop integration**: GNOME/Unity hotkey integrated during install

## Why Deepgram?

It's good, that's why! I am not affiliated with the company in any way. They give free credits, and after that it is paid. To get a key you'll need to sign up: https://console.deepgram.com/signup.

Any alternative you suggest me needs to:

- be real-time with low latency
- have keyword detection for special commands like "delete"
- have good accuracy

## Installation

### Quick Install (Recommended)

```bash
git clone <repository-url>

# Copy config.example.json to config.json and add your deepgram API

./install.sh

# Logout and login back from GNOME because typing (ydotool) won't work without it.

# On non-GNOME desktops you need to follow the hotkey instructions

# Next press Super+] on the keyboard and start talking!
```

The installer will:
- Install system dependencies (portaudio, typing tools)
- Build the Go application
- Install to `~/.local/bin`
- Set up configuration directory (`~/.config/`)
- Configure GNOME/Unity hotkey (`Super+]`)
- Handle Wayland/X11 compatibility

#### Build

```bash
make build
```

## Usage

### Single Recording Session

```bash
./voice-typing
```

### Hotkey Mode (Recommended)

Set up a desktop hotkey (Super+]) that runs:
```bash
/path/to/voice-typing --hotkey
```

**GNOME**: Settings → Keyboard → Custom Shortcuts
**KDE**: System Settings → Shortcuts → Custom Shortcuts


## Voice Commands

- **"undo that"**: Remove the last transcribed phrase
- **"undo word"**: Remove the last word
- **"undo last 3 words"**: Remove multiple words
- **"newline"** or **"new line"**: Insert line break
- **"new paragraph"** or **"next para"**: Insert paragraph break
- **"stop voice"** or **"end recording"**: Stop transcription


## Configuration Options

```json
{
  "deepgram_api_key": "your_key",
  "hotkey": "Super_R+bracketright",
  "audio": {
    "sample_rate": 16000,
    "channels": 1,
    "buffer_size": 1024
  },
  "transcription": {
    "model": "nova-3",
    "language": "en-US",
    "smart_format": true,
    "punctuate": true,
    "profanity_filter": true,
    "filler_words": true,
    "mip_opt_out: true
  }
}
```

You can find docs on the `transcription` configs at [deepgram's docs](https://developers.deepgram.com/reference/speech-to-text-api/listen-streaming).

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Single Go Binary                         │
├─────────────────────────────────────────────────────────────┤
│  Audio Capture (PortAudio)                                  │
│  ├─ Microphone input                                        │
│  └─ Stream to Deepgram WebSocket                            │
├─────────────────────────────────────────────────────────────┤
│  Transcription Engine                                       │
│  ├─ Deepgram Go SDK v3                                      │
│  ├─ Phrase stack for undo                                   │
│  └─ Real-time command processing                            │
├─────────────────────────────────────────────────────────────┤
│  Text Injection                                             │
│  ├─ Tool detection (ydotool, wtype, xdotool)                │
│  ├─ Fallback hierarchy                                      │
│  └─ Cross-platform support                                  │
└─────────────────────────────────────────────────────────────┘
```

## Troubleshooting

- Re-run install script if you change API key: `./install.sh`

### Audio Issues
- Ensure your microphone is working: `arecord -l`
- Check permissions: Add user to `audio` group

### Typing Issues
- Install required tools (see `install.sh`)
- For GNOME, if hotkey doesn't work: Check GNOME Settings → Keyboard → Custom Shortcuts
- For Wayland: Ensure ydotool daemon is running
- Check display server: `echo $XDG_SESSION_TYPE`
- Wayland specifics:
  - Ensure `ydotoold` is active: `systemctl status ydotoold` (or `sudo systemctl enable --now ydotoold`)
  - Add user to `input` group and re-login: `sudo usermod -a -G input $USER`
  - `wtype` is best-effort; some DEs sandbox key injection. If `wtype` fails, `ydotool` should work once the daemon and group permissions are correct.
  - Clipboard fallback requires `wl-copy`; install with: `sudo apt install wl-clipboard`

### Permissions
- For ydotool: Add user to `input` group and log out/in
- For audio: Add user to `audio` group

### Deepgram Connection
- If you see `Error sending audio data: connection is not valid`, it usually means audio started sending before the WebSocket was ready. The app now waits for the `open` event before streaming. If it persists:
  - Check your API key in `config.json`.
  - Ensure outbound `wss://api.deepgram.com` is reachable.
  - Try a lower sample rate like 16000 and mono channels (default).

## Development

```bash
# Run with debug logging
DEEPGRAM_LOG_LEVEL=DEBUG ./voice-typing

# Build for release
make release

# Clean build artifacts
make clean
```

## License

MIT License - see LICENSE file for details.
