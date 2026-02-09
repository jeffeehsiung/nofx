#!/bin/bash

################################################################################
# NOFX+ Development Environment Setup Script
# 
# This script automates the complete setup process for NOFX+, including:
# - Dependency installation with fallback strategies
# - Environment configuration
# - Browser-guided API key setup
# - Project build and initialization
#
# Usage: ./setup.sh
################################################################################

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NOFX_REPO="$SCRIPT_DIR"
ENV_FILE="$NOFX_REPO/.env"
ENV_EXAMPLE="$NOFX_REPO/.env.example"

# Minimum versions required
MIN_GO_VERSION="1.21"
MIN_NODE_VERSION="18"

################################################################################
# Helper Functions
################################################################################

print_header() {
    echo -e "\n${BLUE}===================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===================================================${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Get OS type
get_os() {
    case "$OSTYPE" in
        darwin*) echo "macos" ;;
        linux*) echo "linux" ;;
        *) echo "unknown" ;;
    esac
}

# Compare versions
version_ge() {
    printf '%s\n%s' "$2" "$1" | sort -V -C
}

################################################################################
# Step 1: Check System Prerequisites
################################################################################

step_check_prerequisites() {
    print_header "Step 1: Checking System Prerequisites"
    
    OS=$(get_os)
    if [ "$OS" == "unknown" ]; then
        print_error "Unsupported operating system: $OSTYPE"
        exit 1
    fi
    
    print_info "Detected OS: $OS"
    
    # Check Go
    if command_exists go; then
        GO_VERSION=$(go version | sed 's/.*go\([0-9.]*\).*/\1/')
        if version_ge "$GO_VERSION" "$MIN_GO_VERSION"; then
            print_success "Go $GO_VERSION found"
        else
            print_error "Go version $GO_VERSION is less than required $MIN_GO_VERSION"
            exit 1
        fi
    else
        print_error "Go is not installed"
        print_info "Please install Go $MIN_GO_VERSION+ from https://golang.org/dl/"
        exit 1
    fi
    
    # Check Node.js
    if command_exists node; then
        NODE_VERSION=$(node -v | sed 's/v\([0-9.]*\).*/\1/')
        if version_ge "$NODE_VERSION" "$MIN_NODE_VERSION"; then
            print_success "Node.js $NODE_VERSION found"
        else
            print_error "Node.js version $NODE_VERSION is less than required $MIN_NODE_VERSION"
            exit 1
        fi
    else
        print_error "Node.js is not installed"
        print_info "Please install Node.js $MIN_NODE_VERSION+ from https://nodejs.org/"
        exit 1
    fi
}

################################################################################
# Step 2: Install TA-Lib with Fallback Strategies
################################################################################

install_talib_macos() {
    print_info "Attempting to install TA-Lib on macOS..."
    
    # Try Homebrew first
    if command_exists brew; then
        print_info "Trying Homebrew installation..."
        if brew install ta-lib 2>/dev/null; then
            print_success "TA-Lib installed via Homebrew"
            return 0
        fi
    fi
    
    # Try MacPorts
    if command_exists port; then
        print_info "Trying MacPorts installation..."
        if sudo port install ta-lib 2>/dev/null; then
            print_success "TA-Lib installed via MacPorts"
            return 0
        fi
    fi
    
    # Fallback: Manual installation from source
    print_warning "Standard installation methods failed. Installing from source..."
    
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT
    
    cd "$TEMP_DIR"
    
    # Download TA-Lib source
    print_info "Downloading TA-Lib source code..."
    curl -s -L "https://sourceforge.net/projects/ta-lib/files/ta-lib/0.4.0/ta-lib-0.4.0-src.tar.gz" \
        -o ta-lib-0.4.0-src.tar.gz 2>/dev/null || \
        curl -s -L "https://github.com/mrjbq7/ta-lib/releases/download/TA_Lib-0.4.0/ta-lib-0.4.0-src.tar.gz" \
        -o ta-lib-0.4.0-src.tar.gz 2>/dev/null
    
    if [ ! -f ta-lib-0.4.0-src.tar.gz ]; then
        print_error "Failed to download TA-Lib source"
        print_info "Manual installation required: https://mrjbq7.github.io/ta-lib/install.html"
        return 1
    fi
    
    tar -xzf ta-lib-0.4.0-src.tar.gz
    cd ta-lib
    
    print_info "Compiling and installing TA-Lib..."
    ./configure --prefix=/usr/local 2>/dev/null
    make 2>/dev/null
    sudo make install 2>/dev/null
    
    cd - >/dev/null
    print_success "TA-Lib installed from source"
    return 0
}

