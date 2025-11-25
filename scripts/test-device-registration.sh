#!/bin/bash

# Test script for device registration and WireGuard peer management
# Tests: add device, replace device, bypass device

set -e

SERVER_URL="${1:-http://localhost:8081}"
TEST_EMAIL="test-$(date +%s)@example.com"
DEVICE_NAME="test-device"

echo "==================================="
echo "Device Registration Test Suite"
echo "==================================="
echo "Server: $SERVER_URL"
echo "Test Email: $TEST_EMAIL"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to generate WireGuard key pair
generate_keys() {
    PRIVATE_KEY=$(wg genkey)
    PUBLIC_KEY=$(echo "$PRIVATE_KEY" | wg pubkey)
}

# Function to check if server is running
check_server() {
    echo -n "Checking server health... "
    if curl -s "$SERVER_URL/health" > /dev/null; then
        echo -e "${GREEN}✓ OK${NC}"
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "Server is not running at $SERVER_URL"
        exit 1
    fi
}

# Function to request auth code
request_code() {
    echo -n "Requesting auth code for $TEST_EMAIL... "
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/request-code" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$TEST_EMAIL\"}")

    if echo "$RESPONSE" | grep -q "Code sent to email"; then
        echo -e "${GREEN}✓ OK${NC}"
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "Response: $RESPONSE"
        exit 1
    fi
}

# Function to get auth code from database (for testing)
get_auth_code() {
    echo -n "Getting auth code from database... "

    # Try remote server first (if testing against production)
    if [[ "$SERVER_URL" == *"178.156.133.88"* ]]; then
        CODE=$(ssh root@178.156.133.88 "docker exec roamie-postgres psql -U roamie -d roamie_vpn -t -c \"SELECT code FROM auth_codes WHERE email='$TEST_EMAIL' ORDER BY created_at DESC LIMIT 1\"" 2>/dev/null | xargs)
    else
        # Local testing with Docker PostgreSQL
        CODE=$(docker exec roamie-postgres psql -U roamie -d roamie_vpn -t -c \
            "SELECT code FROM auth_codes WHERE email='$TEST_EMAIL' ORDER BY created_at DESC LIMIT 1" 2>/dev/null | xargs)
    fi

    if [ -z "$CODE" ]; then
        echo -e "${YELLOW}⚠ Could not get code from database${NC}"
        echo "You need to manually enter the code from email or database"
        echo -n "Enter auth code: "
        read CODE
    else
        echo -e "${GREEN}✓ Code: $CODE${NC}"
    fi
}

# Function to verify auth code and get JWT
verify_code() {
    echo -n "Verifying auth code... "
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/verify-code" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$TEST_EMAIL\",\"code\":\"$CODE\"}")

    JWT=$(echo "$RESPONSE" | jq -r '.token' 2>/dev/null)

    if [ "$JWT" != "null" ] && [ -n "$JWT" ]; then
        echo -e "${GREEN}✓ OK${NC}"
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "Response: $RESPONSE"
        exit 1
    fi
}

# Function to count WireGuard peers
count_wg_peers() {
    local count
    if [[ "$SERVER_URL" == *"178.156.133.88"* ]]; then
        count=$(ssh root@178.156.133.88 '/root/roamie-server admin list-peers 2>/dev/null' | grep -c "^[A-Za-z0-9+/=]\{44\}" 2>/dev/null || echo "0")
    else
        count=$(./roamie-server admin list-peers 2>/dev/null | grep -c "^[A-Za-z0-9+/=]\{44\}" 2>/dev/null || echo "0")
    fi
    echo "$count"
}

# Function to list WireGuard peers
list_wg_peers() {
    echo ""
    echo "WireGuard Peers:"
    if [[ "$SERVER_URL" == *"178.156.133.88"* ]]; then
        ssh root@178.156.133.88 '/root/roamie-server admin list-peers'
    else
        ./roamie-server admin list-peers
    fi
    echo ""
}

# Function to register device
register_device() {
    local device_name="$1"
    local public_key="$2"

    echo -n "Registering device '$device_name'... "
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/devices" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $JWT" \
        -d "{\"device_name\":\"$device_name\",\"public_key\":\"$public_key\"}")

    DEVICE_ID=$(echo "$RESPONSE" | jq -r '.device_id' 2>/dev/null)

    if [ "$DEVICE_ID" != "null" ] && [ -n "$DEVICE_ID" ]; then
        echo -e "${GREEN}✓ OK${NC} (ID: $DEVICE_ID)"
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "Response: $RESPONSE"
        return 1
    fi
}

echo "======================================"
echo "Test 0: Prerequisites"
echo "======================================"
check_server
echo ""

echo "======================================"
echo "Test 1: Setup - Create Test User"
echo "======================================"
request_code
get_auth_code
verify_code
echo ""

echo "======================================"
echo "Test 2: Initial State"
echo "======================================"
echo "Checking WireGuard peers before any device registration..."
INITIAL_PEERS=$(count_wg_peers)
echo "Current peers: $INITIAL_PEERS"
list_wg_peers

echo "======================================"
echo "Test 3: Register First Device"
echo "======================================"
echo "Expected: Device should be added to database AND WireGuard"
generate_keys
KEY1_PUBLIC="$PUBLIC_KEY"
echo "Generated keys:"
echo "  Public Key: $KEY1_PUBLIC"

