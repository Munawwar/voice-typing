# Speech-to-Text with Groq Whisper Large v3 Turbo

Voice type into any text field in Ubuntu via hotkey (Super + Space)

Tested on Ubuntu 24 X11 (Wayland untested)

## Quick Start

```bash
# First time run
GROQ_API_KEY='your_api_key_here' ./install.sh

# Future runs
GROQ_API_KEY='your_api_key_here' ./run.sh
```

Press `Super+Space` to start recording, press again to stop and transcribe.

Text is automatically typed to the active window and also copied to clipboard¹.

¹ Clipboard copy is useful for cases when you were focused on the wrong text field

## Other Uses Cases

1. **Single recording:**
   ```bash
   source venv/bin/activate
   python speech_to_text.py -d 5  # Record for 5 seconds
   ```

2. **Copy to clipboard:**
   ```bash
   python speech_to_text.py --copy-to-clipboard -d 5  # Copy transcription to clipboard
   ```

## Why Groq Whisper Large v3 Turbo?

- Fast: Can actually run on your tiny laptop vs complex local models. Also Groq especially has made this model even faster.
- Good accuracy
- Easy setup: Unlike most local ASR models setup where things like NeMo toolkit is needed, which is a pain to install!
- Cheap: Free tier available, paid tier is $0.04 per hour of audio

## Code structure

- `speech_to_text.py` - Main speech recognition script
- `speech_hotkey.py` - System-wide hotkey service  
- `run.sh` - Easy launcher
- `test_installation.py` - Test your setup

## Troubleshooting

- Test installation: `python test_installation.py`
- Check API key: `echo $GROQ_API_KEY`
- Check audio: `pactl list sources short`