install_talib_linux() {
    print_info "Attempting to install TA-Lib on Linux..."
    
    # Detect package manager
    if command_exists apt-get; then
        print_info "Detected apt-based system (Ubuntu/Debian)"
        print_info "Installing libta-lib0-dev..."
        
        sudo apt-get update -qq 2>/dev/null || true
        if sudo apt-get install -y libta-lib0-dev 2>/dev/null; then
            print_success "TA-Lib installed via apt"
            return 0
        fi
    elif command_exists yum; then
        print_info "Detected yum-based system (CentOS/RHEL)"
        print_info "Installing ta-lib-devel..."
        
        if sudo yum install -y ta-lib-devel 2>/dev/null; then
            print_success "TA-Lib installed via yum"
            return 0
        fi
    elif command_exists pacman; then
        print_info "Detected pacman-based system (Arch Linux)"
        print_info "Installing ta-lib..."
        
        if sudo pacman -S --noconfirm ta-lib 2>/dev/null; then
            print_success "TA-Lib installed via pacman"
            return 0
        fi
    fi
    
    # Fallback: Manual installation from source
    print_warning "Standard installation methods failed. Installing from source..."
    
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT
    
    cd "$TEMP_DIR"
    
    print_info "Downloading TA-Lib source code..."
    curl -s -L "https://sourceforge.net/projects/ta-lib/files/ta-lib/0.4.0/ta-lib-0.4.0-src.tar.gz" \
        -o ta-lib-0.4.0-src.tar.gz 2>/dev/null || \
        curl -s -L "https://github.com/mrjbq7/ta-lib/releases/download/TA_Lib-0.4.0/ta-lib-0.4.0-src.tar.gz" \
        -o ta-lib-0.4.0-src.tar.gz 2>/dev/null
    
    if [ ! -f ta-lib-0.4.0-src.tar.gz ]; then
        print_error "Failed to download TA-Lib source"
        print_info "Manual installation required: https://mrjbq7.github.io/ta-lib/install.html"
        return 1
    fi
    
    tar -xzf ta-lib-0.4.0-src.tar.gz
    cd ta-lib
    
    print_info "Compiling and installing TA-Lib..."
    ./configure --prefix=/usr/local 2>/dev/null
    make 2>/dev/null
    sudo make install 2>/dev/null
    
    cd - >/dev/null
    print_success "TA-Lib installed from source"
    return 0
}

step_install_talib() {
    print_header "Step 2: Installing TA-Lib (Technical Indicator Library)"
    
    # Check if already installed
    if pkg-config --exists ta-lib 2>/dev/null; then
        print_success "TA-Lib is already installed"
        return 0
    fi
    
    OS=$(get_os)
    
    if [ "$OS" == "macos" ]; then
        if ! install_talib_macos; then
            print_error "Failed to install TA-Lib"
            print_info "Please try manual installation: https://mrjbq7.github.io/ta-lib/install.html"
            print_warning "Continuing setup (you may need to resolve this manually)"
        fi
    elif [ "$OS" == "linux" ]; then
        if ! install_talib_linux; then
            print_error "Failed to install TA-Lib"
            print_info "Please try manual installation: https://mrjbq7.github.io/ta-lib/install.html"
            print_warning "Continuing setup (you may need to resolve this manually)"
        fi
    else
        print_warning "Unsupported OS for automatic TA-Lib installation"
        print_info "Please install manually: https://mrjbq7.github.io/ta-lib/install.html"
    fi
}

################################################################################
# Step 3: Install Dependencies
################################################################################

