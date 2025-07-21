# Speech-to-Text with Deepgram Streaming API

Real-time voice transcription into any text field in Ubuntu via hotkey (Super + Space)

## Quick Start

```bash
# First time run
DEEPGRAM_API_KEY='your_api_key_here' ./install.sh

# Future runs
DEEPGRAM_API_KEY='your_api_key_here' ./run.sh
```

Press `Super+Space` to start streaming transcription, press again to stop and type.

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
- High accuracy: Nova-2 model with smart formatting
- Low latency: WebSocket-based streaming for minimal delay
- Flexible: Supports both streaming and file-based transcription

## Code structure

- `speech_to_text.py` - Main speech recognition script
- `speech_hotkey.py` - System-wide hotkey service  
- `run.sh` - Easy launcher
- `test_installation.py` - Test your setup

## Troubleshooting

- Test installation: `python test_installation.py`
- Check API key: `echo $DEEPGRAM_API_KEY`
- Check audio: `pactl list sources short`


