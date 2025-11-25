#!/bin/bash

set -e

echo "=== Roamie VPN - Restore WireGuard Configuration ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (sudo)"
  exit 1
fi

BACKUP_DIR="/etc/wireguard/backups"
WG_INTERFACE=${WG_INTERFACE:-wg0}

# Check if backup directory exists
if [ ! -d "$BACKUP_DIR" ]; then
    echo "❌ No backups found in $BACKUP_DIR"
    exit 1
fi

# List available backups
echo "Available backups:"
echo ""
ls -1dt "$BACKUP_DIR"/*/ 2>/dev/null | nl -w2 -s'. ' || {
    echo "❌ No backup directories found"
    exit 1
}

echo ""
read -p "Enter backup number to restore (or 'q' to quit): " choice

if [ "$choice" = "q" ]; then
    echo "Restore cancelled"
    exit 0
fi

# Get the selected backup directory
BACKUP_PATH=$(ls -1dt "$BACKUP_DIR"/*/ 2>/dev/null | sed -n "${choice}p")

if [ -z "$BACKUP_PATH" ]; then
    echo "❌ Invalid selection"
    exit 1
fi

echo ""
echo "Selected backup: $BACKUP_PATH"
echo "Contents:"
ls -lh "$BACKUP_PATH"
echo ""

read -p "Restore this backup? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

# Stop current interface if running
if ip link show $WG_INTERFACE &>/dev/null; then
    echo "Stopping current $WG_INTERFACE interface..."
    wg-quick down $WG_INTERFACE || true
fi

# Restore config file
if [ -f "$BACKUP_PATH/$WG_INTERFACE.conf" ]; then
    cp "$BACKUP_PATH/$WG_INTERFACE.conf" "/etc/wireguard/$WG_INTERFACE.conf"
    chmod 600 "/etc/wireguard/$WG_INTERFACE.conf"
    echo "✓ Restored: $WG_INTERFACE.conf"
fi

# Restore keys
for backup_file in "$BACKUP_PATH"/*.key; do
    if [ -f "$backup_file" ]; then
        filename=$(basename "$backup_file")
        cp "$backup_file" "/etc/wireguard/$filename"
        chmod 600 "/etc/wireguard/$filename"
        echo "✓ Restored: $filename"
    fi
done

echo ""
read -p "Start the restored interface now? (yes/no): " start_now
if [ "$start_now" = "yes" ]; then
    wg-quick up $WG_INTERFACE
    echo "✓ Interface started"
fi

echo ""
echo "=== Restore Complete ==="
echo "Your previous WireGuard configuration has been restored"
echo ""
