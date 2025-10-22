#!/bin/bash

# Voice Typing Installation Script
# Supports Ubuntu/Debian with GNOME/Unity desktop environments

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/voice-typing"
BINARY_NAME="voice-typing"
HOTKEY_BINDING="<Super>bracketright"  # Super+]

# Parse command line arguments
FORCE_BUILD=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --build)
            FORCE_BUILD=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--build]"
            echo "  --build    Force building from source even if binary exists"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}🎤 Voice Typing Installer${NC}"
echo "=================================="

# Function to print status
print_status() {
    echo -e "${GREEN}✅ ${NC}$1"
}

print_warning() {
    echo -e "${YELLOW}⚠️  ${NC}$1"
}

print_error() {
    echo -e "${RED}❌ ${NC}$1"
}

print_info() {
    echo -e "${BLUE}ℹ️  ${NC}$1"
}

# Check if running on supported system
check_system() {
    print_info "Checking system compatibility..."
    
    if [[ "$OSTYPE" != "linux-gnu"* ]]; then
        print_error "This installer only supports Linux systems"
        exit 1
    fi
    
    # Check for package manager
    if ! command -v apt >/dev/null 2>&1; then
        print_warning "This installer is optimized for Ubuntu/Debian systems"
        print_info "You may need to install dependencies manually"
    fi
    
    print_status "System check passed"
}

# Install system dependencies
install_dependencies() {
    print_info "Installing system dependencies..."
    
    # Check if we need sudo
    if command -v apt >/dev/null 2>&1; then
        # Audio dependencies
        if ! sudo apt update; then
            print_warning "Failed to update package list, continuing anyway"
        fi
        if ! sudo apt install -y portaudio19-dev; then
            print_error "Failed to install portaudio19-dev - audio capture may not work"
            print_info "You may need to install it manually later"
        fi
        
        # Typing tools for Wayland/X11
        if [[ "$XDG_SESSION_TYPE" == "wayland" ]]; then
            print_info "Installing Wayland typing tools..."
            sudo apt install -y wtype ydotool wl-clipboard
            
            # Enable ydotool daemon
            if systemctl --user list-unit-files | grep -q ydotoold; then
                systemctl --user enable --now ydotoold || true
            else
                sudo systemctl enable --now ydotoold || true
            fi
            
            # Add user to input group for ydotool
            sudo usermod -a -G input "$USER"
            print_warning "You'll need to log out and back in for ydotool permissions to take effect"
        else
            print_info "Installing X11 typing tools..."
            sudo apt install -y xdotool xclip xsel
        fi
        
        # Notification support
        sudo apt install -y libnotify-bin
        
        print_status "Dependencies installed"
    else
        print_warning "Please install these dependencies manually:"
        echo "  - portaudio19-dev (audio capture)"
        echo "  - wtype, ydotool, wl-clipboard (Wayland typing)"
        echo "  - xdotool, xclip, xsel (X11 typing)"
        echo "  - libnotify-bin (notifications)"
    fi
}

