#!/bin/bash
#
# Installation script for Roamie Biometric Sudo
#
# This script installs the biometric authentication system for Linux sudo commands.
# It copies the Python script, sets up PAM configuration, and configures the JWT token.
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print colored message
print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_error "This script must be run as root"
    exit 1
fi

echo "========================================="
echo "Roamie Biometric Sudo Installer"
echo "========================================="
echo ""

# Check if script directory exists
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_SCRIPT="$SCRIPT_DIR/roamie_biometric_auth.py"
PAM_CONFIG="$SCRIPT_DIR/pam-config-sudo-biometric"

if [ ! -f "$PYTHON_SCRIPT" ]; then
    print_error "Python script not found: $PYTHON_SCRIPT"
    exit 1
fi

if [ ! -f "$PAM_CONFIG" ]; then
    print_error "PAM config not found: $PAM_CONFIG"
    exit 1
fi

# Install Python dependencies
print_status "Installing Python dependencies..."
if command -v apt-get &> /dev/null; then
    apt-get update -qq
    apt-get install -y python3 python3-pip python3-requests libpam-modules &> /dev/null
elif command -v dnf &> /dev/null; then
    dnf install -y python3 python3-pip python3-requests pam &> /dev/null
elif command -v yum &> /dev/null; then
    yum install -y python3 python3-pip python3-requests pam &> /dev/null
else
    print_warning "Could not detect package manager. Please install: python3, python3-pip, python3-requests, libpam-modules"
fi

pip3 install --quiet requests &> /dev/null || true

# Copy Python script
print_status "Installing Python script..."
cp "$PYTHON_SCRIPT" /usr/local/bin/roamie_biometric_auth.py
chmod 755 /usr/local/bin/roamie_biometric_auth.py

# Copy PAM configuration
print_status "Installing PAM configuration..."
cp "$PAM_CONFIG" /etc/pam.d/sudo-biometric
chmod 644 /etc/pam.d/sudo-biometric

# Configure JWT token
echo ""
echo "========================================="
echo "JWT Token Configuration"
echo "========================================="
echo ""
echo "You need to provide your Roamie VPN JWT token."
echo "You can get this token from the Culodi mobile app or by logging in via the API."
echo ""
read -p "Enter your JWT token: " JWT_TOKEN

if [ -z "$JWT_TOKEN" ]; then
    print_error "JWT token is required"
    exit 1
fi

# Save JWT token
print_status "Saving JWT token..."
echo "$JWT_TOKEN" > /root/.roamie_jwt
chmod 600 /root/.roamie_jwt

# Test the authentication
echo ""
echo "========================================="
echo "Testing Authentication"
echo "========================================="
echo ""
print_status "Testing biometric auth script..."

if /usr/local/bin/roamie_biometric_auth.py test &> /tmp/roamie_test.log; then
    TEST_RESULT=0
else
    TEST_RESULT=$?
fi

if [ $TEST_RESULT -eq 0 ]; then
    print_warning "Test completed (you may have approved on your phone)"
else
    print_warning "Test failed or was denied (this is normal if you didn't approve)"
fi

# Ask if user wants to enable biometric auth for sudo
echo ""
echo "========================================="
echo "Enable Biometric Auth for Sudo?"
echo "========================================="
echo ""
echo "Do you want to enable biometric authentication for sudo commands?"
echo ""
print_warning "WARNING: Make sure biometric auth is working before enabling!"
print_warning "WARNING: Keep a root terminal open for recovery!"
echo ""
read -p "Enable biometric auth for sudo? (y/N): " ENABLE_SUDO

if [ "$ENABLE_SUDO" = "y" ] || [ "$ENABLE_SUDO" = "Y" ]; then
    # Backup current sudo PAM config
    print_status "Backing up current sudo PAM config..."
    cp /etc/pam.d/sudo /etc/pam.d/sudo.backup.$(date +%Y%m%d%H%M%S)

    # Add biometric auth to sudo
    print_status "Enabling biometric auth for sudo..."

    # Check if already enabled
    if grep -q "sudo-biometric" /etc/pam.d/sudo; then
        print_warning "Biometric auth already enabled in /etc/pam.d/sudo"
    else
        # Add include at the top of the file
        sed -i '1i @include sudo-biometric' /etc/pam.d/sudo
        print_status "Biometric auth enabled for sudo"
    fi
else
    print_status "Skipped enabling biometric auth for sudo"
    echo ""
    echo "To enable manually, add this line to /etc/pam.d/sudo:"
    echo "  @include sudo-biometric"
fi

# Final instructions
echo ""
echo "========================================="
echo "Installation Complete!"
echo "========================================="
echo ""
print_status "Biometric authentication installed successfully"
echo ""
echo "Next steps:"
echo "  1. Open the Culodi mobile app"
echo "  2. Enable biometric authentication in settings"
echo "  3. Test with: sudo -v"
echo "  4. Your phone will receive an authentication request"
echo ""
print_warning "IMPORTANT: Keep a root terminal open until you confirm it's working!"
echo ""
echo "To disable biometric auth:"
echo "  sudo nano /etc/pam.d/sudo"
echo "  Remove or comment out the line: @include sudo-biometric"
echo ""
echo "To uninstall:"
echo "  sudo rm /usr/local/bin/roamie_biometric_auth.py"
echo "  sudo rm /etc/pam.d/sudo-biometric"
echo "  sudo rm /root/.roamie_jwt"
echo "  sudo nano /etc/pam.d/sudo  # Remove @include sudo-biometric"
echo ""
