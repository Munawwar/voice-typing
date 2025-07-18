#!/bin/bash
# Speech-to-Text Service Launcher

cd "$(dirname "$0")"

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo "❌ Virtual environment not found!"
    echo "Run install.sh first"
    exit 1
fi

# Activate virtual environment
source venv/bin/activate

# Set display environment
export DISPLAY=${DISPLAY:-:0}

# Run the hotkey service
echo "🎤 Starting Speech-to-Text Hotkey Service..."
echo "Press Super+P and hold to record, release to transcribe"
echo "Press Ctrl+C to quit"
echo ""

python speech_hotkey.py
