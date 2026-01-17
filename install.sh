#!/bin/sh
# SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
# SPDX-License-Identifier: MIT
set -e

REPO="vlebo/ctx"
INSTALL_DIR="/usr/local/bin"
BINARY="ctx"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info() { printf "${GREEN}==>${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}==>${NC} %s\n" "$1"; }
error() { printf "${RED}error:${NC} %s\n" "$1" >&2; exit 1; }

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported OS: $(uname -s)" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Check for required commands
check_deps() {
    for cmd in curl tar; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            error "$cmd is required but not installed"
        fi
    done
}

# Get latest release version
get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
        grep '"tag_name":' |
        sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    check_deps

    OS=$(detect_os)
    ARCH=$(detect_arch)

    info "Detected OS: $OS, Arch: $ARCH"

    info "Fetching latest version..."
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        error "Could not determine latest version"
    fi

    info "Latest version: $VERSION"

    # Build download URL
    TARBALL="ctx_${VERSION#v}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"

    info "Downloading $URL..."

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    curl -fsSL "$URL" -o "$TMP_DIR/$TARBALL" || error "Download failed"

    info "Extracting..."
    tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR"

    # Install binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
    else
        info "Requesting sudo to install to $INSTALL_DIR..."
        sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
    fi

    chmod +x "$INSTALL_DIR/$BINARY"

    info "Installed $BINARY to $INSTALL_DIR/$BINARY"

    echo ""
    info "Add shell integration to your ~/.zshrc or ~/.bashrc:"
    echo ""
    echo "    eval \"\$($BINARY shell-hook zsh)\""
    echo ""
    info "Then initialize ctx:"
    echo ""
    echo "    $BINARY init"
    echo ""
}

main
