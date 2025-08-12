#!/bin/bash

# Station Installation Script
# Usage: curl -sSL https://getstation.cloudshipai.com | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
REPO="cloudshipai/station"
BINARY_NAME="stn"
INSTALL_DIR="$HOME/.local/bin"
VERSION="latest"

# Banner
print_banner() {
    echo -e "${BLUE}"
    echo "🚂 Station Installation Script"
    echo "================================="
    echo -e "${NC}"
    echo "Lightweight Runtime for Deployable Sub-Agents"
    echo ""
}

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

# Platform detection
detect_platform() {
    local os arch
    
    # Detect OS
    case "$(uname -s)" in
        Darwin)
            os="darwin"
            ;;
        Linux)
            os="linux"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            os="windows"
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        i386|i686)
            arch="386"
            ;;
        armv7*)
            arch="arm"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            ;;
    esac
    
    echo "${os}_${arch}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Download file with fallback methods
download_file() {
    local url="$1"
    local output="$2"
    
    if command_exists curl; then
        log_info "Downloading with curl..."
        curl -fsSL -o "$output" "$url"
    elif command_exists wget; then
        log_info "Downloading with wget..."
        wget -q -O "$output" "$url"
    else
        log_error "Neither curl nor wget is available. Please install one of them."
    fi
}

# Get latest release version from GitHub API
get_latest_version() {
    local api_url="https://api.github.com/repos/$REPO/releases/latest"
    local version
    
    if command_exists curl; then
        version=$(curl -fsSL "$api_url" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    elif command_exists wget; then
        version=$(wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    else
        log_error "Cannot fetch latest version. Please install curl or wget."
    fi
    
    if [ -z "$version" ]; then
        log_warning "Could not determine latest version, using fallback"
        version="v0.1.0"  # Fallback version
    fi
    
    echo "$version"
}

# Check if running as root or with sudo
check_permissions() {
    if [ "$EUID" -eq 0 ]; then
        return 0  # Running as root
    elif command_exists sudo && sudo -n true 2>/dev/null; then
        return 0  # Can sudo without password
    else
        return 1  # Need to prompt for sudo
    fi
}

# Install binary
install_binary() {
    local platform="$1"
    local version="$2"
    local temp_dir
    
    temp_dir=$(mktemp -d)
    cd "$temp_dir"
    
    # Construct download URL (matching GoReleaser format)
    local os_arch="${platform/_/-}"
    local archive_name="stn-${os_arch}.tar.gz"
    
    # Handle Windows zip format
    if [[ "$platform" == "windows"* ]]; then
        archive_name="stn-${os_arch}.zip"
    fi
    
    local download_url="https://github.com/$REPO/releases/download/$version/$archive_name"
    
    log_info "Downloading Station $version for $platform..."
    log_info "URL: $download_url"
    
    # Download the archive
    if ! download_file "$download_url" "$archive_name"; then
        log_error "Failed to download Station. Please check your internet connection and try again."
    fi
    
    # Extract the archive
    log_info "Extracting archive..."
    if [[ "$archive_name" == *.tar.gz ]]; then
        tar -xzf "$archive_name"
    elif [[ "$archive_name" == *.zip ]]; then
        if command_exists unzip; then
            unzip -q "$archive_name"
        else
            log_error "unzip is required to extract Windows binaries. Please install unzip."
        fi
    fi
    
    # Find the binary
    local binary_path
    if [[ "$platform" == "windows"* ]]; then
        binary_path="$BINARY_NAME.exe"
    else
        binary_path="$BINARY_NAME"
    fi
    
    if [ ! -f "$binary_path" ]; then
        log_error "Binary not found in archive. Expected: $binary_path"
    fi
    
    # Make binary executable
    chmod +x "$binary_path"
    
    # Install binary
    log_info "Installing Station to $INSTALL_DIR..."
    
    if check_permissions; then
        if [ "$EUID" -eq 0 ]; then
            cp "$binary_path" "$INSTALL_DIR/$BINARY_NAME"
        else
            sudo cp "$binary_path" "$INSTALL_DIR/$BINARY_NAME"
        fi
    else
        log_warning "Insufficient permissions to install to $INSTALL_DIR"
        local user_bin="$HOME/.local/bin"
        mkdir -p "$user_bin"
        cp "$binary_path" "$user_bin/$BINARY_NAME"
        INSTALL_DIR="$user_bin"
        log_info "Installed to $user_bin instead"
        
        # Add to PATH if not already there
        if [[ ":$PATH:" != *":$user_bin:"* ]]; then
            echo ""
            log_warning "Add $user_bin to your PATH:"
            echo "  echo 'export PATH=\"$user_bin:\$PATH\"' >> ~/.bashrc"
            echo "  source ~/.bashrc"
        fi
    fi
    
    # Cleanup
    cd /
    rm -rf "$temp_dir"
    
    log_success "Station installed successfully!"
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    if command_exists "$BINARY_NAME"; then
        local version_output
        version_output=$($BINARY_NAME --version 2>&1 || echo "unknown")
        log_success "Station is installed and working!"
        echo "  Version: $version_output"
        echo "  Location: $(which $BINARY_NAME)"
    else
        log_error "Installation verification failed. Station command not found in PATH."
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    echo -e "${PURPLE}🎉 Installation Complete!${NC}"
    echo ""
    echo -e "${CYAN}Next Steps:${NC}"
    echo "1. Initialize Station:"
    echo "   ${BINARY_NAME} init"
    echo ""
    echo "2. Create your first agent:"
    echo "   ${BINARY_NAME} agent create --name \"My Agent\" --description \"My first agent\""
    echo ""
    echo "3. Browse available bundles:"
    echo "   https://cloudshipai.github.io/registry"
    echo ""
    echo "4. Install a bundle:"
    echo "   ${BINARY_NAME} template install <bundle-url>"
    echo ""
    echo -e "${CYAN}Documentation:${NC}"
    echo "• Quick Start: https://cloudshipai.github.io/station/quickstart/"
    echo "• Documentation: https://cloudshipai.github.io/station/"
    echo "• GitHub: https://github.com/$REPO"
    echo ""
    echo -e "${GREEN}Happy automating! 🚂${NC}"
}

# Main installation flow
main() {
    print_banner
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                VERSION="$2"
                shift 2
                ;;
            --install-dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --help)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --version VERSION    Install specific version (default: latest)"
                echo "  --install-dir DIR    Install directory (default: /usr/local/bin)"
                echo "  --help               Show this help message"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                ;;
        esac
    done
    
    # Check prerequisites
    log_info "Checking prerequisites..."
    
    if ! command_exists tar; then
        log_error "tar is required but not installed."
    fi
    
    # Detect platform
    log_info "Detecting platform..."
    local platform
    platform=$(detect_platform)
    log_success "Detected platform: $platform"
    
    # Get version
    if [ "$VERSION" = "latest" ]; then
        log_info "Fetching latest version..."
        VERSION=$(get_latest_version)
    fi
    log_success "Target version: $VERSION"
    
    # Install
    install_binary "$platform" "$VERSION"
    
    # Verify
    verify_installation
    
    # Print next steps
    print_next_steps
}

# Handle Ctrl+C
trap 'echo -e "\n${RED}Installation interrupted by user.${NC}"; exit 1' INT

# Run main function
main "$@"