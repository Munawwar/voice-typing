#!/bin/bash

set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/voice-typing"
BINARY_NAME="voice-typing"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
FORCE_BUILD=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --build) FORCE_BUILD=true ;;
        --help|-h)
            echo "Usage: $0 [--build]"
            echo "  --build    Require source code and build it"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
    shift
done

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

install_packages() {
    if ! sudo apt install -y "$@"; then
        print_warning "Could not install packages: $*"
        return 1
    fi
}

setup_ydotool_user_service() {
    local daemon user_systemd_dir service_file
    daemon="$(command -v ydotoold 2>/dev/null || true)"
    user_systemd_dir="$HOME/.config/systemd/user"
    service_file="$user_systemd_dir/ydotoold.service"
    if [[ -z "$daemon" ]]; then
        print_warning "ydotoold is unavailable; key injection may not work"
        return 1
    fi
    if ! mkdir -p "$user_systemd_dir"; then
        print_warning "Could not create $user_systemd_dir"
        return 1
    fi

    if [[ ! -e "$service_file" ]]; then
        if ! {
            echo "# Managed by the voice-typing installer"
            echo "[Unit]"
            echo "Description=ydotool daemon (user)"
            echo
            echo "[Service]"
            echo "ExecStart=$daemon -p %t/.ydotool_socket -P 0600"
            echo "Restart=on-failure"
            echo
            echo "[Install]"
            echo "WantedBy=default.target"
        } > "$service_file"; then
            print_warning "Could not create $service_file"
            return 1
        fi
    fi
    if ! systemctl --user daemon-reload >/dev/null 2>&1 ||
       ! systemctl --user enable --now ydotoold >/dev/null 2>&1; then
        print_warning "Could not start the user ydotoold service"
        return 1
    fi
    print_status "Configured user ydotoold service"
}

setup_ydotool_daemon() {
    print_info "Configuring ydotool daemon..."
    if systemctl --user list-unit-files >/dev/null 2>&1; then
        if systemctl --user list-unit-files 2>/dev/null | grep -q '^ydotoold\.service'; then
            systemctl --user enable --now ydotoold >/dev/null 2>&1 && {
                print_status "Enabled user ydotoold service"
                return
            }
        elif systemctl --user list-unit-files 2>/dev/null | grep -q '^ydotool\.service'; then
            systemctl --user enable --now ydotool >/dev/null 2>&1 && {
                print_status "Enabled user ydotool service"
                return
            }
        elif setup_ydotool_user_service; then
            return
        fi
    fi

    if systemctl list-unit-files 2>/dev/null | grep -q '^ydotoold\.service'; then
        sudo systemctl enable --now ydotoold && {
            print_status "Enabled system ydotoold service"
            return
        }
    elif systemctl list-unit-files 2>/dev/null | grep -q '^ydotool\.service'; then
        sudo systemctl enable --now ydotool && {
            print_status "Enabled system ydotool service"
            return
        }
    fi
    print_warning "No working ydotool service was found"
}

install_application() {
    if [[ -f "$SCRIPT_DIR/main.go" && -f "$SCRIPT_DIR/go.mod" ]]; then
        if ! command -v go >/dev/null 2>&1; then
            print_error "Go 1.24.4 or newer is required to build from source"
            exit 1
        fi
        print_info "Building current source..."
        if ! go -C "$SCRIPT_DIR" build -buildvcs=false -trimpath -ldflags="-buildid=" -o "$SCRIPT_DIR/$BINARY_NAME" .; then
            print_error "Build failed"
            exit 1
        fi
    elif [[ "$FORCE_BUILD" == true ]]; then
        print_error "--build requires main.go and go.mod"
        exit 1
    elif [[ ! -x "$SCRIPT_DIR/$BINARY_NAME" ]]; then
        print_error "No source code or prebuilt $BINARY_NAME binary was found"
        exit 1
    else
        print_status "Using packaged $BINARY_NAME binary"
    fi

    print_info "Installing files..."
    if ! mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" ||
       ! install -m 0755 "$SCRIPT_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"; then
        print_error "Could not install the application"
        exit 1
    fi

    if [[ ! -f "$CONFIG_DIR/config.json" ]]; then
        if [[ -f "$SCRIPT_DIR/config.json" ]]; then
            install -m 0600 "$SCRIPT_DIR/config.json" "$CONFIG_DIR/config.json"
        elif ! install -m 0600 "$SCRIPT_DIR/config.example.json" "$CONFIG_DIR/config.json"; then
            print_error "Could not install the example configuration"
            exit 1
        fi
        print_warning "Add your Deepgram API key to $CONFIG_DIR/config.json"
    else
        print_info "Keeping existing configuration"
    fi

    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]] &&
       ! grep -Fq 'export PATH="$HOME/.local/bin:$PATH"' "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"
        print_warning "Added $INSTALL_DIR to PATH in ~/.bashrc"
    fi
    print_status "Installed $INSTALL_DIR/$BINARY_NAME"
}

