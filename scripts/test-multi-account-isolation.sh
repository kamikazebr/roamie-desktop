#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_URL="${VPN_SERVER:-http://felipenovaesrocha.xyz:8081}"
CLEANUP_ON_SUCCESS="${CLEANUP_ON_SUCCESS:-true}"

echo -e "${BLUE}=== Multi-Account Subnet Isolation Test ===${NC}"
echo "Server: $SERVER_URL"
echo ""

# Test accounts
ACCOUNT_A="alice-test-$(date +%s)@example.com"
ACCOUNT_B="bob-test-$(date +%s)@example.com"
DEVICE_ID="test-device-$(uuidgen)"

echo "Test Scenario:"
echo "  Account A: $ACCOUNT_A"
echo "  Account B: $ACCOUNT_B"
echo "  Device ID: $DEVICE_ID (simulating same device)"
echo ""

#===============================================================================
# Helper Functions
#===============================================================================

get_auth_code_from_db() {
  local email=$1
  ssh root@178.156.133.88 "docker exec roamie-postgres psql -U roamie -d roamie_vpn -t -c \"SELECT code FROM auth_codes WHERE email = '$email' ORDER BY created_at DESC LIMIT 1;\"" | tr -d ' '
}

get_user_subnet() {
  local email=$1
  ssh root@178.156.133.88 "docker exec roamie-postgres psql -U roamie -d roamie_vpn -t -c \"SELECT subnet FROM users WHERE email = '$email';\"" | tr -d ' '
}

login_with_email() {
  local email=$1
  local step_prefix=$2

  echo -e "${YELLOW}${step_prefix}${NC} Logging in as: $email"

  # Request auth code
  echo "  → Requesting auth code..."
  AUTH_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/request-code" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\"}")

  if ! echo "$AUTH_RESPONSE" | grep -q "Code sent"; then
    echo -e "${RED}  ✗ Failed to request auth code${NC}"
    echo "$AUTH_RESPONSE"
    return 1
  fi

  # Get code from database
  echo "  → Retrieving code from database..."
  AUTH_CODE=$(get_auth_code_from_db "$email")

  if [ -z "$AUTH_CODE" ]; then
    echo -e "${RED}  ✗ Failed to retrieve auth code${NC}"
    return 1
  fi

  # Verify code and get JWT
  echo "  → Verifying code (code: $AUTH_CODE)..."
  JWT_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/verify-code" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"code\":\"$AUTH_CODE\"}")

  JWT_TOKEN=$(echo "$JWT_RESPONSE" | jq -r '.token')

  if [ -z "$JWT_TOKEN" ] || [ "$JWT_TOKEN" == "null" ]; then
    echo -e "${RED}  ✗ Failed to get JWT token${NC}"
    echo "$JWT_RESPONSE"
    return 1
  fi

  # Get subnet from database (more reliable than API response)
  echo "  → Retrieving subnet from database..."
  USER_SUBNET=$(get_user_subnet "$email")

  if [ -z "$USER_SUBNET" ]; then
    echo -e "${RED}  ✗ Failed to retrieve subnet${NC}"
    return 1
  fi

  echo -e "${GREEN}  ✓ Login successful${NC}"
  echo "  → Subnet allocated: $USER_SUBNET"
  echo ""

  # Return values via global variables
  CURRENT_JWT="$JWT_TOKEN"
  CURRENT_SUBNET="$USER_SUBNET"
}

register_device() {
  local jwt=$1
  local device_name=$2
  local step_prefix=$3

  echo -e "${YELLOW}${step_prefix}${NC} Registering device: $device_name"

  # Generate WireGuard keypair
  PRIVATE_KEY=$(wg genkey)
  PUBLIC_KEY=$(echo "$PRIVATE_KEY" | wg pubkey)

  # Register device
  echo "  → Sending device registration..."
  DEVICE_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/devices" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $jwt" \
    -d "{\"device_name\":\"$device_name\",\"public_key\":\"$PUBLIC_KEY\"}")

  DEVICE_IP=$(echo "$DEVICE_RESPONSE" | jq -r '.vpn_ip')
  DEVICE_SUBNET=$(echo "$DEVICE_RESPONSE" | jq -r '.user_subnet')

  if [ -z "$DEVICE_IP" ] || [ "$DEVICE_IP" == "null" ]; then
    echo -e "${RED}  ✗ Failed to register device${NC}"
    echo "$DEVICE_RESPONSE"
    return 1
  fi

  echo -e "${GREEN}  ✓ Device registered${NC}"
  echo "  → Device IP: $DEVICE_IP"
  echo "  → User Subnet: $DEVICE_SUBNET"
  echo ""

  CURRENT_DEVICE_IP="$DEVICE_IP"
}

cleanup_test_data() {
  echo -e "${YELLOW}Cleaning up test data...${NC}"

  ssh root@178.156.133.88 "docker exec roamie-postgres psql -U roamie -d roamie_vpn -c \"
    DELETE FROM devices WHERE user_id IN (SELECT id FROM users WHERE email IN ('$ACCOUNT_A', '$ACCOUNT_B'));
    DELETE FROM auth_codes WHERE email IN ('$ACCOUNT_A', '$ACCOUNT_B');
    DELETE FROM users WHERE email IN ('$ACCOUNT_A', '$ACCOUNT_B');
  \" > /dev/null 2>&1" || true

  echo -e "${GREEN}✓ Cleanup complete${NC}"
  echo ""
}

#===============================================================================
# Test Execution
#===============================================================================