step_install_dependencies() {
    print_header "Step 3: Installing Go and Node.js Dependencies"
    
    cd "$NOFX_REPO"
    
    # Run go mod tidy to ensure go.sum is created and dependencies are resolved
    print_info "Resolving Go module dependencies (go mod tidy)..."
    if ! go mod tidy; then
        print_error "go mod tidy failed"
        print_info "Troubleshooting: Check if go.mod is valid and has correct module name"
        return 1
    fi
    print_success "Go modules resolved"
    
    print_info "Downloading Go modules..."
    if ! go mod download; then
        print_warning "go mod download completed with warnings (this may be OK)"
    fi
    print_success "Go modules processed"
    
    print_info "Installing Node.js dependencies..."
    cd "$NOFX_REPO/web"
    
    # Clear npm cache to avoid stale package issues
    npm cache clean --force 2>/dev/null || true
    
    npm install
    if [ $? -ne 0 ]; then
        print_warning "npm install failed, retrying..."
        rm -rf node_modules package-lock.json
        npm install
    fi
    print_success "Node.js dependencies installed"
    
    cd "$NOFX_REPO"
}

################################################################################
# Step 4: Generate Encryption Keys
################################################################################

generate_encryption_keys() {
    # Generate JWT secret (32-byte Base64)
    JWT_SECRET=$(openssl rand -base64 32 2>/dev/null)
    
    # Generate AES-256 encryption key (32-byte Base64)
    DATA_ENCRYPTION_KEY=$(openssl rand -base64 32 2>/dev/null)
    
    # Generate RSA private key (2048-bit) in PKCS#1 format with escaped newlines for .env storage
    # Use genrsa -traditional to get "BEGIN RSA PRIVATE KEY" format instead of PKCS#8
    RSA_KEY_TEMP=$(openssl genrsa 2048 2>/dev/null)
    RSA_PRIVATE_KEY=$(echo "$RSA_KEY_TEMP" | sed 's/$/\\n/' | tr -d '\n' | sed 's/\\n$//')
    
    echo "$JWT_SECRET|$DATA_ENCRYPTION_KEY|$RSA_PRIVATE_KEY"
}

################################################################################
# Step 5: Setup Environment Configuration
################################################################################

step_setup_environment() {
    print_header "Step 5: Setting Up Environment Configuration"
    
    if [ -f "$ENV_FILE" ]; then
        print_warning ".env file already exists"
        read -p "Do you want to reconfigure it? (y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Keeping existing .env file"
            return 0
        fi
    fi
    
    # Copy template
    if [ -f "$ENV_EXAMPLE" ]; then
        cp "$ENV_EXAMPLE" "$ENV_FILE"
        print_success "Created .env from template"
    else
        print_error ".env.example not found"
        return 1
    fi
    
    # Generate encryption keys
    print_info "Generating encryption keys..."
    IFS='|' read -r JWT_SECRET DATA_ENCRYPTION_KEY RSA_PRIVATE_KEY <<< "$(generate_encryption_keys)"
    print_success "Generated secure encryption keys"
    
    # Update .env file with generated keys
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS sed syntax - need to use @ delimiter instead of | to handle backslashes in RSA key
        sed -i '' "s|your-jwt-secret-change-this-in-production|$JWT_SECRET|g" "$ENV_FILE"
        sed -i '' "s|your-base64-encoded-32-byte-key|$DATA_ENCRYPTION_KEY|g" "$ENV_FILE"
        # Use printf to properly escape the RSA key value for sed substitution
        RSA_KEY_ESCAPED=$(printf '%s\n' "$RSA_PRIVATE_KEY" | sed -e 's:[&/\]:\\&:g')
        sed -i '' "s|rsa-key-with-escaped-newlines|$RSA_KEY_ESCAPED|g" "$ENV_FILE"
    else
        # Linux sed syntax
        sed -i "s|your-jwt-secret-change-this-in-production|$JWT_SECRET|g" "$ENV_FILE"
        sed -i "s|your-base64-encoded-32-byte-key|$DATA_ENCRYPTION_KEY|g" "$ENV_FILE"
        # Use printf to properly escape the RSA key value for sed substitution
        RSA_KEY_ESCAPED=$(printf '%s\n' "$RSA_PRIVATE_KEY" | sed -e 's:[&/\]:\\&:g')
        sed -i "s|rsa-key-with-escaped-newlines|$RSA_KEY_ESCAPED|g" "$ENV_FILE"
    fi
    
    # Interactive API key configuration
    print_info ""
    print_info "Now let's configure your trading API keys"
    print_info "The setup will open browser windows to help you obtain the necessary keys"
    print_info ""
    
    # Always open API key pages to help user obtain keys
    print_info "Opening API key management pages in your browser..."
    print_info ""
    
    # Open DeepSeek
    if command_exists open; then
        open "https://platform.deepseek.com/api_keys" 2>/dev/null || true
    elif command_exists xdg-open; then
        xdg-open "https://platform.deepseek.com/api_keys" 2>/dev/null || true
    fi
    
    # Open Binance (main site for login/signup)
    if command_exists open; then
        open "https://www.binance.com/en" 2>/dev/null || true
    elif command_exists xdg-open; then
        xdg-open "https://www.binance.com/en" 2>/dev/null || true
    fi
    
    print_success "Browser pages opened for:"
    print_info "  1. DeepSeek API: https://platform.deepseek.com/api_keys"
    print_info "  2. Binance:"
    print_info "     - Sign up/Login: https://www.binance.com/en"
    print_info "     - API Management: Account (top right) → API Management"
    print_info "     - Demo Trading API: https://demo.binance.com/en/my/settings/api-management"
    print_info ""
    
    read -p "Do you want to enter API keys now? (y/n) " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "You can configure API keys later by editing: $ENV_FILE"
        print_info "Or run the API key configuration wizard with: ./setup.sh --api-keys"
        return 0
    fi
    
    # Guide through API key configuration
    configure_api_keys_interactive
}

