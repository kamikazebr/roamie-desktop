#!/bin/bash

# Helper functions for Docker VPN integration testing

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

#===============================================================================
# WireGuard Setup Functions
#===============================================================================

setup_wireguard_in_container() {
  local container=$1
  local wg_config=$2

  echo "  → Setting up WireGuard in $container..."

  # Write config to container
  echo "$wg_config" | docker exec -i "$container" sh -c "cat > /etc/wireguard/wg0.conf"

  # Set permissions
  docker exec "$container" chmod 600 /etc/wireguard/wg0.conf

  # Bring up WireGuard interface
  docker exec "$container" wg-quick up wg0 2>&1 || {
    echo -e "${RED}  ✗ Failed to bring up WireGuard${NC}"
    return 1
  }

  echo -e "${GREEN}  ✓ WireGuard interface up${NC}"
  return 0
}

get_wireguard_stats() {
  local container=$1
  docker exec "$container" wg show wg0 2>/dev/null || echo "No stats available"
}

stop_wireguard_in_container() {
  local container=$1
  docker exec "$container" wg-quick down wg0 2>/dev/null || true
}

#===============================================================================
# Connectivity Testing Functions
#===============================================================================

test_ping() {
  local from_container=$1
  local to_ip=$2
  local should_succeed=$3
  local description=$4

  echo -n "  → Testing: $description... "

  # Try to ping (timeout 2 seconds, 3 packets)
  if docker exec "$from_container" ping -c 3 -W 2 "$to_ip" >/dev/null 2>&1; then
    if [ "$should_succeed" == "true" ]; then
      echo -e "${GREEN}✓ Success (as expected)${NC}"
      return 0
    else
      echo -e "${RED}✗ FAILED - ping succeeded but should have failed!${NC}"
      return 1
    fi
  else
    if [ "$should_succeed" == "false" ]; then
      echo -e "${GREEN}✓ Blocked (as expected)${NC}"
      return 0
    else
      echo -e "${RED}✗ FAILED - ping failed but should have succeeded!${NC}"
      return 1
    fi
  fi
}

test_wireguard_encrypted() {
  local container=$1

  echo "  → Checking WireGuard encryption..."

  # Check if handshake occurred
  local handshake=$(docker exec "$container" wg show wg0 latest-handshakes 2>/dev/null | awk '{print $2}')

  if [ -z "$handshake" ] || [ "$handshake" == "0" ]; then
    echo -e "${RED}  ✗ No handshake - not encrypted!${NC}"
    return 1
  fi

  # Check if data is being transferred
  local rx_bytes=$(docker exec "$container" wg show wg0 transfer 2>/dev/null | awk '{print $2}')

  if [ -z "$rx_bytes" ] || [ "$rx_bytes" == "0" ]; then
    echo -e "${YELLOW}  ⚠ No data transferred yet${NC}"
  else
    echo -e "${GREEN}  ✓ Encrypted tunnel active (received: $rx_bytes bytes)${NC}"
  fi

  return 0
}

#===============================================================================
# API Helper Functions
#===============================================================================

login_account() {
  local email=$1
  local server_url=$2

  # Request auth code
  curl -s -X POST "$server_url/api/auth/request-code" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\"}" >/dev/null

  sleep 1

  # Get code from database
  local code=$(docker exec roamie-test-db psql -U roamie -d roamie_vpn -t -c \
    "SELECT code FROM auth_codes WHERE email = '$email' ORDER BY created_at DESC LIMIT 1;" | tr -d ' ')

  # Verify code and get JWT
  local response=$(curl -s -X POST "$server_url/api/auth/verify-code" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"code\":\"$code\"}")

  echo "$response" | jq -r '.token'
}

