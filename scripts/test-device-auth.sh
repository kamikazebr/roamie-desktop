#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVER_URL="${SERVER_URL:-http://10.100.0.1:8081}"
TEST_EMAIL="${TEST_EMAIL:-felipe.novaes.rocha@gmail.com}"
TEST_DEVICE_ID="test-device-$(uuidgen)"
CLEANUP_ON_SUCCESS="${CLEANUP_ON_SUCCESS:-true}"

echo "=== Roamie VPN Device Authorization Flow Test ==="
echo "Server: $SERVER_URL"
echo "Email: $TEST_EMAIL"
echo "Device ID: $TEST_DEVICE_ID"
echo ""

# Step 1: Request auth code
echo -e "${YELLOW}[Step 1/7]${NC} Requesting authentication code..."
AUTH_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/request-code" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\"}")

if echo "$AUTH_RESPONSE" | grep -q "Code sent"; then
  echo -e "${GREEN}✓${NC} Auth code sent to email"
else
  echo -e "${RED}✗${NC} Failed to request auth code"
  echo "$AUTH_RESPONSE"
  exit 1
fi

# Step 2: Get auth code from database
echo -e "${YELLOW}[Step 2/7]${NC} Retrieving auth code from database..."
AUTH_CODE=$(ssh root@178.156.133.88 "docker exec roamie-postgres psql -U culodi -d roamie_vpn -t -c \"SELECT code FROM auth_codes WHERE email = '$TEST_EMAIL' ORDER BY created_at DESC LIMIT 1;\"" | tr -d ' ')

if [ -z "$AUTH_CODE" ]; then
  echo -e "${RED}✗${NC} Failed to retrieve auth code"
  exit 1
fi
echo -e "${GREEN}✓${NC} Auth code: $AUTH_CODE"

# Step 3: Verify code and get JWT
echo -e "${YELLOW}[Step 3/7]${NC} Verifying code and obtaining JWT..."
JWT_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/auth/verify-code" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"code\":\"$AUTH_CODE\"}")

JWT_TOKEN=$(echo "$JWT_RESPONSE" | jq -r '.token')
if [ -z "$JWT_TOKEN" ] || [ "$JWT_TOKEN" == "null" ]; then
  echo -e "${RED}✗${NC} Failed to get JWT token"
  echo "$JWT_RESPONSE"
  exit 1
fi
echo -e "${GREEN}✓${NC} JWT obtained"

# Step 4: Create device authorization request
echo -e "${YELLOW}[Step 4/7]${NC} Creating device authorization request..."
DEVICE_REQUEST=$(curl -s -X POST "$SERVER_URL/api/auth/device-request" \
  -H "Content-Type: application/json" \
  -d "{\"device_id\":\"$TEST_DEVICE_ID\",\"hostname\":\"test-laptop\"}")

CHALLENGE_ID=$(echo "$DEVICE_REQUEST" | jq -r '.challenge_id')
QR_DATA=$(echo "$DEVICE_REQUEST" | jq -r '.qr_data')

if [ -z "$CHALLENGE_ID" ] || [ "$CHALLENGE_ID" == "null" ]; then
  echo -e "${RED}✗${NC} Failed to create device request"
  echo "$DEVICE_REQUEST"
  exit 1
fi
echo -e "${GREEN}✓${NC} Challenge created: $CHALLENGE_ID"
echo "  QR Data: $QR_DATA"

# Step 5: List pending devices
echo -e "${YELLOW}[Step 5/7]${NC} Listing pending device authorizations..."
PENDING_DEVICES=$(curl -s "$SERVER_URL/api/device-auth/pending" \
  -H "Authorization: Bearer $JWT_TOKEN")

PENDING_COUNT=$(echo "$PENDING_DEVICES" | jq -r '.count')
if [ "$PENDING_COUNT" -lt 1 ]; then
  echo -e "${RED}✗${NC} No pending devices found"
  echo "$PENDING_DEVICES"
  exit 1
fi
echo -e "${GREEN}✓${NC} Found $PENDING_COUNT pending device(s)"

# Step 6: Approve device
echo -e "${YELLOW}[Step 6/7]${NC} Approving device..."
APPROVE_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/device-auth/approve" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d "{\"challenge_id\":\"$CHALLENGE_ID\",\"approved\":true}")

APPROVE_STATUS=$(echo "$APPROVE_RESPONSE" | jq -r '.status')
if [ "$APPROVE_STATUS" != "approved" ]; then
  echo -e "${RED}✗${NC} Failed to approve device"
  echo "$APPROVE_RESPONSE"
  exit 1
fi
echo -e "${GREEN}✓${NC} Device approved"

# Step 7: Poll for JWT and refresh token
echo -e "${YELLOW}[Step 7/7]${NC} Polling for device JWT and refresh token..."
POLL_RESPONSE=$(curl -s "$SERVER_URL/api/auth/device-poll/$CHALLENGE_ID")

DEVICE_JWT=$(echo "$POLL_RESPONSE" | jq -r '.jwt')
REFRESH_TOKEN=$(echo "$POLL_RESPONSE" | jq -r '.refresh_token')
EXPIRES_AT=$(echo "$POLL_RESPONSE" | jq -r '.expires_at')

if [ -z "$DEVICE_JWT" ] || [ "$DEVICE_JWT" == "null" ]; then
  echo -e "${RED}✗${NC} Failed to get device JWT"
  echo "$POLL_RESPONSE"
  exit 1
fi

echo -e "${GREEN}✓${NC} Device JWT obtained"
echo "  Expires: $EXPIRES_AT"
echo "  Refresh token: ${REFRESH_TOKEN:0:20}..."

# Cleanup
if [ "$CLEANUP_ON_SUCCESS" == "true" ]; then
  echo ""
  echo -e "${YELLOW}Cleaning up test data...${NC}"
  ssh root@178.156.133.88 "docker exec roamie-postgres psql -U culodi -d roamie_vpn -c \"DELETE FROM auth_codes WHERE email = '$TEST_EMAIL';\" > /dev/null 2>&1" || true
  ssh root@178.156.133.88 "docker exec roamie-postgres psql -U culodi -d roamie_vpn -c \"DELETE FROM refresh_tokens WHERE device_id = '$TEST_DEVICE_ID';\" > /dev/null 2>&1" || true
  ssh root@178.156.133.88 "docker exec roamie-postgres psql -U culodi -d roamie_vpn -c \"DELETE FROM device_auth_challenges WHERE device_id = '$TEST_DEVICE_ID';\" > /dev/null 2>&1" || true
  echo -e "${GREEN}✓${NC} Cleanup complete"
fi

echo ""
echo -e "${GREEN}=== All Tests Passed! ===${NC}"
echo ""
echo "Summary:"
echo "  - Device authorization request created"
echo "  - Device appeared in pending list"
echo "  - Device approved successfully"
echo "  - Device JWT and refresh token obtained"
echo ""
echo "Device Authorization Flow: WORKING ✓"