configure_api_keys_interactive() {
    print_header "API Key Configuration Wizard"
    
    # LLM API Keys (Required)
    print_info "Setting up LLM API keys (Required for AI features)..."
    print_info ""
    
    # DeepSeek API Key
    print_info "Configuring DeepSeek API..."
    print_info "Opening DeepSeek API Management page in your browser..."
    
    if command_exists open; then
        open "https://platform.deepseek.com/api_keys" 2>/dev/null || true
    elif command_exists xdg-open; then
        xdg-open "https://platform.deepseek.com/api_keys" 2>/dev/null || true
    fi
    
    read -p "Enter your DeepSeek API Key: " DEEPSEEK_API_KEY
    
    # Update .env
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "/DEEPSEEK_API_KEY/s/=.*/=$DEEPSEEK_API_KEY/" "$ENV_FILE" || echo "DEEPSEEK_API_KEY=$DEEPSEEK_API_KEY" >> "$ENV_FILE"
    else
        sed -i "/DEEPSEEK_API_KEY/s/=.*/=$DEEPSEEK_API_KEY/" "$ENV_FILE" || echo "DEEPSEEK_API_KEY=$DEEPSEEK_API_KEY" >> "$ENV_FILE"
    fi
    
    print_success "DeepSeek API key configured"
    print_info ""
    
    # Trading Exchange API Keys
    print_info "Setting up Trading Exchange API keys..."
    print_info ""
    
    # Binance API Keys
    print_info "Setting up Binance API keys..."
    print_info ""
    print_info "To get your Binance API keys:"
    print_info "  1. Sign up/Login at: https://www.binance.com/en"
    print_info "  2. Click Account (top right) → API Management"
    print_info "  3. Create a new API key or use existing one"
    print_info "  4. Copy your API Key and Secret below"
    print_info ""
    print_info "For demo trading API keys, go to:"
    print_info "  https://demo.binance.com/en/my/settings/api-management"
    print_info ""
    
    if command_exists open; then
        open "https://www.binance.com/en" 2>/dev/null || true
    elif command_exists xdg-open; then
        xdg-open "https://www.binance.com/en" 2>/dev/null || true
    fi
    
    read -p "Enter your Binance API Key: " BINANCE_API_KEY
    read -sp "Enter your Binance API Secret: " BINANCE_API_SECRET
    echo
    
    # Update .env
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "/BINANCE_API_KEY/s/=.*/=$BINANCE_API_KEY/" "$ENV_FILE" || echo "BINANCE_API_KEY=$BINANCE_API_KEY" >> "$ENV_FILE"
        sed -i '' "/BINANCE_API_SECRET/s/=.*/=$BINANCE_API_SECRET/" "$ENV_FILE" || echo "BINANCE_API_SECRET=$BINANCE_API_SECRET" >> "$ENV_FILE"
    else
        sed -i "/BINANCE_API_KEY/s/=.*/=$BINANCE_API_KEY/" "$ENV_FILE" || echo "BINANCE_API_KEY=$BINANCE_API_KEY" >> "$ENV_FILE"
        sed -i "/BINANCE_API_SECRET/s/=.*/=$BINANCE_API_SECRET/" "$ENV_FILE" || echo "BINANCE_API_SECRET=$BINANCE_API_SECRET" >> "$ENV_FILE"
    fi
    
    print_success "Binance API keys configured"
    print_info ""
    
    # Optional: Other exchanges
    read -p "Do you want to configure additional exchange API keys? (y/n) " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_info "You can manually add other exchange keys to: $ENV_FILE"
    fi
}