# Build or verify the application binary
build_application() {
    # If --build flag is used, prefer building from source
    if [[ "$FORCE_BUILD" == "true" ]]; then
        if [[ ! -f "main.go" || ! -f "go.mod" ]]; then
            print_error "--build flag used but no source code present"
            print_error "Source files (main.go, go.mod) are required to build"
            exit 1
        fi
        
        print_info "Building Go application from source (--build flag)"
        
        if ! command -v go >/dev/null 2>&1; then
            print_error "Go is not installed. Please install Go 1.19+ first:"
            echo "  https://golang.org/doc/install"
            exit 1
        fi
        
        # Build the application
        if ! make build; then
            print_error "Build failed"
            exit 1
        fi
        
        if [[ ! -f "./$BINARY_NAME" ]]; then
            print_error "Build failed - binary not found"
            exit 1
        fi
        
        print_status "Application built successfully from source"
        return 0
    fi
    
    # Default behavior: prefer pre-built binary
    if [[ -f "./$BINARY_NAME" ]]; then
        print_status "Using pre-built binary: $BINARY_NAME"
        return 0
    fi
    
    # Fallback: build from source if available
    if [[ ! -f "main.go" || ! -f "go.mod" ]]; then
        print_error "No pre-built binary found and no source code present"
        print_error "This appears to be an incomplete distribution"
        print_error "Please download the complete package or a pre-built binary"
        exit 1
    fi
    
    print_info "Building Go application from source..."
    
    if ! command -v go >/dev/null 2>&1; then
        print_error "Go is not installed. Please install Go 1.19+ first:"
        echo "  https://golang.org/doc/install"
        exit 1
    fi
    
    # Build the application
    if ! make build; then
        print_error "Build failed"
        exit 1
    fi
    
    if [[ ! -f "./$BINARY_NAME" ]]; then
        print_error "Build failed - binary not found"
        exit 1
    fi
    
    print_status "Application built successfully from source"
}

# Install binary and config
install_files() {
    print_info "Installing files..."
    
    # Create directories
    if ! mkdir -p "$INSTALL_DIR"; then
        print_error "Failed to create install directory: $INSTALL_DIR"
        exit 1
    fi
    if ! mkdir -p "$CONFIG_DIR"; then
        print_error "Failed to create config directory: $CONFIG_DIR"
        exit 1
    fi
    
    # Install binary
    if ! cp "./$BINARY_NAME" "$INSTALL_DIR/"; then
        print_error "Failed to copy binary to $INSTALL_DIR"
        exit 1
    fi
    if ! chmod +x "$INSTALL_DIR/$BINARY_NAME"; then
        print_error "Failed to make binary executable"
        exit 1
    fi
    
    # Install config if it doesn't exist
    if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
        if [[ -f "config.json" ]]; then
            if ! cp "config.json" "$CONFIG_DIR/"; then
                print_error "Failed to copy config.json"
                exit 1
            fi
        else
            if ! cp "config.example.json" "$CONFIG_DIR/config.json"; then
                print_error "Failed to copy config.example.json"
                exit 1
            fi
            print_warning "Please edit $CONFIG_DIR/config.json with your Deepgram API key"
        fi
    else
        print_info "Existing config found, not overwriting"
    fi
    
    # Make sure ~/.local/bin is in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        print_warning "Added $INSTALL_DIR to PATH in ~/.bashrc"
        print_warning "Run 'source ~/.bashrc' or restart your terminal"
    fi
    
    print_status "Files installed to $INSTALL_DIR"
}

# Detect desktop environment
detect_desktop() {
    local desktop="$XDG_CURRENT_DESKTOP"
    local session="$XDG_SESSION_DESKTOP"
    
    if [[ "$desktop" =~ ^(GNOME|Unity|ubuntu)$ ]] || [[ "$session" =~ ^(gnome|ubuntu)$ ]]; then
        echo "gnome"
    elif [[ "$desktop" =~ ^(KDE|plasma)$ ]]; then
        echo "kde"
    elif [[ "$desktop" =~ ^(XFCE|xfce)$ ]]; then
        echo "xfce"
    elif [[ "$desktop" =~ ^(Hyprland|hyprland)$ ]]; then
        echo "hyprland"
    else
        echo "unknown"
    fi
}

# Setup GNOME hotkeys
setup_gnome_hotkey() {
    print_info "Setting up GNOME hotkeys..."
    
    # Setup start/toggle hotkey (Super+])
    setup_gnome_keybinding "Voice Typing" "$INSTALL_DIR/$BINARY_NAME --hotkey" "<Super>bracketright"
    
    # Setup stop hotkey (Super+[)
    setup_gnome_keybinding "Voice Typing Stop" "$INSTALL_DIR/$BINARY_NAME --stopkey" "<Super>bracketleft"
}

