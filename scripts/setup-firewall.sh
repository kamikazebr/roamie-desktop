#!/bin/bash
# Simple firewall rules for Roamie VPN
# This runs on server startup

set -e

# Flush existing rules
iptables -F
iptables -X

# Default policies
iptables -P INPUT ACCEPT
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT

# Allow loopback
iptables -A INPUT -i lo -j ACCEPT

# Allow established connections
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Allow SSH (port 22)
iptables -A INPUT -p tcp --dport 22 -j ACCEPT

# Allow HTTP/HTTPS (80, 443)
iptables -A INPUT -p tcp --dport 80 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -j ACCEPT

# Allow API ports (8080-8100 range for auto-detection)
iptables -A INPUT -p tcp --dport 8080:8100 -j ACCEPT

# Allow WireGuard (51820)
iptables -A INPUT -p udp --dport 51820 -j ACCEPT

# Allow ping
iptables -A INPUT -p icmp -j ACCEPT

# Log and drop everything else (optional - commented for now)
# iptables -A INPUT -j LOG --log-prefix "iptables-dropped: "
# iptables -P INPUT DROP

echo "✓ Firewall rules configured"
iptables -L -n -v | head -20
