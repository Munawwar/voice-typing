# Speech-to-Text with CrisperWhisper

A lightweight, accurate speech-to-text system using nyrahealth/CrisperWhisper. Press Super+P to record, release to transcribe instantly!

## Installation

1. **Run the installer:**
   ```bash
   ./install.sh
   ```

2. **Start the service:**
   ```bash
   ./run.sh
   ```

## Quick Start

- **Hotkey mode:** Press `Super+P`, speak, release to transcribe
- **Single recording:** `python speech_to_text.py -d 5` (record for 5 seconds)
- **Test audio:** `python test_installation.py`

## Features

- **🎤 System-wide hotkey** - Works in any application
- **🚀 Fast startup** - No massive dependencies 
- **💾 Memory efficient** - Auto-detects GPU memory, falls back to CPU
- **🔧 Audio smart** - Automatically finds and uses your microphone
- **📱 Cross-platform** - Supports both X11 and Wayland

## Files

- `speech_to_text.py` - Main speech recognition engine
- `speech_hotkey.py` - System-wide hotkey service
- `run.sh` - Service launcher
- `install.sh` - One-command setup
- `requirements.txt` - Python dependencies

## Troubleshooting

- **Audio issues:** `pactl list sources short` to check microphones
- **GPU issues:** `nvidia-smi` to check GPU memory
- **Dependencies:** `python test_installation.py` to verify setup
- **Permissions:** Log out/in after installation for Wayland support

## Why CrisperWhisper?

- **Lightweight:** ~3.5GB total vs ~15GB+ for NVIDIA models
- **Simple:** Uses standard transformers, no complex NeMo dependencies  
- **Accurate:** 6.67% WER performance
- **Smart:** CPU fallback for low-memory GPUs
- **Compatible:** Works with any microphone, any desktop environment

