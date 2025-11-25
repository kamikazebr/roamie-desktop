#!/bin/bash
#
# Roamie Client Installer
# Usage: curl -sSL https://raw.githubusercontent.com/kamikazebr/roamie-desktop/main/scripts/install.sh | bash
#

set -e

REPO="kamikazebr/roamie-desktop"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="roamie"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}→${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux";;
        Darwin*)    echo "darwin";;
        MINGW*|MSYS*|CYGWIN*) echo "windows";;
        *)          error "Unsupported OS: $(uname -s)";;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64";;
        aarch64|arm64)  echo "arm64";;
        *)              error "Unsupported architecture: $(uname -m)";;
    esac
}

# Get latest release version
get_latest_version() {
    curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    echo ""
    echo "  ██████╗  ██████╗  █████╗ ███╗   ███╗██╗███████╗"
    echo "  ██╔══██╗██╔═══██╗██╔══██╗████╗ ████║██║██╔════╝"
    echo "  ██████╔╝██║   ██║███████║██╔████╔██║██║█████╗  "
    echo "  ██╔══██╗██║   ██║██╔══██║██║╚██╔╝██║██║██╔══╝  "
    echo "  ██║  ██║╚██████╔╝██║  ██║██║ ╚═╝ ██║██║███████╗"
    echo "  ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝╚══════╝"
    echo ""
    echo "  ██████╗ ███████╗██╗   ██╗"
    echo "  ██╔══██╗██╔════╝██║   ██║"
    echo "  ██║  ██║█████╗  ██║   ██║"
    echo "  ██║  ██║██╔══╝  ╚██╗ ██╔╝"
    echo "  ██████╔╝███████╗ ╚████╔╝ "
    echo "  ╚═════╝ ╚══════╝  ╚═══╝  "
    echo ""
    echo "       break free • code anywhere"
    echo ""

    OS=$(detect_os)
    ARCH=$(detect_arch)

    info "Detected: ${OS}/${ARCH}"

    # Get latest version
    info "Fetching latest version..."
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        error "Failed to fetch latest version"
    fi

    info "Latest version: ${VERSION}"

    # Build download URL
    FILENAME="${BINARY_NAME}-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        FILENAME="${FILENAME}.exe"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    # Download
    info "Downloading ${FILENAME}..."
    TMP_FILE=$(mktemp)

    if ! curl -sSL -o "$TMP_FILE" "$DOWNLOAD_URL"; then
        rm -f "$TMP_FILE"
        error "Failed to download from ${DOWNLOAD_URL}"
    fi

    # Install
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."

    if [ ! -w "$INSTALL_DIR" ]; then
        warn "Need sudo to install to ${INSTALL_DIR}"
        sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # Verify
    if command -v roamie &> /dev/null; then
        echo ""
        echo -e "${GREEN}✓ Roamie Client installed successfully!${NC}"
        echo ""
        roamie version
        echo ""
        echo "Get started:"
        echo "  sudo roamie auth login"
        echo ""
    else
        warn "Installed but 'roamie' not in PATH. You may need to add ${INSTALL_DIR} to your PATH."
    fi
}

main
