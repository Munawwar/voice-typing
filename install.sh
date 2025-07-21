#!/bin/bash
# Comprehensive Speech-to-Text Installer
# Supports both X11 and Wayland, checks all dependencies

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

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   log_error "Don't run this script as root!"
   exit 1
fi

echo "🎤 Deepgram Streaming Speech-to-Text Installer"
echo "=============================================="

# Step 1: Check Python version
log_info "Checking Python version..."
PYTHON_CMD=""
MIN_PYTHON_VERSION="3.11"

# Try different Python commands
for cmd in python3.12 python3.11 python3; do
    if command -v "$cmd" >/dev/null 2>&1; then
        PYTHON_VERSION=$($cmd --version 2>&1 | cut -d' ' -f2)
        PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d'.' -f1)
        PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d'.' -f2)
        
        if [[ $PYTHON_MAJOR -eq 3 ]] && [[ $PYTHON_MINOR -ge 11 ]] && [[ $PYTHON_MINOR -le 12 ]]; then
            PYTHON_CMD="$cmd"
            log_success "Found compatible Python: $cmd ($PYTHON_VERSION)"
            break
        fi
    fi
done

if [[ -z "$PYTHON_CMD" ]]; then
    log_error "No compatible Python found! Need Python 3.11 or 3.12"
    log_info "Install with: sudo apt install python3.12 python3.12-venv python3.12-dev"
    exit 1
fi

# Step 2: Check for DEEPGRAM_API_KEY environment variable
log_info "Checking for Deepgram API key..."
if [[ -z "$DEEPGRAM_API_KEY" ]]; then
    log_warning "DEEPGRAM_API_KEY environment variable not set"
    log_info "You'll need to set this before running the service:"
    log_info "export DEEPGRAM_API_KEY='your_api_key_here'"
else
    log_success "DEEPGRAM_API_KEY environment variable is set"
fi

# Step 3: Install system dependencies
log_info "Installing system dependencies..."

# Update package list
sudo apt update

# Install essential packages
PACKAGES=(
    "build-essential"
    "libnotify-bin"
    "python3-venv"
    "python3-dev"
    "python3-pip"
    "git"
    "curl"
    "wget"
)

# Display server specific packages
DISPLAY_SERVER="${XDG_SESSION_TYPE:-x11}"
log_info "Display server detected: $DISPLAY_SERVER"

if [[ "$DISPLAY_SERVER" == "wayland" ]]; then
    log_info "Installing Wayland tools (primary) + X11 tools (fallback)..."
    PACKAGES+=(
        "wtype"
        "ydotool"  
        "xdotool"
    )
else
    log_info "Installing X11 tools (primary) + Wayland tools (fallback)..."
    PACKAGES+=(
        "xdotool"
        "wtype"
        "ydotool"
    )
fi

# Install packages
log_info "Installing: ${PACKAGES[*]}"
sudo apt install -y "${PACKAGES[@]}"

# Configure ydotool for Wayland support
if command -v ydotool >/dev/null 2>&1; then
    log_info "Configuring ydotool..."
    sudo systemctl enable --now ydotoold 2>/dev/null || log_warning "ydotoold setup skipped"
    sudo usermod -a -G input "$USER" 2>/dev/null || log_warning "input group assignment skipped"
fi

# Step 4: Set up project directory
if [ "$(basename "$PWD")" == "speech-to-text" ] && [ -f "install.sh" ]; then
    # We're already in the project directory (likely from git clone)
    PROJECT_DIR="$PWD"
    log_info "Using current directory as project: $PROJECT_DIR"
else
    # Create project directory in home
    PROJECT_DIR="$HOME/speech-to-text"
    log_info "Creating project directory: $PROJECT_DIR"
    
    if [[ -d "$PROJECT_DIR" ]]; then
        log_warning "Project directory exists. Continuing..."
    else
        mkdir -p "$PROJECT_DIR"
    fi
    
    cd "$PROJECT_DIR"
fi