# Helper function to setup a single GNOME keybinding
setup_gnome_keybinding() {
    local name="$1"
    local cmd="$2"
    local binding="$3"
    
    # Find available custom keybinding slot
    local slot=0
    while true; do
        local slot_name=$(gsettings get "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom$slot/" name 2>/dev/null || echo "''")
        
        if [[ "$slot_name" == "''" || "$slot_name" == "" || "$slot_name" == "@ms nothing" ]]; then
            break  # Found empty slot
        fi
        
        ((slot++))
        if [[ $slot -gt 20 ]]; then
            print_error "Too many custom keybindings, cannot add more"
            return 1
        fi
    done
    
    local key_path="/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom$slot/"
    
    # Set the keybinding properties
    if ! gsettings set "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" name "$name" 2>/dev/null; then
        print_error "Failed to set keybinding name"
        return 1
    fi
    if ! gsettings set "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" command "$cmd" 2>/dev/null; then
        print_error "Failed to set keybinding command"
        return 1
    fi
    if ! gsettings set "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" binding "$binding" 2>/dev/null; then
        print_error "Failed to set keybinding shortcut"
        return 1
    fi
    
    # Get current keybindings list
    local existing_bindings=$(gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings 2>/dev/null || echo "@as []")
    
    # Add to the list of custom keybindings
    if [[ "$existing_bindings" == "[]" || "$existing_bindings" == "@as []" ]]; then
        if ! gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "['$key_path']" 2>/dev/null; then
            print_error "Failed to set custom keybindings list"
            return 1
        fi
    else
        # Extract just the content inside the brackets, then split into individual items
        local existing_items=$(echo "$existing_bindings" | sed 's/@as //g' | sed 's/^\[//g' | sed 's/\]$//g')
        
        # Build new list by appending to existing items
        if [[ -n "$existing_items" ]]; then
            if ! gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "['$key_path', $existing_items]" 2>/dev/null; then
                print_error "Failed to update custom keybindings list"
                return 1
            fi
        else
            if ! gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "['$key_path']" 2>/dev/null; then
                print_error "Failed to set custom keybindings list"
                return 1
            fi
        fi
    fi
    
    # Verify the setup
    sleep 0.5
    local verify_name=$(gsettings get "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" name 2>/dev/null || echo "")
    
    if [[ "$verify_name" =~ $name ]]; then
        print_status "Configured: $binding → $name"
    else
        print_warning "Setup may have failed for '$name'"
    fi
}

# Setup hotkey based on desktop environment
setup_hotkey() {
    local desktop=$(detect_desktop)
    
    print_info "Detected desktop environment: $desktop"
    
    case "$desktop" in
        "gnome")
            if command -v gsettings >/dev/null 2>&1; then
                setup_gnome_hotkey
            else
                print_warning "gsettings not found, cannot auto-configure hotkey"
                show_manual_hotkey_instructions
            fi
            ;;
        "kde")
            print_info "KDE detected - hotkey setup requires manual configuration"
            show_kde_instructions
            ;;
        "xfce")
            print_info "XFCE detected - hotkey setup requires manual configuration"
            show_xfce_instructions
            ;;
        "hyprland")
            print_info "Hyprland detected - hotkey setup requires manual configuration"
            show_hyprland_instructions
            ;;
        *)
            print_warning "Unknown desktop environment: $desktop"
            show_manual_hotkey_instructions
            ;;
    esac
}

# Manual hotkey instructions
show_manual_hotkey_instructions() {
    print_info "Manual hotkey setup instructions:"
    echo
    echo "Start/Toggle Recording:"
    echo "  Command: $INSTALL_DIR/$BINARY_NAME --hotkey"
    echo "  Suggested hotkey: Super+] (Windows key + right bracket)"
    echo
    echo "Stop Recording (graceful):"
    echo "  Command: $INSTALL_DIR/$BINARY_NAME --stopkey"
    echo "  Suggested hotkey: Super+[ (Windows key + left bracket)"
    echo
    echo "Set these up in your desktop environment's keyboard shortcuts settings."
}

