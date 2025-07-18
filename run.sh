#!/bin/bash
# Speech-to-Text Service Launcher
# Enhanced launcher with dependency checking and environment setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
log_success() { echo -e "${GREEN}✅ $1${NC}"; }
log_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
log_error() { echo -e "${RED}❌ $1${NC}"; }

# Get script directory and change to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🎤 Speech-to-Text Service Launcher"
echo "=================================="

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    log_error "Virtual environment not found!"
    log_info "Run install.sh first to set up the environment"
    exit 1
fi

# Activate virtual environment
log_info "Activating virtual environment..."
source venv/bin/activate

# Quick dependency checks
log_info "Checking key dependencies..."

# Check PyTorch
if ! python -c "import torch" 2>/dev/null; then
    log_error "PyTorch not available - run install.sh to fix"
    exit 1
fi

# Check NeMo
if ! python -c "import nemo.collections.speechlm" 2>/dev/null; then
    log_error "NeMo Toolkit not available - run install.sh to fix"
    exit 1
fi

log_success "Dependencies available"

# Set up environment for GUI applications
export DISPLAY=${DISPLAY:-:0}
export XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}

# Check for conflicting processes
if pgrep -f "speech_hotkey.py" >/dev/null; then
    log_warning "Speech hotkey service already running"
    log_info "Kill existing process? (y/N)"
    read -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        pkill -f "speech_hotkey.py"
        sleep 2
        log_info "Existing process killed"
    else
        log_info "Exiting to avoid conflicts"
        exit 1
    fi
fi

# Show startup information
echo ""
log_success "Starting Speech-to-Text Hotkey Service..."
echo ""
echo "📱 Hotkey: Super+P (hold to record, release to transcribe)"
echo "🛑 Press Ctrl+C to stop the service"
echo ""

# Handle cleanup on exit
cleanup() {
    echo ""
    log_info "Service stopped"
    exit 0
}

trap cleanup SIGINT SIGTERM

# Start the service
log_info "Loading speech recognition model (this may take a moment)..."
python speech_hotkey.py
