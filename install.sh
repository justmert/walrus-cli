#!/bin/bash

# Walrus CLI Installation Script
# Detects OS and architecture, downloads the appropriate binary

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="walrus-rclone/mvp"
BINARY_NAME="walrus-cli"
INSTALL_DIR="${INSTALL_DIR:-$HOME/bin}"

# Functions
log_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

log_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

log_info() {
    echo -e "${BLUE}→ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# Detect OS and Architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)     OS_TYPE="linux";;
        Darwin*)    OS_TYPE="darwin";;
        CYGWIN*|MINGW*|MSYS*) OS_TYPE="windows";;
        *)          log_error "Unsupported operating system: $OS"; exit 1;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH_TYPE="amd64";;
        arm64|aarch64) ARCH_TYPE="arm64";;
        *) log_error "Unsupported architecture: $ARCH"; exit 1;;
    esac

    PLATFORM="${OS_TYPE}-${ARCH_TYPE}"

    # Windows binaries have .exe extension
    if [ "$OS_TYPE" = "windows" ]; then
        BINARY_FILE="${BINARY_NAME}-${PLATFORM}.exe"
    else
        BINARY_FILE="${BINARY_NAME}-${PLATFORM}"
    fi
}

# Get latest release from GitHub
get_latest_release() {
    log_info "Fetching latest release..."

    # Try to get the latest release URL
    RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        RESPONSE=$(curl -s "$RELEASE_URL")
    elif command -v wget >/dev/null 2>&1; then
        RESPONSE=$(wget -qO- "$RELEASE_URL")
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    # Parse the version tag
    VERSION=$(echo "$RESPONSE" | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        log_error "Could not determine latest version. Please check your internet connection."
        exit 1
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_FILE}"
}

# Download and install binary
install_binary() {
    log_info "Downloading Walrus CLI ${VERSION} for ${PLATFORM}..."

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download binary
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "${TMP_DIR}/${BINARY_NAME}" "$DOWNLOAD_URL" || {
            log_error "Failed to download binary"
            exit 1
        }
    else
        wget -O "${TMP_DIR}/${BINARY_NAME}" "$DOWNLOAD_URL" || {
            log_error "Failed to download binary"
            exit 1
        }
    fi

    # Make binary executable
    chmod +x "${TMP_DIR}/${BINARY_NAME}"

    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"

    # Move binary to install directory
    mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

    log_success "Installed ${BINARY_NAME} to ${INSTALL_DIR}"
}

# Check if install directory is in PATH
check_path() {
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        log_warning "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "Add it to your PATH by adding this line to your shell profile:"
        echo ""

        # Detect shell and provide appropriate instruction
        SHELL_NAME=$(basename "$SHELL")
        case "$SHELL_NAME" in
            bash)
                echo "  echo 'export PATH=\"\$PATH:${INSTALL_DIR}\"' >> ~/.bashrc"
                echo "  source ~/.bashrc"
                ;;
            zsh)
                echo "  echo 'export PATH=\"\$PATH:${INSTALL_DIR}\"' >> ~/.zshrc"
                echo "  source ~/.zshrc"
                ;;
            fish)
                echo "  echo 'set -gx PATH \$PATH ${INSTALL_DIR}' >> ~/.config/fish/config.fish"
                echo "  source ~/.config/fish/config.fish"
                ;;
            *)
                echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
                ;;
        esac
        echo ""
    fi
}

# Main installation flow
main() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║     Walrus CLI Installation Script   ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════╝${NC}"
    echo ""

    # Check for local development install
    if [ "$1" = "--local" ] || [ "$1" = "-l" ]; then
        log_info "Installing from local build..."

        if [ ! -f "dist/${BINARY_NAME}" ]; then
            log_error "No local build found. Run 'make build' first."
            exit 1
        fi

        mkdir -p "$INSTALL_DIR"
        cp "dist/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

        log_success "Installed local build to ${INSTALL_DIR}"
    else
        # Normal installation from GitHub releases
        detect_platform
        get_latest_release
        install_binary
    fi

    # Check PATH
    check_path

    # Verify installation
    if [ -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        echo ""
        log_success "Installation complete!"
        echo ""

        # Show version
        "${INSTALL_DIR}/${BINARY_NAME}" version
        echo ""

        # Show next steps
        echo "Get started with:"
        echo "  ${BINARY_NAME} setup    # Configure Walrus"
        echo "  ${BINARY_NAME} upload   # Upload files"
        echo "  ${BINARY_NAME} list     # List stored files"
        echo "  ${BINARY_NAME} web      # Launch web interface"
        echo ""
    else
        log_error "Installation verification failed"
        exit 1
    fi
}

# Handle uninstall
if [ "$1" = "--uninstall" ] || [ "$1" = "-u" ]; then
    log_info "Uninstalling Walrus CLI..."

    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        rm "${INSTALL_DIR}/${BINARY_NAME}"
        log_success "Removed ${INSTALL_DIR}/${BINARY_NAME}"
    else
        log_warning "Walrus CLI not found at ${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # Check for config files
    CONFIG_DIR="$HOME/.walrus-rclone"
    if [ -d "$CONFIG_DIR" ]; then
        read -p "Remove configuration files at ${CONFIG_DIR}? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$CONFIG_DIR"
            log_success "Removed configuration files"
        fi
    fi

    exit 0
fi

# Run main installation
main "$@"