show_kde_instructions() {
    echo
    print_info "KDE Plasma hotkey setup:"
    echo "1. Open System Settings → Shortcuts → Custom Shortcuts"
    echo "2. Click 'Edit' → 'New' → 'Global Shortcut' → 'Command/URL'"
    echo
    echo "For Start/Toggle (Super+]):"
    echo "  - Name: 'Voice Typing'"
    echo "  - Command: $INSTALL_DIR/$BINARY_NAME --hotkey"
    echo "  - Shortcut: Meta+]"
    echo
    echo "For Stop (Super+[):"
    echo "  - Name: 'Voice Typing Stop'"
    echo "  - Command: $INSTALL_DIR/$BINARY_NAME --stopkey"
    echo "  - Shortcut: Meta+["
}

show_xfce_instructions() {
    echo
    print_info "XFCE hotkey setup:"
    echo "1. Open Settings → Keyboard → Application Shortcuts"
    echo "2. Click 'Add' for each command:"
    echo
    echo "Start/Toggle (Super+]):"
    echo "  - Command: $INSTALL_DIR/$BINARY_NAME --hotkey"
    echo "  - Shortcut: Super+]"
    echo
    echo "Stop (Super+[):"
    echo "  - Command: $INSTALL_DIR/$BINARY_NAME --stopkey"
    echo "  - Shortcut: Super+["
}

show_hyprland_instructions() {
    echo
    print_info "Hyprland hotkey setup:"
    echo "Add these lines to your ~/.config/hypr/hyprland.conf:"
    echo "bind = SUPER, bracketright, exec, $INSTALL_DIR/$BINARY_NAME --hotkey"
    echo "bind = SUPER, bracketleft, exec, $INSTALL_DIR/$BINARY_NAME --stopkey"
    echo "Then reload Hyprland config"
}

# Check configuration
check_config() {
    local config_file="$CONFIG_DIR/config.json"
    
    if [[ -f "$config_file" ]]; then
        if grep -q "your_deepgram_api_key_here\|your_actual_api_key_here" "$config_file"; then
            print_warning "Please update your Deepgram API key in: $config_file"
            print_info "Get a free API key at: https://console.deepgram.com/signup"
            return 1
        else
            print_status "Configuration file looks good"
            return 0
        fi
    else
        print_error "Configuration file not found: $config_file"
        return 1
    fi
}

# Test installation
test_installation() {
    print_info "Testing installation..."
    
    if command -v "$INSTALL_DIR/$BINARY_NAME" >/dev/null 2>&1; then
        print_status "Binary is accessible in PATH"
    else
        print_warning "Binary not in PATH - you may need to restart your terminal"
    fi
    
    # Test config
    if check_config; then
        print_status "Ready to use!"
    else
        print_warning "Please configure your Deepgram API key before using"
    fi
}

# Main installation process
main() {
    echo
    check_system
    echo
    
    install_dependencies
    echo
    
    build_application
    echo
    
    install_files
    echo
    
    setup_hotkey
    echo
    
    test_installation
    echo
    
    print_status "Installation completed!"
    echo
    print_info "Usage:"
    echo "  • Single session: $BINARY_NAME"
    echo "  • Start/Toggle: Press Super+] (or your configured hotkey)"
    echo "  • Stop (graceful): Press Super+[ (or your configured stop hotkey)"
    echo "  • Config file: $CONFIG_DIR/config.json"
    echo
    print_info "Voice commands:"
    echo "  • 'newline' - Insert line break"
    echo "  • 'new paragraph' - Insert paragraph break"
    echo "  • 'undo that' - Remove last phrase"
    echo "  • 'stop voice' - End recording"
    echo
    
    if [[ "$XDG_SESSION_TYPE" == "wayland" ]] && ! groups | grep -q input; then
        print_warning "You'll need to log out and back in for ydotool permissions to take effect"
    fi
}

# Run main installation
main "$@"
