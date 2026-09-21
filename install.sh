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
YDOTOOL_VERSION="1.0.4"
YDOTOOL_URL="https://github.com/ReimuNotMoe/ydotool/releases/download/v$YDOTOOL_VERSION/ydotool-release-ubuntu-latest"
YDOTOOLD_URL="https://github.com/ReimuNotMoe/ydotool/releases/download/v$YDOTOOL_VERSION/ydotoold-release-ubuntu-latest"
YDOTOOL_SHA256="daa83507a596d6839b7467540382dbdc6e4bf64ebfa4f7d6416e877d9a522c0c"
YDOTOOLD_SHA256="3f14f96308935214c0fb154507360f7632e7deda1935dc2d538259fd9986ed36"

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

setup_ydotool_daemon() {
    local client daemon client_help daemon_output installed_version reply download_dir runtime_dir
    local user_systemd_dir="$HOME/.config/systemd/user"
    local service_file="$user_systemd_dir/ydotoold.service"
    local needs_upgrade=true

    client="$(command -v ydotool 2>/dev/null || true)"
    daemon="$(command -v ydotoold 2>/dev/null || true)"
    client_help="$([[ -n "$client" ]] && "$client" help 2>&1 || true)"
    installed_version=""
    if [[ "$client" == /usr/bin/ydotool && "$client_help" == *recorder* ]]; then
        installed_version="$(dpkg-query -W -f='${Version}' ydotool 2>/dev/null || true)"
    elif [[ -n "$daemon" ]]; then
        daemon_output="$(timeout 2 "$daemon" --version 2>/dev/null || true)"
        if [[ "$daemon_output" =~ v?([0-9]+\.[0-9]+\.[0-9]+) ]]; then
            installed_version="${BASH_REMATCH[1]}"
            if dpkg --compare-versions "$installed_version" ge "$YDOTOOL_VERSION"; then
                needs_upgrade=false
            fi
        fi
    fi

    if [[ "$needs_upgrade" == true ]]; then
        if [[ "$(uname -m)" != "x86_64" ]]; then
            print_warning "ydotool $YDOTOOL_VERSION prebuilt binaries are only available for x86_64"
            print_warning "Build ydotool $YDOTOOL_VERSION or newer from source, then rerun this installer"
            return 1
        fi
        if [[ -n "$client" ]]; then
            print_warning "The installed ydotool${installed_version:+ $installed_version} is incompatible with voice-typing"
        else
            print_warning "ydotool $YDOTOOL_VERSION or newer is required on Wayland"
        fi
        echo "The installer can disable existing ydotool services, install the official"
        echo "ydotool $YDOTOOL_VERSION client and daemon globally in /usr/local/bin, and configure"
        echo "one per-user daemon using the Wayland runtime socket."
        if [[ ! -t 0 ]]; then
            print_warning "Run the installer in a terminal to approve the ydotool upgrade"
            return 1
        fi
        read -r -p "Replace the old ydotool installation and services? [y/N] " reply
        if [[ ! "$reply" =~ ^[Yy]([Ee][Ss])?$ ]]; then
            print_warning "Keeping the existing ydotool installation"
            return 1
        fi

        download_dir="$(mktemp -d)" || {
            print_warning "Could not create a temporary download directory"
            return 1
        }
        trap 'rm -f -- "$download_dir/ydotool" "$download_dir/ydotoold"; rmdir "$download_dir" 2>/dev/null || true' RETURN
        print_info "Downloading official ydotool $YDOTOOL_VERSION binaries..."
        if ! curl -fL --retry 3 -o "$download_dir/ydotool" "$YDOTOOL_URL" ||
           ! curl -fL --retry 3 -o "$download_dir/ydotoold" "$YDOTOOLD_URL" ||
           ! printf '%s  %s\n%s  %s\n' \
                "$YDOTOOL_SHA256" "$download_dir/ydotool" \
                "$YDOTOOLD_SHA256" "$download_dir/ydotoold" | sha256sum -c -; then
            print_warning "Could not download and verify ydotool $YDOTOOL_VERSION"
            return 1
        fi

        systemctl --user disable --now ydotoold.service ydotool.service >/dev/null 2>&1 || true
        sudo systemctl disable --now ydotoold.service ydotool.service >/dev/null 2>&1 || true
        if ! sudo install -m 0755 "$download_dir/ydotool" /usr/local/bin/ydotool ||
           ! sudo install -m 0755 "$download_dir/ydotoold" /usr/local/bin/ydotoold; then
            print_warning "Could not install ydotool $YDOTOOL_VERSION into /usr/local/bin"
            return 1
        fi
        client="/usr/local/bin/ydotool"
        daemon="/usr/local/bin/ydotoold"
        installed_version="$YDOTOOL_VERSION"
        print_status "Installed ydotool $YDOTOOL_VERSION"
    else
        print_status "Found compatible ydotool $installed_version at $client"
    fi

    if ! mkdir -p "$user_systemd_dir" || ! {
        echo "# Managed by the voice-typing installer"
        echo "[Unit]"
        echo "Description=ydotool daemon (user)"
        echo
        echo "[Service]"
        echo "ExecStart=$daemon --socket-path=%t/.ydotool_socket"
        echo "Restart=on-failure"
        echo
        echo "[Install]"
        echo "WantedBy=default.target"
    } > "$service_file"; then
        print_warning "Could not create $service_file"
        return 1
    fi
    if ! systemctl --user daemon-reload >/dev/null 2>&1 ||
       ! systemctl --user enable ydotoold >/dev/null 2>&1 ||
       ! systemctl --user restart ydotoold >/dev/null 2>&1; then
        print_warning "Configured ydotoold, but it could not start; log out and back in after installation"
        return 1
    fi
    runtime_dir="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
    if ! printf '' | "$client" type --file - >/dev/null 2>&1; then
        print_warning "ydotool could not connect to the new daemon at $runtime_dir/.ydotool_socket"
        return 1
    fi
    print_status "Configured ydotool $installed_version user daemon"
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
        install_packages curl wtype wl-clipboard || dependencies_failed=true
        if ! sudo usermod -a -G input "$USER"; then
            print_warning "Could not add $USER to the input group"
            dependencies_failed=true
        fi
        setup_ydotool_daemon || dependencies_failed=true
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
    echo "  Start: $INSTALL_DIR/$BINARY_NAME --hotkey"
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
print_info "Press Super+] to start recording or run $BINARY_NAME directly"
