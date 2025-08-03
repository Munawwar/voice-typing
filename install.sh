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

# Install packages with error handling
log_info "Installing essential packages..."
ESSENTIAL_PACKAGES=(
    "build-essential"
    "libnotify-bin"
    "python3-venv"
    "python3-dev"
    "python3-pip"
    "git"
    "curl"
    "wget"
)

# Install essential packages first
sudo apt install -y "${ESSENTIAL_PACKAGES[@]}"

# Install typing tools with fallback handling
log_info "Installing typing tools..."
TYPING_TOOLS_INSTALLED=()

if [[ "$DISPLAY_SERVER" == "wayland" ]]; then
    log_info "Installing Wayland typing tools..."
    
    # Try to install wtype (may not be in standard repos)
    if sudo apt install -y wtype 2>/dev/null; then
        TYPING_TOOLS_INSTALLED+=("wtype")
        log_success "wtype installed successfully"
    else
        log_warning "wtype not available in repositories"
        log_info "You can install manually from: https://github.com/atx/wtype"
    fi
    
    # Try to install ydotool
    if sudo apt install -y ydotool 2>/dev/null; then
        TYPING_TOOLS_INSTALLED+=("ydotool")
        log_success "ydotool installed successfully"
    else
        log_warning "ydotool not available in repositories"
    fi
    
    # Install xdotool as fallback
    if sudo apt install -y xdotool 2>/dev/null; then
        TYPING_TOOLS_INSTALLED+=("xdotool")
        log_success "xdotool installed as fallback"
    fi
else
    log_info "Installing X11 typing tools..."
    
    # Install xdotool (primary for X11)
    if sudo apt install -y xdotool 2>/dev/null; then
        TYPING_TOOLS_INSTALLED+=("xdotool")
        log_success "xdotool installed successfully"
    else
        log_error "Failed to install xdotool - this is required for X11"
        exit 1
    fi
    
    # Install Wayland tools as backup
    sudo apt install -y wtype ydotool 2>/dev/null || log_warning "Wayland tools installation optional"
fi

