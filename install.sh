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
REPO="justmert/walrus-cli"
BINARY_NAME="walrus-cli"
# Default to user's local bin directory (no sudo needed)
DEFAULT_INSTALL_DIR="$HOME/.local/bin"
if [ ! -d "$DEFAULT_INSTALL_DIR" ]; then
    DEFAULT_INSTALL_DIR="$HOME/bin"
fi
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

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
        x86_64|amd64) ARCH_TYPE="x86_64";;
        arm64|aarch64) ARCH_TYPE="arm64";;
        *) log_error "Unsupported architecture: $ARCH"; exit 1;;
    esac
}

# Construct archive filename based on VERSION and OS
construct_archive_name() {
    if [ "$OS_TYPE" = "darwin" ]; then
        # macOS uses universal binary
        ARCHIVE_FILE="${BINARY_NAME}_${VERSION#v}_Darwin_all.tar.gz"
    elif [ "$OS_TYPE" = "windows" ]; then
        ARCHIVE_FILE="${BINARY_NAME}_${VERSION#v}_Windows_${ARCH_TYPE}.zip"
    else
        # Linux
        OS_NAME="Linux"
        ARCHIVE_FILE="${BINARY_NAME}_${VERSION#v}_${OS_NAME}_${ARCH_TYPE}.tar.gz"
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

    # If no release found, try getting the latest tag
    if [ -z "$VERSION" ] || [ "$VERSION" = "null" ]; then
        log_info "No releases found. Trying to fetch latest tag..."

        TAGS_URL="https://api.github.com/repos/${REPO}/tags"
        if command -v curl >/dev/null 2>&1; then
            TAGS_RESPONSE=$(curl -s "$TAGS_URL")
        else
            TAGS_RESPONSE=$(wget -qO- "$TAGS_URL")
        fi

        VERSION=$(echo "$TAGS_RESPONSE" | grep '"name":' | head -1 | sed -E 's/.*"name": *"([^"]+)".*/\1/')

        if [ -z "$VERSION" ] || [ "$VERSION" = "null" ]; then
            log_error "Could not determine version. The release may still be building."
            log_info "Check https://github.com/${REPO}/releases for available releases."
            exit 1
        fi
    fi

    # Construct the archive filename after getting the version
    construct_archive_name

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_FILE}"
}

# Download and install binary
install_binary() {
    log_info "Downloading Walrus CLI ${VERSION}..."

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download archive
    ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE_FILE}"
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "${ARCHIVE_PATH}" "$DOWNLOAD_URL" || {
            log_error "Failed to download archive"
            exit 1
        }
    else
        wget -O "${ARCHIVE_PATH}" "$DOWNLOAD_URL" || {
            log_error "Failed to download archive"
            exit 1
        }
    fi

    # Extract archive
    log_info "Extracting archive..."
    if [[ "$ARCHIVE_FILE" == *.tar.gz ]]; then
        tar -xzf "${ARCHIVE_PATH}" -C "${TMP_DIR}" || {
            log_error "Failed to extract archive"
            exit 1
        }
    elif [[ "$ARCHIVE_FILE" == *.zip ]]; then
        unzip -q "${ARCHIVE_PATH}" -d "${TMP_DIR}" || {
            log_error "Failed to extract archive"
            exit 1
        }
    fi

    # Find the binary (it may have a different name in the archive)
    BINARY_PATH="${TMP_DIR}/${BINARY_NAME}"
    if [ ! -f "$BINARY_PATH" ]; then
        # Look for binary with version suffix
        VERSIONED_BINARY=$(find "$TMP_DIR" -name "${BINARY_NAME}*" -type f | head -1)
        if [ -n "$VERSIONED_BINARY" ]; then
            BINARY_PATH="$VERSIONED_BINARY"
        else
            log_error "Binary not found in archive"
            exit 1
        fi
    fi

    # Make binary executable
    chmod +x "${BINARY_PATH}"

    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        mkdir -p "$INSTALL_DIR" || {
            log_error "Failed to create directory $INSTALL_DIR"
            log_info "Try setting INSTALL_DIR to a writable location:"
            log_info "  curl -sSL ... | INSTALL_DIR=~/bin bash"
            exit 1
        }
    fi

    # Move binary to install directory
    mv "${BINARY_PATH}" "${INSTALL_DIR}/${BINARY_NAME}" || {
        log_error "Failed to install to $INSTALL_DIR"
        log_info "Try setting INSTALL_DIR to a writable location:"
        log_info "  curl -sSL ... | INSTALL_DIR=~/bin bash"
        exit 1
    }

    log_success "Installed ${BINARY_NAME} to ${INSTALL_DIR}"
}

# Check if install directory is in PATH
check_path() {
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        log_warning "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "To use walrus-cli from anywhere, add this to your PATH:"
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

        echo "For system-wide installation (optional):"
        echo "  sudo mv ${INSTALL_DIR}/${BINARY_NAME} /usr/local/bin/"
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