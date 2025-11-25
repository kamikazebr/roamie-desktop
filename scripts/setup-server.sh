#!/bin/bash

set -e

echo "=== Roamie VPN Server Setup ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (sudo)"
  exit 1
fi

# Install WireGuard
echo "Installing WireGuard..."
apt-get update
apt-get install -y wireguard wireguard-tools

# Enable IP forwarding
echo "Enabling IP forwarding..."
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
echo "net.ipv6.conf.all.forwarding=1" >> /etc/sysctl.conf
sysctl -p

# Create WireGuard directory
mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

# Backup existing WireGuard configuration
WG_INTERFACE=${WG_INTERFACE:-wg0}
BACKUP_DIR="/etc/wireguard/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

if [ -f "/etc/wireguard/$WG_INTERFACE.conf" ] || ip link show $WG_INTERFACE &>/dev/null; then
    echo "⚠️  Existing WireGuard configuration detected!"
    echo ""

    # Check if interface is running
    if ip link show $WG_INTERFACE &>/dev/null; then
        echo "Interface $WG_INTERFACE is currently active"
        wg show $WG_INTERFACE 2>/dev/null || true
        echo ""
        read -p "Stop interface and continue? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "Setup cancelled by user"
            exit 1
        fi
        echo "Stopping $WG_INTERFACE..."
        wg-quick down $WG_INTERFACE || true
    fi

    # Create backup directory
    mkdir -p "$BACKUP_DIR"
    echo "Creating backup in $BACKUP_DIR/$WG_INTERFACE-$TIMESTAMP/"
    mkdir -p "$BACKUP_DIR/$WG_INTERFACE-$TIMESTAMP"

    # Backup config file
    if [ -f "/etc/wireguard/$WG_INTERFACE.conf" ]; then
        cp "/etc/wireguard/$WG_INTERFACE.conf" "$BACKUP_DIR/$WG_INTERFACE-$TIMESTAMP/$WG_INTERFACE.conf"
        echo "✓ Backed up: $WG_INTERFACE.conf"
    fi

    # Backup any keys
    for key_file in /etc/wireguard/*.key; do
        if [ -f "$key_file" ]; then
            cp "$key_file" "$BACKUP_DIR/$WG_INTERFACE-$TIMESTAMP/"
            echo "✓ Backed up: $(basename $key_file)"
        fi
    done

    echo ""
    echo "Backup complete! Original config saved to:"
    echo "  $BACKUP_DIR/$WG_INTERFACE-$TIMESTAMP/"
    echo ""
fi

# Generate server keys if not exist
if [ ! -f /etc/wireguard/server_private.key ]; then
    echo "Generating server keys..."
    wg genkey | tee /etc/wireguard/server_private.key | wg pubkey > /etc/wireguard/server_public.key
    chmod 600 /etc/wireguard/server_private.key
    chmod 644 /etc/wireguard/server_public.key
    echo "Server public key: $(cat /etc/wireguard/server_public.key)"
fi

# Get server IP
SERVER_IP=$(curl -s ifconfig.me)
echo "Detected server IP: $SERVER_IP"

# Create WireGuard interface config
WG_PORT=${WG_PORT:-51820}
SERVER_PRIVATE_KEY=$(cat /etc/wireguard/server_private.key)

cat > /etc/wireguard/$WG_INTERFACE.conf <<EOF
[Interface]
PrivateKey = $SERVER_PRIVATE_KEY
Address = 10.100.0.1/16
ListenPort = $WG_PORT
PostUp = iptables -A FORWARD -i $WG_INTERFACE -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i $WG_INTERFACE -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

# Peers will be added dynamically by the application
EOF

chmod 600 /etc/wireguard/$WG_INTERFACE.conf

# Start WireGuard
echo "Starting WireGuard interface..."
wg-quick up $WG_INTERFACE

# Enable on boot
systemctl enable wg-quick@$WG_INTERFACE

# Configure firewall
echo "Configuring firewall..."
ufw allow $WG_PORT/udp
ufw allow 8080/tcp  # API port
ufw --force enable

echo ""
echo "=== Setup Complete ==="
echo "WireGuard interface: $WG_INTERFACE"
echo "Listen port: $WG_PORT"
echo "Server public key: $(cat /etc/wireguard/server_public.key)"
echo "Server endpoint: $SERVER_IP:$WG_PORT"
echo ""
echo "Next steps:"
echo "1. Configure PostgreSQL database"
echo "2. Set environment variables in .env file"
echo "3. Run database migrations"
echo "4. Start the server: ./roamie-server"