register_device() {
  local jwt=$1
  local device_name=$2
  local server_url=$3

  # Generate WireGuard keypair in temp container
  local private_key=$(docker exec alice-device-1 sh -c "wg genkey")
  local public_key=$(echo "$private_key" | docker exec -i alice-device-1 sh -c "wg pubkey")

  # Register device
  local response=$(curl -s -X POST "$server_url/api/devices" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $jwt" \
    -d "{\"device_name\":\"$device_name\",\"public_key\":\"$public_key\"}")

  # Return: private_key|device_id|vpn_ip|subnet|server_key|server_endpoint
  local device_id=$(echo "$response" | jq -r '.device_id')
  local vpn_ip=$(echo "$response" | jq -r '.vpn_ip')
  local subnet=$(echo "$response" | jq -r '.user_subnet')
  local server_key=$(echo "$response" | jq -r '.server_public_key')
  local server_endpoint=$(echo "$response" | jq -r '.server_endpoint')

  echo "$private_key|$device_id|$vpn_ip|$subnet|$server_key|$server_endpoint"
}

generate_wireguard_config() {
  local private_key=$1
  local vpn_ip=$2
  local allowed_ips=$3
  local server_key=$4
  local server_endpoint=$5

  cat <<EOF
[Interface]
PrivateKey = $private_key
Address = $vpn_ip/32
DNS = 1.1.1.1

[Peer]
PublicKey = $server_key
Endpoint = $server_endpoint
AllowedIPs = $allowed_ips
PersistentKeepalive = 25
EOF
}

#===============================================================================
# Utility Functions
#===============================================================================

wait_for_vpn_connection() {
  local container=$1
  local max_wait=$2
  local elapsed=0

  echo -n "  → Waiting for VPN handshake in $container"

  while [ $elapsed -lt $max_wait ]; do
    local handshake=$(docker exec "$container" wg show wg0 latest-handshakes 2>/dev/null | awk '{print $2}')
    if [ -n "$handshake" ] && [ "$handshake" != "0" ]; then
      echo -e " ${GREEN}✓${NC}"
      return 0
    fi
    echo -n "."
    sleep 1
    elapsed=$((elapsed + 1))
  done

  echo -e " ${RED}✗ timeout${NC}"
  return 1
}

show_container_routes() {
  local container=$1
  echo "Routes in $container:"
  docker exec "$container" ip route
}

show_container_interfaces() {
  local container=$1
  echo "Interfaces in $container:"
  docker exec "$container" ip addr
}

#===============================================================================
# SSH Tunnel Helper Functions
#===============================================================================

tunnel_register_key() {
  local container=$1
  local jwt=$2

  echo -n "  → Registering tunnel key in $container"

  # Register tunnel (generates key, registers with server, allocates port)
  local output=$(docker exec "$container" /usr/local/bin/roamie tunnel register <<EOF
$jwt
EOF
  2>&1)

  if echo "$output" | grep -q "registered successfully"; then
    echo -e " ${GREEN}✓${NC}"
    return 0
  else
    echo -e " ${RED}✗${NC}"
    echo "$output"
    return 1
  fi
}

tunnel_enable_via_api() {
  local device_id=$1
  local jwt=$2
  local server=$3

  echo -n "  → Enabling tunnel via API for device $device_id"

  local response=$(curl -s -X PATCH \
    -H "Authorization: Bearer $jwt" \
    -H "Content-Type: application/json" \
    "$server/api/devices/$device_id/tunnel/enable")

  if echo "$response" | grep -q "tunnel enabled"; then
    echo -e " ${GREEN}✓${NC}"
    return 0
  else
    echo -e " ${RED}✗${NC}"
    echo "Response: $response"
    return 1
  fi
}

tunnel_get_allocated_port() {
  local jwt=$1
  local server=$2
  local device_id=$3

  local response=$(curl -s -X GET \
    -H "Authorization: Bearer $jwt" \
    "$server/api/tunnel/status")

  # Extract port for specific device
  local port=$(echo "$response" | jq -r ".tunnels[] | select(.device_id==\"$device_id\") | .port")

  if [ -n "$port" ] && [ "$port" != "null" ]; then
    echo "$port"
    return 0
  else
    echo "0"
    return 1
  fi
}

tunnel_start_client() {
  local container=$1

  echo -n "  → Starting tunnel client in $container"

  # Start tunnel in background
  docker exec -d "$container" sh -c "nohup /usr/local/bin/roamie tunnel start > /tmp/tunnel.log 2>&1 &"

  # Wait a bit for tunnel to establish
  sleep 3

  # Check if tunnel process is running
  if docker exec "$container" pgrep -f "roamie tunnel start" > /dev/null; then
    echo -e " ${GREEN}✓${NC}"
    return 0
  else
    echo -e " ${RED}✗${NC}"
    docker exec "$container" cat /tmp/tunnel.log 2>/dev/null || true
    return 1
  fi
}

tunnel_test_ssh_connection() {
  local from_container=$1
  local tunnel_port=$2
  local server=$3
  local expect_success=$4  # "yes" or "no"

  echo -n "  → Testing SSH through tunnel (port $tunnel_port)"

  # Try to SSH through tunnel using test key
  local output=$(docker exec "$from_container" sh -c "timeout 5 \
    /usr/bin/ssh -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o ConnectTimeout=3 \
        -i /root/.ssh/test_key \
        -p $tunnel_port \
        testuser@$server \
        \"echo 'tunnel_test_success'\"" 2>&1)

  if [ "$expect_success" = "yes" ]; then
    if echo "$output" | grep -q "tunnel_test_success"; then
      echo -e " ${GREEN}✓ Connected${NC}"
      return 0
    else
      echo -e " ${RED}✗ Failed to connect${NC}"
      echo "Output: $output"
      return 1
    fi
  else
    if echo "$output" | grep -q "tunnel_test_success"; then
      echo -e " ${RED}✗ Should have been blocked but connected${NC}"
      return 1
    else
      echo -e " ${GREEN}✓ Correctly blocked${NC}"
      return 0
    fi
  fi
}

tunnel_kill_connection() {
  local container=$1

  echo -n "  → Killing tunnel connection in $container"

  # Kill the roamie tunnel process
  docker exec "$container" pkill -f "roamie tunnel start" || true

  sleep 1

  if ! docker exec "$container" pgrep -f "roamie tunnel start" > /dev/null; then
    echo -e " ${GREEN}✓${NC}"
    return 0
  else
    echo -e " ${RED}✗ Process still running${NC}"
    return 1
  fi
}

tunnel_wait_for_reconnect() {
  local container=$1
  local max_wait=$2
  local elapsed=0

  echo -n "  → Waiting for tunnel auto-reconnect in $container"

  while [ $elapsed -lt $max_wait ]; do
    if docker exec "$container" pgrep -f "roamie tunnel start" > /dev/null 2>&1; then
      # Check if it's actually connected (not just starting)
      sleep 2
      if docker exec "$container" pgrep -f "roamie tunnel start" > /dev/null 2>&1; then
        echo -e " ${GREEN}✓ Reconnected in ${elapsed}s${NC}"
        return 0
      fi
    fi
    echo -n "."
    sleep 1
    elapsed=$((elapsed + 1))
  done

  echo -e " ${RED}✗ timeout${NC}"
  return 1
}

tunnel_show_status() {
  local container=$1
  echo "Tunnel status in $container:"
  docker exec "$container" /usr/local/bin/roamie tunnel status 2>&1 || echo "Failed to get status"
}

tunnel_check_server_listening() {
  local server=$1

  echo -n "  → Checking if tunnel server is listening on port 2222"

  # Check if server is listening on port 2222
  if docker exec "$server" netstat -tuln | grep -q ":2222"; then
    echo -e " ${GREEN}✓${NC}"
    return 0
  else
    echo -e " ${RED}✗ Not listening${NC}"
    return 1
  fi
}