# Step 5: Setup Python virtual environment
EXISTING_VENV=false
if [[ -d "venv" ]] && [[ -f "venv/bin/activate" ]]; then
    log_info "Existing virtual environment found, checking compatibility..."
    source venv/bin/activate
    
    # Check if the Python version in venv matches our requirements
    VENV_PYTHON_VERSION=$(python --version 2>&1 | cut -d' ' -f2)
    VENV_MAJOR=$(echo $VENV_PYTHON_VERSION | cut -d'.' -f1)
    VENV_MINOR=$(echo $VENV_PYTHON_VERSION | cut -d'.' -f2)
    
    if [[ $VENV_MAJOR -eq 3 ]] && [[ $VENV_MINOR -ge 11 ]] && [[ $VENV_MINOR -le 12 ]]; then
        log_success "Compatible virtual environment found (Python $VENV_PYTHON_VERSION)"
        EXISTING_VENV=true
    else
        log_warning "Incompatible Python version in venv ($VENV_PYTHON_VERSION), recreating..."
        deactivate 2>/dev/null || true
        rm -rf venv
    fi
fi

if [[ "$EXISTING_VENV" == false ]]; then
    log_info "Creating new Python virtual environment..."
    $PYTHON_CMD -m venv venv
    source venv/bin/activate
    
    # Upgrade pip
    log_info "Upgrading pip..."
    pip install --upgrade pip
else
    log_info "Using existing virtual environment"
fi

# Step 6: Install Python dependencies from requirements.txt
log_info "Installing Python dependencies..."

# Install from requirements.txt
if [[ -f "requirements.txt" ]]; then
    log_info "Installing dependencies from requirements.txt..."
    pip install -r requirements.txt
else
    log_error "requirements.txt not found!"
    exit 1
fi

# Step 7: Test Deepgram package installation
log_info "Testing Deepgram package installation..."
python -c "
import deepgram
print(f'Deepgram package version: {deepgram.__version__}')
print('✅ Deepgram package installed successfully')
" || {
    log_error "Deepgram package test failed!"
    exit 1
}

log_success "All dependencies installed successfully!"

# Step 8: Ensure run.sh is executable
log_info "Setting up launcher script..."
if [ -f "run.sh" ]; then
    chmod +x run.sh
    log_success "run.sh is ready"
else
    log_error "run.sh not found!"
    exit 1
fi

# Step 9: Verify required files exist
log_info "Verifying required Python scripts..."

# Check if the required files exist in the current directory
if [ ! -f "speech_to_text.py" ] || [ ! -f "speech_hotkey.py" ]; then
    log_error "Required Python scripts not found!"
    log_info "Make sure you're running this script from the cloned repository directory"
    log_info "Expected files: speech_to_text.py and speech_hotkey.py"
    exit 1
fi

log_success "Required Python scripts found"
chmod +x speech_to_text.py speech_hotkey.py

# Step 10: Test installation
log_info "Testing complete installation..."

# Create test script
cat > test_installation.py << 'EOF'
#!/usr/bin/env python3
import sys
import os
import subprocess

def test_imports():
    """Test all required imports"""
    try:
        import deepgram
        import pynput
        import tempfile
        import subprocess
        import asyncio
        from dotenv import load_dotenv
        
        print("✅ All Python packages imported successfully")
        print(f"✅ Deepgram package version: {deepgram.__version__}")
        
        return True
    except ImportError as e:
        print(f"❌ Import error: {e}")
        return False

def test_system_tools():
    """Test system tools availability"""
    display_server = os.environ.get('XDG_SESSION_TYPE', 'x11')
    
    tools_to_test = []
    if display_server == 'wayland':
        tools_to_test = ['wtype', 'ydotool']
    else:
        tools_to_test = ['xdotool']
    
    tools_to_test.extend(['pactl', 'notify-send'])
    
    success = True
    for tool in tools_to_test:
        try:
            subprocess.run([tool, '--help'], capture_output=True, timeout=5)
            print(f"✅ {tool} available")
        except (FileNotFoundError, subprocess.TimeoutExpired):
            print(f"❌ {tool} not found")
            success = False
    
    return success

