#!/bin/bash
# Speech-to-Text Service Launcher

cd "$(dirname "$0")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo -e "${RED}❌ Virtual environment not found!${NC}"
    echo "Run install.sh first"
    exit 1
fi

# Check for API key
if [ -z "$DEEPGRAM_API_KEY" ]; then
    echo -e "${RED}❌ DEEPGRAM_API_KEY environment variable not set!${NC}"
    echo -e "${YELLOW}Set your API key first:${NC}"
    echo "export DEEPGRAM_API_KEY='your_api_key_here'"
    echo ""
    echo -e "${YELLOW}Or add it to your ~/.bashrc or ~/.zshrc:${NC}"
    echo "echo 'export DEEPGRAM_API_KEY=\"your_api_key_here\"' >> ~/.bashrc"
    exit 1
fi

# Activate virtual environment
source venv/bin/activate

# Set display environment
export DISPLAY=${DISPLAY:-:0}

# Run the hotkey service
echo -e "${GREEN}🎤 Starting Speech-to-Text Hotkey Service...${NC}"
echo -e "${GREEN}📱 Press Super+Space to start streaming, press again to stop and transcribe${NC}"
echo -e "${YELLOW}🛑 Press Ctrl+C to quit${NC}"
echo ""

python speech_hotkey.py