setup_gnome_keybinding() {
    local name="$1" command="$2" binding="$3"
    local key_path="" existing_paths existing_command slot
    existing_paths="$(gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings 2>/dev/null || echo '@as []')"

    while IFS= read -r candidate; do
        candidate="${candidate//\'/}"
        existing_command="$(gsettings get "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$candidate" command 2>/dev/null || true)"
        existing_command="${existing_command#\'}"
        existing_command="${existing_command%\'}"
        if [[ "$existing_command" == "$command" ]]; then
            key_path="$candidate"
            break
        fi
    done < <(grep -o "'/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/[^']*/'" <<< "$existing_paths")

    if [[ -z "$key_path" ]]; then
        for ((slot=0; slot<=100; slot++)); do
            key_path="/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/custom$slot/"
            if ! grep -Fq "'$key_path'" <<< "$existing_paths"; then
                break
            fi
        done
    fi

    if ! gsettings set "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" name "$name" ||
       ! gsettings set "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" command "$command" ||
       ! gsettings set "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$key_path" binding "$binding"; then
        print_warning "Could not configure $name"
        return 1
    fi
    if ! grep -Fq "'$key_path'" <<< "$existing_paths"; then
        local paths="${existing_paths#@as }"
        if [[ "$paths" == "[]" ]]; then
            paths="['$key_path']"
        else
            paths="${paths%]}, '$key_path']"
        fi
        gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "$paths"
    fi
    print_status "Configured: $binding → $name"
}

desktop="${XDG_CURRENT_DESKTOP:-${XDG_SESSION_DESKTOP:-unknown}}"
desktop_lower="${desktop,,}"

echo -e "${BLUE}🎤 Voice Typing Installer${NC}"
echo "=================================="

if [[ "$OSTYPE" != "linux-gnu"* ]]; then
    print_error "This installer supports Linux only"
    exit 1
fi

print_info "Installing system dependencies..."
if command -v apt >/dev/null 2>&1; then
    dependencies_failed=false
    sudo apt update || print_warning "Could not refresh the apt package index"
    install_packages portaudio19-dev libnotify-bin || dependencies_failed=true
    if [[ "${XDG_SESSION_TYPE:-}" == "wayland" ]]; then
        install_packages wtype ydotool wl-clipboard || dependencies_failed=true
        setup_ydotool_daemon
        if ! sudo usermod -a -G input "$USER"; then
            print_warning "Could not add $USER to the input group"
            dependencies_failed=true
        fi
    else
        install_packages xdotool xclip xsel || dependencies_failed=true
    fi
    if [[ "$dependencies_failed" == true ]]; then
        print_warning "Installation will continue, but some system dependencies are missing"
    else
        print_status "Dependencies installed"
    fi
else
    print_warning "Install PortAudio, libnotify, and a typing tool for your display server manually"
fi
install_application

print_info "Detected desktop environment: $desktop"
if [[ "$desktop_lower" =~ (^|:)(gnome|unity|ubuntu)(:|$) ]] &&
   command -v gsettings >/dev/null 2>&1; then
    setup_gnome_keybinding "Voice Typing" "$INSTALL_DIR/$BINARY_NAME --hotkey" "<Super>bracketright"
    setup_gnome_keybinding "Voice Typing Stop" "$INSTALL_DIR/$BINARY_NAME --stopkey" "<Super>bracketleft"
else
    print_info "Configure these two application shortcuts in $desktop:"
    echo
    echo "  Start/Toggle: $INSTALL_DIR/$BINARY_NAME --hotkey"
    echo "  Suggested shortcut: Super+]"
    echo
    echo "  Stop: $INSTALL_DIR/$BINARY_NAME --stopkey"
    echo "  Suggested shortcut: Super+["
    if [[ "$desktop_lower" == *hyprland* ]]; then
        echo
        echo "Hyprland configuration:"
        echo "  bind = SUPER, bracketright, exec, $INSTALL_DIR/$BINARY_NAME --hotkey"
        echo "  bind = SUPER, bracketleft, exec, $INSTALL_DIR/$BINARY_NAME --stopkey"
    fi
fi

if grep -q 'your_deepgram_api_key_here\|your_actual_api_key_here' "$CONFIG_DIR/config.json"; then
    print_warning "Add a Deepgram API key to $CONFIG_DIR/config.json before using voice typing"
else
    print_status "Configuration contains an API key"
fi
if [[ "${XDG_SESSION_TYPE:-}" == "wayland" ]] && ! id -nG | tr ' ' '\n' | grep -qx input; then
    print_warning "Log out and back in for ydotool input-group access"
fi

print_status "Installation completed"
print_info "Press Super+] to toggle recording or run $BINARY_NAME directly"