def test_deepgram_api_key():
    """Test DEEPGRAM_API_KEY environment variable"""
    api_key = os.environ.get('DEEPGRAM_API_KEY')
    if api_key:
        print("✅ DEEPGRAM_API_KEY environment variable is set")
        return True
    else:
        print("⚠️  DEEPGRAM_API_KEY environment variable not set")
        print("   Set it with: export DEEPGRAM_API_KEY='your_api_key_here'")
        return False

if __name__ == "__main__":
    print("🧪 Testing Speech-to-Text Installation")
    print("=" * 40)
    
    tests = [
        ("Python packages", test_imports),
        ("System tools", test_system_tools), 
        ("Deepgram API key", test_deepgram_api_key)
    ]
    
    results = []
    for test_name, test_func in tests:
        print(f"\n🔍 Testing {test_name}...")
        results.append(test_func())
    
    print("\n" + "=" * 40)
    if all(results):
        print("🎉 All tests passed! Installation successful!")
        print("\nNext steps:")
        print("1. Run: ./run.sh")
    else:
        print("⚠️  Some tests failed. Check the output above.")
        if not os.environ.get('DEEPGRAM_API_KEY'):
            print("💡 Don't forget to set your DEEPGRAM_API_KEY!")
        sys.exit(1)
EOF

# Run test
python test_installation.py

# Final setup
log_info "Final setup..."

# Don't overwrite existing README.md as it was already updated
if [ ! -f "README.md" ]; then
cat > README.md << 'EOF'
# Speech-to-Text with Deepgram Streaming API

Real-time voice transcription into any text field in Ubuntu via hotkey (Super + Space)

Features keyword detection (say "delete" to remove last transcription segment)

## Quick Start

1. **Set your API key:**
   ```bash
   export DEEPGRAM_API_KEY='your_api_key_here'
   ```

2. **Start the service:**
   ```bash
   ./run.sh
   ```

3. **Use hotkey:** Press `Super+Space` to start streaming, press again to stop and type
   - Text is automatically typed to the active window in real-time

4. **Single streaming session:**
   ```bash
   source venv/bin/activate
   python speech_to_text.py -d 5  # Stream for 5 seconds
   ```

5. **Continuous streaming mode:**
   ```bash
   python speech_to_text.py -c  # Interactive streaming
   ```

## Files

- `speech_to_text.py` - Main speech recognition script
- `speech_hotkey.py` - System-wide hotkey service  
- `run.sh` - Easy launcher
- `test_installation.py` - Test your setup

## Troubleshooting

- Test installation: `python test_installation.py`
- Check API key: `echo $DEEPGRAM_API_KEY`
- Check audio: `pactl list sources short`

## Why Deepgram Streaming API?

- **Real-time:** Live streaming transcription with instant results
- **Keywords:** Built-in support for detecting special commands like "delete"
- **High accuracy:** Nova-2 model with smart formatting
- **Low latency:** WebSocket-based streaming for minimal delay
- **Flexible:** Supports both streaming and file-based transcription

EOF
fi

echo ""
log_success "Installation completed successfully!"
echo ""
echo "📁 Project directory: $PROJECT_DIR"
echo "🐍 Python version: $($PYTHON_CMD --version)"
echo "🌐 API: Deepgram Streaming API"
echo "🖥️  Display server: $DISPLAY_SERVER"
echo ""
echo "🚀 Ready to use! Start with:"
echo "   export DEEPGRAM_API_KEY='your_api_key_here'"
echo "   cd $PROJECT_DIR && ./run.sh"
echo "📱 Hotkey: Super+Space (Windows key + Space)"
echo ""

if [[ "$DISPLAY_SERVER" == "wayland" ]] && ! groups "$USER" | grep -q input; then
    log_warning "For Wayland support, log out and back in for group changes to take effect"
fi

# Offer to start immediately
if [[ -n "$DEEPGRAM_API_KEY" ]]; then
    read -p "Start the speech-to-text service now? (Y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        log_info "Starting service..."
        ./run.sh
    fi
else
    log_warning "Set DEEPGRAM_API_KEY first, then run ./run.sh"
fi
