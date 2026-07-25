#!/bin/bash

set -uo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/voice-typing"
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

if [[ -x "$INSTALL_DIR/$BINARY_NAME" ]]; then
    print_info "Stopping the active recording, if any..."
    "$INSTALL_DIR/$BINARY_NAME" --stopkey || print_warning "Could not stop the active recording"
    if rm "$INSTALL_DIR/$BINARY_NAME"; then
        print_status "Removed $INSTALL_DIR/$BINARY_NAME"
    else
        print_warning "Could not remove $INSTALL_DIR/$BINARY_NAME"
    fi
fi

read -r -p "Remove configuration directory ($CONFIG_DIR)? [y/N]: " reply
if [[ "$reply" =~ ^[Yy]$ ]] && [[ -d "$CONFIG_DIR" ]]; then
    rm -rf "$CONFIG_DIR"
    print_status "Removed configuration directory"
else
    print_info "Keeping configuration directory"
fi

if command -v gsettings >/dev/null 2>&1; then
    bindings="$(gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings 2>/dev/null || echo '@as []')"
    kept=()
    removed=0
    while IFS= read -r candidate; do
        candidate="${candidate//\'/}"
        command="$(gsettings get "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$candidate" command 2>/dev/null || true)"
        if [[ "$command" == *"$INSTALL_DIR/$BINARY_NAME"* ]]; then
            gsettings reset "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$candidate" name 2>/dev/null || true
            gsettings reset "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$candidate" command 2>/dev/null || true
            gsettings reset "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$candidate" binding 2>/dev/null || true
            ((removed++))
        else
            kept+=("'$candidate'")
        fi
    done < <(grep -o "'/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/[^']*/'" <<< "$bindings")

    new_bindings="@as []"
    if (( ${#kept[@]} > 0 )); then
        joined="$(IFS=,; echo "${kept[*]}")"
        new_bindings="[$joined]"
    fi
    if (( removed > 0 )); then
        gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "$new_bindings"
        print_status "Removed $removed GNOME keybinding(s)"
    fi
fi

service_file="$HOME/.config/systemd/user/ydotoold.service"
if [[ -f "$service_file" ]] && grep -Fq '# Managed by the voice-typing installer' "$service_file"; then
    systemctl --user disable --now ydotoold >/dev/null 2>&1 || true
    if rm "$service_file"; then
        systemctl --user daemon-reload >/dev/null 2>&1 || true
        print_status "Removed the managed ydotoold service"
    fi
fi

print_status "Uninstall completed"
print_info "Optional dependency cleanup:"
echo "  sudo apt remove portaudio19-dev wtype ydotool wl-clipboard xdotool xclip xsel"