if register_device "$DEVICE_NAME" "$KEY1_PUBLIC"; then
    sleep 1
    PEERS_AFTER=$(count_wg_peers)
    echo ""
    echo "Peers before: $INITIAL_PEERS"
    echo "Peers after:  $PEERS_AFTER"

    if [ "$PEERS_AFTER" -gt "$INITIAL_PEERS" ]; then
        echo -e "${GREEN}✓ TEST PASSED${NC} - Peer was added to WireGuard"
    else
        echo -e "${RED}✗ TEST FAILED${NC} - Peer was NOT added to WireGuard"
    fi
    list_wg_peers
else
    echo -e "${RED}✗ TEST FAILED${NC} - Could not register device"
    exit 1
fi

echo "======================================"
echo "Test 4: Replace Device (Different Key)"
echo "======================================"
echo "Expected: Old peer removed, new peer added"
generate_keys
KEY2_PUBLIC="$PUBLIC_KEY"
echo "Generated new keys:"
echo "  Public Key: $KEY2_PUBLIC"

PEERS_BEFORE=$(count_wg_peers)
if register_device "$DEVICE_NAME" "$KEY2_PUBLIC"; then
    sleep 1
    PEERS_AFTER=$(count_wg_peers)

    echo ""
    echo "Peers before replace: $PEERS_BEFORE"
    echo "Peers after replace:  $PEERS_AFTER"

    if [ "$PEERS_AFTER" -eq "$PEERS_BEFORE" ]; then
        echo -e "${GREEN}✓ TEST PASSED${NC} - Peer count stayed same (replaced)"

        # Check if new key exists
        if [[ "$SERVER_URL" == *"178.156.133.88"* ]]; then
            PEER_LIST=$(ssh root@178.156.133.88 '/root/roamie-server admin list-peers')
        else
            PEER_LIST=$(./roamie-server admin list-peers)
        fi

        if echo "$PEER_LIST" | grep -q "$KEY2_PUBLIC"; then
            echo -e "${GREEN}✓ TEST PASSED${NC} - New peer exists in WireGuard"
        else
            echo -e "${RED}✗ TEST FAILED${NC} - New peer NOT found in WireGuard"
        fi

        # Check if old key is gone
        if ! echo "$PEER_LIST" | grep -q "$KEY1_PUBLIC"; then
            echo -e "${GREEN}✓ TEST PASSED${NC} - Old peer removed from WireGuard"
        else
            echo -e "${RED}✗ TEST FAILED${NC} - Old peer still in WireGuard"
        fi
    else
        echo -e "${RED}✗ TEST FAILED${NC} - Peer count changed unexpectedly"
    fi
    list_wg_peers
else
    echo -e "${RED}✗ TEST FAILED${NC} - Could not replace device"
fi

echo "======================================"
echo "Test 5: Bypass (Same Key)"
echo "======================================"
echo "Expected: Return existing device, no WireGuard changes"

PEERS_BEFORE=$(count_wg_peers)
if register_device "$DEVICE_NAME" "$KEY2_PUBLIC"; then
    sleep 1
    PEERS_AFTER=$(count_wg_peers)

    echo ""
    echo "Peers before: $PEERS_BEFORE"
    echo "Peers after:  $PEERS_AFTER"

    if [ "$PEERS_AFTER" -eq "$PEERS_BEFORE" ]; then
        echo -e "${GREEN}✓ TEST PASSED${NC} - No peer changes (bypass worked)"
    else
        echo -e "${RED}✗ TEST FAILED${NC} - Peer count changed (should not change)"
    fi
    list_wg_peers
else
    echo -e "${RED}✗ TEST FAILED${NC} - Could not register device"
fi

echo "======================================"
echo "Test 6: Cleanup"
echo "======================================"
echo "Deleting test device..."
if [[ "$SERVER_URL" == *"178.156.133.88"* ]]; then
    DELETE_CMD="ssh root@178.156.133.88 '/root/roamie-server admin delete-device --email=\"$TEST_EMAIL\" --device-name=\"$DEVICE_NAME\"'"
    if echo "yes" | eval "$DELETE_CMD" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Device deleted${NC}"
    else
        echo -e "${YELLOW}⚠ Could not delete test device${NC}"
    fi
else
    if ./roamie-server admin delete-device --email="$TEST_EMAIL" --device-name="$DEVICE_NAME" <<< "yes" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Device deleted${NC}"
    else
        echo -e "${YELLOW}⚠ Could not delete test device${NC}"
    fi
fi

sleep 1
FINAL_PEERS=$(count_wg_peers)
echo "Final peer count: $FINAL_PEERS"

if [ "$FINAL_PEERS" -eq "$INITIAL_PEERS" ]; then
    echo -e "${GREEN}✓ TEST PASSED${NC} - Peer removed from WireGuard after deletion"
else
    echo -e "${RED}✗ TEST FAILED${NC} - Peer still in WireGuard after deletion"
fi
list_wg_peers

echo ""
echo "======================================"
echo "Test Summary"
echo "======================================"
echo "All tests completed!"
echo ""
echo "Manual verification:"
echo "1. Check database: ./roamie-server admin list-devices --email=$TEST_EMAIL"
echo "2. Check WireGuard: ./roamie-server admin list-peers"
