#!/bin/bash

# Voice Typing Uninstaller

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Paths
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/voice-typing"
BINARY_NAME="voice-typing"

print_status() {
    echo -e "${GREEN}✅ ${NC}$1"
}

print_info() {
    echo -e "${BLUE}ℹ️  ${NC}$1"
}

print_warning() {
    echo -e "${YELLOW}⚠️  ${NC}$1"
}

echo -e "${BLUE}🗑️ Voice Typing Uninstaller${NC}"
echo "===================================="

# Kill any running instances
print_info "Stopping any running instances..."
pkill -f "$BINARY_NAME" || true

# Remove binary
if [[ -f "$INSTALL_DIR/$BINARY_NAME" ]]; then
    rm "$INSTALL_DIR/$BINARY_NAME"
    print_status "Removed binary from $INSTALL_DIR"
fi

# Ask about config removal
echo
read -p "Remove configuration directory ($CONFIG_DIR)? [y/N]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [[ -d "$CONFIG_DIR" ]]; then
        rm -rf "$CONFIG_DIR"
        print_status "Removed configuration directory"
    fi
else
    print_info "Keeping configuration directory"
fi

# Remove GNOME hotkey
if command -v gsettings >/dev/null 2>&1; then
    print_info "Removing GNOME hotkey..."
    
    # Find and remove the custom keybinding
    custom_bindings=$(gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings 2>/dev/null || echo "[]")
    
    if [[ "$custom_bindings" != "[]" && "$custom_bindings" != "@as []" ]]; then
        # Look for our keybinding
        for binding in $(echo "$custom_bindings" | grep -o "'/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom[0-9]*/'"); do
            binding_clean=$(echo "$binding" | tr -d "'")
            cmd=$(gsettings get org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_clean" command 2>/dev/null || echo "")
            
            if [[ "$cmd" =~ voice-typing ]]; then
                # Remove this keybinding
                gsettings reset org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_clean" name 2>/dev/null || true
                gsettings reset org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_clean" command 2>/dev/null || true
                gsettings reset org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_clean" binding 2>/dev/null || true
                
                # Remove from the list
                new_bindings=$(echo "$custom_bindings" | sed "s|, *$binding||g" | sed "s|$binding, *||g" | sed "s|$binding||g")
                if [[ "$new_bindings" =~ ^\[.*\]$ ]]; then
                    gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "$new_bindings"
                fi
                
                print_status "Removed GNOME hotkey"
                break
            fi
        done
    fi
fi

echo
print_status "Uninstall completed!"
print_info "You may want to manually remove these dependencies if not used elsewhere:"
echo "  sudo apt remove portaudio19-dev wtype ydotool wl-clipboard xdotool xclip xsel"