################################################################################
# Step 6: Build Project
################################################################################

step_build_project() {
    print_header "Step 6: Building NOFX+ Project"
    
    cd "$NOFX_REPO"
    
    print_info "Cleaning previous builds..."
    make clean 2>/dev/null || true
    go clean --cache 2>/dev/null || true
    
    print_info "Building backend..."
    make build
    print_success "Backend built successfully"
    
    print_info "Building frontend..."
    make build-frontend
    print_success "Frontend built successfully"
}

################################################################################
# Step 7: Startup Verification
################################################################################

step_verify_installation() {
    print_header "Step 7: Verifying Installation"
    
    # Check binary
    if [ -f "$NOFX_REPO/nofxplus" ]; then
        print_success "Backend binary created"
    else
        print_error "Backend binary not found"
    fi
    
    # Check frontend build
    if [ -d "$NOFX_REPO/web/dist" ] || [ -d "$NOFX_REPO/web/build" ]; then
        print_success "Frontend build created"
    else
        print_warning "Frontend build directory not found"
    fi
    
    print_success "Installation verification complete!"
    
    # Run the comprehensive verification script
    print_info ""
    print_info "Running comprehensive environment verification..."
    print_info ""
    
    if [ -f "$NOFX_REPO/verify.sh" ]; then
        chmod +x "$NOFX_REPO/verify.sh"
        "$NOFX_REPO/verify.sh"
    else
        print_warning "verify.sh not found - skipping comprehensive verification"
    fi
}

################################################################################
# Step 8: Show Startup Instructions
################################################################################

step_show_instructions() {
    print_header "Setup Complete! 🎉"
    
    cat << 'EOF'

Your NOFX+ environment is now ready!

To start the application:

  1. Start the backend:
     make run
     OR
     ./nofxplus

  2. In another terminal, start the frontend:
     cd web
     npm start
     OR
     make run-frontend

  3. Open your browser and navigate to:
     http://localhost:3000

Environment Configuration:
  - Configuration file: .env
  - Edit this file to add/update your API keys
  - Backend runs on: http://localhost:8080 (default)
  - Frontend runs on: http://localhost:3000 (default)

Documentation:
  - Quick Start: docs/README.md
  - Architecture: docs/architecture/README.md
  - Configuration: docs/

Need Help?
  - Check README.md for more information
  - Review CONTRIBUTING.md for development guidelines
  - Create an issue on GitHub for support

Happy trading! 🚀

EOF

    print_info "You can now run 'make run' to start the backend"
    print_info "And 'make run-frontend' in another terminal to start the frontend"
}

################################################################################
# Main Execution Flow
################################################################################

main() {
    # Check for command line arguments
    if [ "$1" == "--api-keys" ] || [ "$1" == "-k" ]; then
        # API keys configuration mode - just configure keys, skip full setup
        print_header "🔑 API Key Configuration"
        
        # Check if .env exists
        if [ ! -f "$ENV_FILE" ]; then
            print_error ".env file not found"
            print_info "Please run ./setup.sh first to initialize the environment"
            exit 1
        fi
        
        print_info "Configure your trading and AI API keys"
        echo ""
        
        # Show the API key configuration
        configure_api_keys_interactive
        print_success "API key configuration completed!"
        return 0
    fi
    
    # Full setup flow (default)
    print_header "🚀 NOFX+ Development Environment Setup"
    
    print_info "Starting automated setup process..."
    print_info "This will install all dependencies and configure your environment"
    echo ""
    
    step_check_prerequisites
    step_install_talib
    step_install_dependencies
    step_setup_environment
    step_build_project
    step_verify_installation
    step_show_instructions
}

# Run main function
main "$@"

print_success "Setup script completed!"
