#!/bin/bash

################################################################################
# NOFX+ Environment Verification Script
# 
# This script checks if all dependencies and configuration are properly set up
# 
# Usage: ./verify.sh
################################################################################

# Don't exit on errors - we want to continue checking and report all issues
set +e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
GRAY='\033[0;37m'
NC='\033[0m'

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NOFX_REPO="$SCRIPT_DIR"
ENV_FILE="$NOFX_REPO/.env"

# Counters
PASSED=0
FAILED=0
WARNINGS=0

print_header() {
    echo -e "\n${BLUE}===================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===================================================${NC}\n"
}

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

check_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARNINGS++))
}

check_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

version_ge() {
    printf '%s\n%s' "$2" "$1" | sort -V -C
}

main() {
    print_header "🔍 NOFX+ Environment Verification"
    
    # System Check
    print_header "System Prerequisites"
    
    if command_exists go; then
        GO_VERSION=$(go version | sed -E 's/.*go([0-9.]+).*/\1/')
        if version_ge "$GO_VERSION" "1.21"; then
            check_pass "Go $GO_VERSION installed"
        else
            check_fail "Go version $GO_VERSION < 1.21 required"
        fi
    else
        check_fail "Go is not installed"
    fi
    
    if command_exists node; then
        NODE_VERSION=$(node -v | sed -E 's/v([0-9.]+).*/\1/')
        if version_ge "$NODE_VERSION" "18"; then
            check_pass "Node.js $NODE_VERSION installed"
        else
            check_fail "Node.js version $NODE_VERSION < 18 required"
        fi
    else
        check_fail "Node.js is not installed"
    fi
    
    if command_exists npm; then
        NPM_VERSION=$(npm -v)
        check_pass "npm $NPM_VERSION installed"
    else
        check_fail "npm is not installed"
    fi
    
    if command_exists git; then
        GIT_VERSION=$(git --version | sed -E 's/.*version ([0-9.]+).*/\1/')
        check_pass "Git $GIT_VERSION installed"
    else
        check_fail "Git is not installed"
    fi
    
    if command_exists openssl; then
        check_pass "openssl installed"
    else
        check_warning "openssl not found (needed for key generation)"
    fi
    
    if command_exists curl; then
        check_pass "curl installed"
    else
        check_warning "curl not found (needed for some operations)"
    fi
    
    # TA-Lib Check
    print_header "TA-Lib Library"
    
    if pkg-config --exists ta-lib 2>/dev/null; then
        TALIB_VERSION=$(pkg-config --modversion ta-lib 2>/dev/null || echo "unknown")
        check_pass "TA-Lib $TALIB_VERSION installed"
    else
        check_warning "TA-Lib not installed (optional but recommended)"
        check_info "Install with: brew install ta-lib (macOS) or apt-get install libta-lib0-dev (Linux)"
    fi
    
    # Project Structure
    print_header "Project Structure"
    
    if [ -f "$NOFX_REPO/main.go" ]; then
        check_pass "main.go found"
    else
        check_fail "main.go not found"
    fi
    
    if [ -f "$NOFX_REPO/go.mod" ]; then
        check_pass "go.mod found"
    else
        check_fail "go.mod not found"
    fi
    
    if [ -f "$NOFX_REPO/package.json" ]; then
        check_pass "package.json found"
    else
        check_fail "package.json not found"
    fi
    
    if [ -d "$NOFX_REPO/web" ]; then
        check_pass "web/ directory found"
    else
        check_fail "web/ directory not found"
    fi
    
    if [ -d "$NOFX_REPO/api" ]; then
        check_pass "api/ directory found"
    else
        check_fail "api/ directory not found"
    fi
    
    # Dependencies Check
    print_header "Dependencies"
    
    if [ -d "$NOFX_REPO/vendor" ] || go mod graph 2>/dev/null | grep -q .; then
        check_pass "Go modules appear to be set up"
    else
        check_warning "Go modules may not be downloaded (run: go mod download)"
    fi
    
    if [ -d "$NOFX_REPO/web/node_modules" ]; then
        check_pass "Node.js dependencies installed (node_modules found)"
    else
        check_warning "Node.js dependencies not installed (run: cd web && npm install)"
    fi
    
    # Configuration Check
    print_header "Configuration"
    
    if [ -f "$ENV_FILE" ]; then
        check_pass ".env file exists"
        
        # Check for required keys
        if grep -q "JWT_SECRET=" "$ENV_FILE"; then
            check_pass "JWT_SECRET configured"
        else
            check_fail "JWT_SECRET not set in .env"
        fi
        
        if grep -q "DATA_ENCRYPTION_KEY=" "$ENV_FILE"; then
            check_pass "DATA_ENCRYPTION_KEY configured"
        else
            check_fail "DATA_ENCRYPTION_KEY not set in .env"
        fi
        
        if grep -q "RSA_PRIVATE_KEY=" "$ENV_FILE"; then
            check_pass "RSA_PRIVATE_KEY configured"
        else
            check_fail "RSA_PRIVATE_KEY not set in .env"
        fi
        
        # Check for API keys
        if grep -q "BINANCE_API_KEY=" "$ENV_FILE"; then
            if grep "BINANCE_API_KEY=your" "$ENV_FILE" >/dev/null; then
                check_warning "BINANCE_API_KEY not configured (still has template value)"
            else
                check_pass "BINANCE_API_KEY configured"
            fi
        else
            check_warning "BINANCE_API_KEY not set in .env"
        fi
    else
        check_fail ".env file not found"
        check_info "Create one with: cp .env.example .env"
    fi
    
    if [ -f "$NOFX_REPO/.env.example" ]; then
        check_pass ".env.example template found"
    else
        check_fail ".env.example template not found"
    fi
    
    # Build Artifacts Check
    print_header "Build Artifacts"
    
    if [ -f "$NOFX_REPO/nofxplus" ]; then
        check_pass "Backend binary found"
        if [ -x "$NOFX_REPO/nofxplus" ]; then
            check_pass "Backend binary is executable"
        else
            check_warning "Backend binary exists but is not executable"
        fi
    else
        check_warning "Backend binary not found (run: make build)"
    fi
    
    if [ -d "$NOFX_REPO/web/dist" ] || [ -d "$NOFX_REPO/web/build" ]; then
        check_pass "Frontend build directory found"
    else
        check_warning "Frontend build not found (run: make build-frontend)"
    fi
    
    # Makefile Check
    print_header "Build Configuration"
    
    if [ -f "$NOFX_REPO/Makefile" ]; then
        check_pass "Makefile found"
        
        if grep -q "build:" "$NOFX_REPO/Makefile"; then
            check_pass "Build target available"
        else
            check_warning "Build target not found in Makefile"
        fi
        
        if grep -q "run:" "$NOFX_REPO/Makefile"; then
            check_pass "Run target available"
        else
            check_warning "Run target not found in Makefile"
        fi
    else
        check_fail "Makefile not found"
    fi
    
    # Port Availability Check
    print_header "Port Availability"
    
    if command_exists lsof; then
        if ! lsof -i :8080 >/dev/null 2>&1; then
            check_pass "Port 8080 (backend) is available"
        else
            check_warning "Port 8080 (backend) is in use"
        fi
        
        if ! lsof -i :3000 >/dev/null 2>&1; then
            check_pass "Port 3000 (frontend) is available"
        else
            check_warning "Port 3000 (frontend) is in use"
        fi
    else
        check_info "Unable to check port availability (lsof not found)"
    fi
    
    # Summary
    print_header "Summary"
    
    echo -e "${GREEN}Passed: $PASSED${NC}"
    if [ $WARNINGS -gt 0 ]; then
        echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
    fi
    if [ $FAILED -gt 0 ]; then
        echo -e "${RED}Failed: $FAILED${NC}"
    fi
    
    if [ $FAILED -eq 0 ]; then
        echo ""
        echo -e "${GREEN}✓ All critical checks passed!${NC}"
        
        if [ $WARNINGS -gt 0 ]; then
            echo -e "${YELLOW}⚠ Some optional components are missing${NC}"
            echo "  You can still run the application, but some features may not work"
        fi
        
        echo ""
        check_info "To start the application:"
        echo "  1. Run: make run (backend)"
        echo "  2. In another terminal: make run-frontend (frontend)"
        echo "  3. Open: http://localhost:3000"
    else
        echo ""
        echo -e "${RED}✗ Some critical checks failed${NC}"
        echo "  Please fix the issues above before running the application"
        echo ""
        echo -e "${BLUE}Run ./setup.sh to fix common issues automatically${NC}"
    fi
    
    echo ""
}

# Run main function
main "$@"