if [[ ${#TYPING_TOOLS_INSTALLED[@]} -eq 0 ]]; then
    log_error "No typing tools were installed successfully!"
    log_info "Manual installation required:"
    log_info "- For X11: sudo apt install xdotool"
    log_info "- For Wayland: install wtype and ydotool manually"
    exit 1
else
    log_success "Typing tools installed: ${TYPING_TOOLS_INSTALLED[*]}"
fi

# Install/upgrade ydotool for better Wayland support
log_info "Checking ydotool installation..."

# Check if ydotool is installed via APT (old version)
APT_YDOTOOL_VERSION=""
if apt list --installed ydotool 2>/dev/null | grep -q "ydotool/"; then
    APT_YDOTOOL_VERSION=$(apt list --installed ydotool 2>/dev/null | grep "ydotool/" | cut -d' ' -f2)
    log_warning "Found old ydotool version from APT: $APT_YDOTOOL_VERSION"
    log_info "Ubuntu's ydotool package is outdated and causes latency issues"
    
    read -p "Remove old ydotool and install latest version v1.0.4? (Y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        log_info "Removing old ydotool..."
        sudo apt remove -y ydotool ydotoold 2>/dev/null || true
        sudo apt autoremove -y 2>/dev/null || true
        INSTALL_LATEST_YDOTOOL=true
    else
        log_warning "Keeping old ydotool - may have latency issues"
        INSTALL_LATEST_YDOTOOL=false
    fi
else
    # No APT version, check if we have a newer version already
    if command -v ydotool >/dev/null 2>&1; then
        CURRENT_VERSION=$(ydotool help 2>&1 | head -1 | grep -o "v[0-9]\+\.[0-9]\+\.[0-9]\+" || echo "unknown")
        if [[ "$CURRENT_VERSION" == "v1.0.4" ]]; then
            log_success "Latest ydotool v1.0.4 already installed"
            INSTALL_LATEST_YDOTOOL=false
        else
            log_info "Found ydotool version: $CURRENT_VERSION (upgrading to v1.0.4)"
            INSTALL_LATEST_YDOTOOL=true
        fi
    else
        log_info "ydotool not found - installing latest version v1.0.4"
        INSTALL_LATEST_YDOTOOL=true
    fi
fi

# Install latest ydotool if needed
if [[ "$INSTALL_LATEST_YDOTOOL" == "true" ]]; then
    log_info "Installing latest ydotool v1.0.4..."
    
    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # Download latest binaries
    log_info "Downloading ydotool binaries..."
    if wget -O ydotool https://github.com/ReimuNotMoe/ydotool/releases/download/v1.0.4/ydotool-release-ubuntu-latest 2>/dev/null &&
       wget -O ydotoold https://github.com/ReimuNotMoe/ydotool/releases/download/v1.0.4/ydotoold-release-ubuntu-latest 2>/dev/null; then
        
        # Make executable and install
        chmod +x ydotool ydotoold
        sudo cp ydotool /usr/local/bin/ydotool
        sudo cp ydotoold /usr/local/bin/ydotoold
        
        log_success "ydotool v1.0.4 installed successfully"
        
        # Create systemd service
        log_info "Setting up ydotoold daemon..."
        sudo tee /etc/systemd/system/ydotoold.service > /dev/null << 'EOF'
[Unit]
Description=ydotool daemon

[Service]
Type=simple
ExecStart=/usr/local/bin/ydotoold
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF
        
        # Start and enable service
        sudo systemctl daemon-reload
        if sudo systemctl enable ydotoold 2>/dev/null && sudo systemctl start ydotoold 2>/dev/null; then
            log_success "ydotoold daemon started successfully"
        else
            log_warning "Failed to start ydotoold daemon - will work with latency"
        fi
        
    else
        log_error "Failed to download ydotool binaries"
        log_info "Falling back to APT installation..."
        sudo apt install -y ydotool 2>/dev/null || log_warning "APT installation also failed"
    fi
    
    # Cleanup
    cd - > /dev/null
    rm -rf "$TEMP_DIR"
fi

# Configure user permissions
if command -v ydotool >/dev/null 2>&1; then
    if sudo usermod -a -G input "$USER" 2>/dev/null; then
        log_success "Added user to input group"
        log_info "You may need to log out and back in for group changes to take effect"
    else
        log_warning "Failed to add user to input group - ydotool may need manual setup"
    fi
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
    else:
        print("⚠️  Some tests failed. Check the output above.")
        if not os.environ.get('DEEPGRAM_API_KEY'):
            print("💡 Don't forget to set your DEEPGRAM_API_KEY!")
        sys.exit(1)
EOF

# Run test
python test_installation.py

# Step 11: Setup GNOME hotkey for Wayland users
log_info "Setting up desktop hotkey integration..."

# Create command that activates venv and runs the script
PROJECT_DIR="$(pwd)"

# Include DEEPGRAM_API_KEY in the command if it's set
if [[ -n "$DEEPGRAM_API_KEY" ]]; then
    SCRIPT_COMMAND="/bin/bash -c \"cd $PROJECT_DIR && source venv/bin/activate && env DEEPGRAM_API_KEY='$DEEPGRAM_API_KEY' python speech_hotkey.py --hotkey\""
    log_success "DEEPGRAM_API_KEY will be included in hotkey command"
else
    SCRIPT_COMMAND="/bin/bash -c \"cd $PROJECT_DIR && source venv/bin/activate && python speech_hotkey.py --hotkey\""
    log_warning "DEEPGRAM_API_KEY not set - hotkey may not work until API key is configured"
    log_info "After setting your API key, re-run the install script to update the hotkey command"
fi

# Check if we're on GNOME (common on Ubuntu)
if [[ "$XDG_CURRENT_DESKTOP" == *"GNOME"* ]] || [[ "$DESKTOP_SESSION" == *"gnome"* ]] || command -v gnome-shell >/dev/null 2>&1; then
    log_info "GNOME detected - setting up Super+] hotkey..."
    
    # Find an available custom shortcut slot
    SHORTCUT_INDEX=""
    for i in {0..9}; do
        EXISTING_NAME=$(gsettings get org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom$i/ name 2>/dev/null)
        if [[ -z "$EXISTING_NAME" ]] || [[ "$EXISTING_NAME" == "''" ]]; then
            SHORTCUT_INDEX=$i
            break
        fi
    done
    
    if [[ -n "$SHORTCUT_INDEX" ]]; then
        # Set up the custom keybinding
        KEYBINDING_PATH="/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom$SHORTCUT_INDEX/"
        
        # Configure the shortcut
        if gsettings set org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$KEYBINDING_PATH name 'Speech-to-Text Toggle' 2>/dev/null &&
           gsettings set org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$KEYBINDING_PATH command "$SCRIPT_COMMAND" 2>/dev/null &&
           gsettings set org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$KEYBINDING_PATH binding '<Super>bracketright' 2>/dev/null; then
            
            # Add to the list of custom keybindings
            CURRENT_KEYBINDINGS=$(gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings 2>/dev/null)
            if [[ "$CURRENT_KEYBINDINGS" == "@as []" ]]; then
                # No existing keybindings
                if gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "['$KEYBINDING_PATH']" 2>/dev/null; then
                    log_success "GNOME hotkey configured: Super+] → Speech-to-Text"
                    log_info "Hotkey will work immediately in GNOME"
                else
                    log_warning "Hotkey created but failed to activate"
                fi
            else
                # Add to existing keybindings if not already present
                if [[ "$CURRENT_KEYBINDINGS" != *"$KEYBINDING_PATH"* ]]; then
                    NEW_KEYBINDINGS=$(echo "$CURRENT_KEYBINDINGS" | sed "s|]|, \"$KEYBINDING_PATH\"]|g")
                    if gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "$NEW_KEYBINDINGS" 2>/dev/null; then
                        log_success "GNOME hotkey configured: Super+] → Speech-to-Text"
                        log_info "Hotkey will work immediately in GNOME"
                    else
                        log_warning "Hotkey created but failed to activate"
                    fi
                else
                    log_success "GNOME hotkey already configured: Super+] → Speech-to-Text"
                fi
            fi
        else
            log_error "Failed to configure hotkey settings"
            log_info "Manually add shortcut: Settings > Keyboard > Custom Shortcuts"
            log_info "Command: $SCRIPT_COMMAND"
            log_info "Shortcut: Super+]"
        fi
    else
        log_warning "Could not find available custom shortcut slot"
        log_info "Manually add shortcut: Settings > Keyboard > Custom Shortcuts"
        log_info "Command: $SCRIPT_COMMAND"
        log_info "Shortcut: Super+]"
    fi
    
elif [[ "$XDG_CURRENT_DESKTOP" == *"KDE"* ]] || [[ "$DESKTOP_SESSION" == *"kde"* ]]; then
    log_info "KDE detected - hotkey setup requires manual configuration"
    log_info "Go to: System Settings > Shortcuts > Custom Shortcuts"
    log_info "Add: Super+] → $SCRIPT_COMMAND"
    
else
    log_info "Desktop environment: $XDG_CURRENT_DESKTOP"
    log_info "For global hotkeys, add this to your desktop environment:"
    log_info "Command: $SCRIPT_COMMAND"
    log_info "Shortcut: Super+]"
fi

# Final setup
log_info "Final setup..."

# Display next steps
echo ""
echo "========================================="
echo "🎉 Installation completed successfully!"
echo ""
echo "Next steps:"
echo "1. $(tput setaf 3)⚠️  IMPORTANT: Log out and back in (or reboot) for ydotool permissions to take effect!$(tput sgr0)"
echo "2. DEEPGRAM_API_KEY='your_api_key_here' ./run.sh"
echo ""