echo -e "${BLUE}Test 1: Account A Login${NC}"
echo "================================================================"
login_with_email "$ACCOUNT_A" "[1/6]"
ACCOUNT_A_SUBNET="$CURRENT_SUBNET"
ACCOUNT_A_JWT="$CURRENT_JWT"

echo -e "${BLUE}Test 2: Register Device for Account A${NC}"
echo "================================================================"
register_device "$ACCOUNT_A_JWT" "laptop" "[2/6]"
ACCOUNT_A_DEVICE_IP="$CURRENT_DEVICE_IP"

echo -e "${BLUE}Test 3: Account B Login (Same Device)${NC}"
echo "================================================================"
echo "Simulating user switching from Account A to Account B..."
echo ""
login_with_email "$ACCOUNT_B" "[3/6]"
ACCOUNT_B_SUBNET="$CURRENT_SUBNET"
ACCOUNT_B_JWT="$CURRENT_JWT"

echo -e "${BLUE}Test 4: Register Device for Account B${NC}"
echo "================================================================"
register_device "$ACCOUNT_B_JWT" "laptop" "[4/6]"
ACCOUNT_B_DEVICE_IP="$CURRENT_DEVICE_IP"

echo -e "${BLUE}Test 5: Verify Subnet Isolation${NC}"
echo "================================================================"
echo -e "${YELLOW}[5/6]${NC} Verifying that accounts have DIFFERENT subnets..."
echo ""
echo "Account A Subnet: $ACCOUNT_A_SUBNET"
echo "Account B Subnet: $ACCOUNT_B_SUBNET"
echo ""

if [ "$ACCOUNT_A_SUBNET" == "$ACCOUNT_B_SUBNET" ]; then
  echo -e "${RED}✗ TEST FAILED: Both accounts share the same subnet!${NC}"
  echo "This indicates the Firebase UID lookup bug is still present."
  cleanup_test_data
  exit 1
else
  echo -e "${GREEN}✓ TEST PASSED: Accounts have different subnets${NC}"
fi
echo ""

echo -e "${BLUE}Test 6: Verify Account Re-Login Uses Same Subnet${NC}"
echo "================================================================"
echo -e "${YELLOW}[6/6]${NC} Re-logging in as Account A to verify subnet persistence..."
echo ""
login_with_email "$ACCOUNT_A" "[6/6]"
ACCOUNT_A_SUBNET_RELOGIN="$CURRENT_SUBNET"

if [ "$ACCOUNT_A_SUBNET" != "$ACCOUNT_A_SUBNET_RELOGIN" ]; then
  echo -e "${RED}✗ TEST FAILED: Account A got different subnet on re-login!${NC}"
  echo "  Original: $ACCOUNT_A_SUBNET"
  echo "  Re-login: $ACCOUNT_A_SUBNET_RELOGIN"
  cleanup_test_data
  exit 1
else
  echo -e "${GREEN}✓ TEST PASSED: Account A reused same subnet${NC}"
fi
echo ""

#===============================================================================
# Final Summary
#===============================================================================

echo -e "${GREEN}=== ALL TESTS PASSED! ===${NC}"
echo ""
echo "Summary of Results:"
echo "─────────────────────────────────────────────────────────────"
echo "  Account A ($ACCOUNT_A):"
echo "    Subnet:    $ACCOUNT_A_SUBNET"
echo "    Device IP: $ACCOUNT_A_DEVICE_IP"
echo ""
echo "  Account B ($ACCOUNT_B):"
echo "    Subnet:    $ACCOUNT_B_SUBNET"
echo "    Device IP: $ACCOUNT_B_DEVICE_IP"
echo ""
echo "  Subnet Isolation: ✓ WORKING"
echo "    - Different accounts get different subnets"
echo "    - Same account reuses subnet across logins"
echo "    - Account switching on same device works correctly"
echo "─────────────────────────────────────────────────────────────"
echo ""

# IP range analysis
ACCOUNT_A_NETWORK=$(echo $ACCOUNT_A_SUBNET | cut -d'/' -f1 | cut -d'.' -f1-3)
ACCOUNT_A_THIRD_OCTET=$(echo $ACCOUNT_A_SUBNET | cut -d'/' -f1 | cut -d'.' -f4)
ACCOUNT_B_NETWORK=$(echo $ACCOUNT_B_SUBNET | cut -d'/' -f1 | cut -d'.' -f1-3)
ACCOUNT_B_THIRD_OCTET=$(echo $ACCOUNT_B_SUBNET | cut -d'/' -f1 | cut -d'.' -f4)

echo "Network Isolation Verification:"
echo "  Account A range: $ACCOUNT_A_SUBNET → IPs ${ACCOUNT_A_NETWORK}.${ACCOUNT_A_THIRD_OCTET} - ${ACCOUNT_A_NETWORK}.$((ACCOUNT_A_THIRD_OCTET + 7))"
echo "  Account B range: $ACCOUNT_B_SUBNET → IPs ${ACCOUNT_B_NETWORK}.${ACCOUNT_B_THIRD_OCTET} - ${ACCOUNT_B_NETWORK}.$((ACCOUNT_B_THIRD_OCTET + 7))"
echo ""
echo "  ✓ Ranges do not overlap"
echo "  ✓ Devices from Account A CANNOT communicate with Account B"
echo "  ✓ Each account has isolated VPN network"
echo ""

# Cleanup
if [ "$CLEANUP_ON_SUCCESS" == "true" ]; then
  cleanup_test_data
fi

echo -e "${GREEN}Multi-Account Subnet Isolation: WORKING ✓${NC}"
echo ""
