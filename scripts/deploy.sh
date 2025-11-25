#!/bin/bash

# Deployment script for Roamie VPN
# Usage: ./scripts/deploy.sh [server-address]

set -e

SERVER="${1:-root@178.156.133.88}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Roamie VPN Deployment ==="
echo "Target: $SERVER"
echo ""

# Check if roamie-server binary exists
if [ ! -f "$PROJECT_DIR/roamie-server" ]; then
    echo "Error: roamie-server binary not found. Run ./scripts/build.sh first"
    exit 1
fi

# Check if .env.production exists
if [ ! -f "$PROJECT_DIR/.env.production" ]; then
    echo "Error: .env.production not found"
    exit 1
fi

echo "Step 1: Stopping service on remote server..."
ssh "$SERVER" << 'ENDSSH'
# Stop service if running
systemctl stop roamie 2>/dev/null || true
sleep 1

# Kill any remaining roamie-server processes
pkill -9 roamie-server 2>/dev/null || true
sleep 1

# Remove old binary to prevent file locking issues
rm -f /root/roamie-server

echo "✓ Service stopped and old binary removed"
ENDSSH

echo "Step 2: Copying files to server..."
scp "$PROJECT_DIR/roamie-server" "$SERVER:/root/" || {
    echo "Error: Failed to copy roamie-server. Check SSH connection."
    exit 1
}
scp "$PROJECT_DIR/.env.production" "$SERVER:/root/.env"
scp "$PROJECT_DIR/roamie.service" "$SERVER:/tmp/"

echo ""
echo "Step 2.1: Fixing DATABASE_URL if needed..."
ssh "$SERVER" << 'ENDSSH'
# Fix DATABASE_URL to use correct port (5433 instead of 5432)
if grep -q "localhost:5432/" /root/.env 2>/dev/null; then
    sed -i 's|localhost:5432/|localhost:5433/|g' /root/.env
    echo "✓ DATABASE_URL fixed to use port 5433"
else
    echo "✓ DATABASE_URL already correct"
fi
ENDSSH

echo ""
echo "Step 2.5: Deploying Firebase credentials (if configured)..."

# Check if Firebase is configured in .env.production
if grep -q "FIREBASE_CREDENTIALS_PATH" "$PROJECT_DIR/.env.production"; then
    FIREBASE_PATH=$(grep "FIREBASE_CREDENTIALS_PATH" "$PROJECT_DIR/.env.production" | cut -d'=' -f2)
    echo "  ✓ FIREBASE_CREDENTIALS_PATH configured in .env.production"
    echo "    Path: $FIREBASE_PATH"
else
    echo "  ℹ️  FIREBASE_CREDENTIALS_PATH not set in .env.production"
    echo "    Firebase authentication will not be available"
fi

# Deploy Firebase credentials if exists locally
if [ -f "$PROJECT_DIR/config/firebase-service-account.json" ]; then
    echo ""
    echo "  → Found Firebase credentials locally"

    # Create config directory on server
    ssh "$SERVER" "mkdir -p /root/config" || true

    # Check if file already exists on server
    if ssh "$SERVER" "[ -f /root/config/firebase-service-account.json ]" 2>/dev/null; then
        echo "  ⚠️  Firebase credentials already exist on server"

        # Check if running in interactive mode
        if [ -t 0 ]; then
            read -p "  Overwrite? [y/N]: " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                scp "$PROJECT_DIR/config/firebase-service-account.json" "$SERVER:/root/config/" || true
                ssh "$SERVER" "chmod 600 /root/config/firebase-service-account.json" || true
                echo "  ✓ Firebase credentials updated"
            else
                echo "  ✓ Keeping existing Firebase credentials"
            fi
        else
            # Non-interactive mode: skip overwrite
            echo "  ✓ Keeping existing Firebase credentials (non-interactive mode)"
        fi
    else
        scp "$PROJECT_DIR/config/firebase-service-account.json" "$SERVER:/root/config/" || true
        ssh "$SERVER" "chmod 600 /root/config/firebase-service-account.json" || true
        echo "  ✓ Firebase credentials deployed"
    fi
else
    echo ""
    echo "  ℹ️  No Firebase credentials found locally"
    echo "    (looking for: $PROJECT_DIR/config/firebase-service-account.json)"
    echo "    Firebase authentication will not be available on server"
fi

echo ""
echo "Step 3: Installing systemd service..."
ssh "$SERVER" << 'ENDSSH'
set -e

# Make binary executable
chmod +x /root/roamie-server

# Install systemd service
echo "Installing systemd service..."
mv /tmp/roamie.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable roamie.service
systemctl start roamie.service

echo ""
echo "✓ Deployment complete!"
echo ""
echo "Service status:"
systemctl status roamie.service --no-pager || true

echo ""
echo "--- Firebase Configuration Check ---"
if [ -f "/root/config/firebase-service-account.json" ]; then
    echo "✓ Firebase credentials present on server"
    echo "  Path: /root/config/firebase-service-account.json"
    PERMS=$(stat -c "%a" /root/config/firebase-service-account.json)
    if [ "$PERMS" = "600" ]; then
        echo "  ✓ Permissions correct (600)"
    else
        echo "  ⚠️  Permissions: $PERMS (should be 600)"
        chmod 600 /root/config/firebase-service-account.json
        echo "  ✓ Permissions fixed to 600"
    fi
else
    echo "⚠️  Firebase credentials NOT found on server"
    echo "  Firebase authentication will not work"
    echo "  To enable: copy config/firebase-service-account.json manually"
fi

echo ""
echo "To view logs: journalctl -u roamie -f"
ENDSSH

echo ""
echo "=== Deployment Successful ==="
echo ""

# Wait for server to start
echo "Waiting for server to start..."
sleep 3

# Health check
echo -n "Checking server health... "
if curl -s -f http://178.156.133.88:8081/health > /dev/null; then
    echo "✓ Server is healthy"
else
    echo "✗ Health check failed"
    echo "Check logs: ssh $SERVER 'journalctl -u roamie -f'"
    exit 1
fi

echo ""
echo "Useful commands:"
echo "  ssh $SERVER 'systemctl status roamie'     # Check status"
echo "  ssh $SERVER 'journalctl -u roamie -f'     # Follow logs"
echo "  ssh $SERVER 'systemctl restart roamie'    # Restart service"
echo "  ssh $SERVER 'systemctl stop roamie'       # Stop service"
