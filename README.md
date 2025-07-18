# Speech-to-Text with NVIDIA Canary

This project provides a system-wide speech-to-text service using NVIDIA's Canary Qwen 2.5B model. It allows you to convert speech to text anywhere in your operating system using a simple hotkey (Super+P). The transcribed text is automatically typed into whatever application has focus, making it perfect for dictating emails, documents, chat messages, or any text input. The system supports both X11 and Wayland display servers with automatic detection and fallback methods. **Note: This has only been tested on Ubuntu 24.04 with X11.**

## Installation and Usage

### Prerequisites
- Ubuntu 24.04 X11 (tested environment)
- Prepare for 10 GB download of dependencies. PyTorch + Nemo toolkit + Qwen model is huge. You need fast internet connection + disk space.
- NVIDIA GPU with drivers installed (optional but recommended, will fall back to CPU)
- Python 3.11 or 3.12
- Microphone access

### Install

1. **Download the repository files:**
   ```bash
   # Clone or download the repository
   git clone <repository-url>
   cd speech-to-text
   ```

2. **Run the installer:**
   ```bash
   chmod +x install.sh
   ./install.sh
   ```
   
   The installer will:
   - Check system requirements (GPU, Python, drivers)
   - Install system dependencies
   - Create Python virtual environment
   - Install PyTorch with CUDA support
   - Install NVIDIA NeMo toolkit
   - Set up typing tools for both X11 and Wayland

### Run

**Start the hotkey service:**
```bash
cd ~/speech-to-text
./run_speech_service.sh
```

**Usage:**
- Press and hold `Super+P` (Windows key + P)
- Speak your message
- Release the keys to transcribe and type the text

**Alternative usage:**
```bash
# Activate the virtual environment
source ~/speech-to-text/venv/bin/activate

# Single 5-second recording
python speech_to_text.py -d 5

# Transcribe an audio file
python speech_to_text.py -f audio.wav

# Interactive continuous mode
python speech_to_text.py -c
```

### Optional: Auto-start Service

To start the service automatically on login:
```bash
cp speech-to-text.service ~/.config/systemd/user/
systemctl --user enable speech-to-text.service
systemctl --user start speech-to-text.service
```

## Troubleshooting

- **CUDA issues:** Ensure NVIDIA drivers are installed with `nvidia-smi`
- **Audio issues:** Check microphone access with `arecord -l`
- **Typing issues:** Install tools with `sudo apt install xdotool wtype ydotool`
- **Permissions:** For Wayland, ensure you're in the input group and log out/in

## Hardware Requirements

- **Minimum:** Any x86_64 system with microphone
- **Recommended:** NVIDIA GPU with 4GB+ VRAM for faster transcription
- **Tested on:** NVIDIA T500 Mobile, Ubuntu 24.04, X11
