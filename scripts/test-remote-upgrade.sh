#!/bin/bash
# Test Remote Upgrade System
# Usage: ./scripts/test-remote-upgrade.sh [device-id]

set -e

SERVER_URL="${ROAMIE_SERVER_URL:-http://178.156.133.88:8081}"
DEVICE_ID="${1:-18b01818-ce05-442c-90f4-a4c072e7d517}"
CONFIG_FILE="${HOME}/.config/roamie/config.json"

echo "=== Roamie Remote Upgrade Test ==="
echo "Server: $SERVER_URL"
echo "Device ID: $DEVICE_ID"
echo ""

# Try to get JWT from local config
if [ -f "$CONFIG_FILE" ]; then
  echo "Loading JWT from local config..."
  JWT=$(jq -r '.jwt' "$CONFIG_FILE" 2>/dev/null || echo "")

  if [ -n "$JWT" ] && [ "$JWT" != "null" ]; then
    echo "✓ Using local JWT"
  else
    echo "❌ No valid JWT in config"
    echo "Run: ./roamie auth login"
    exit 1
  fi
else
  echo "❌ Config file not found: $CONFIG_FILE"
  echo "Run: ./roamie auth login"
  exit 1
fi

# Step 1: Trigger remote upgrade
echo ""
echo "Step 1: Triggering remote upgrade..."
TRIGGER_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/devices/$DEVICE_ID/trigger-upgrade" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json")

REQUEST_ID=$(echo "$TRIGGER_RESPONSE" | jq -r '.request_id')

if [ "$REQUEST_ID" = "null" ] || [ -z "$REQUEST_ID" ]; then
  echo "❌ Failed to trigger upgrade"
  echo "$TRIGGER_RESPONSE" | jq .
  exit 1
fi

echo "✓ Upgrade triggered!"
echo "Request ID: $REQUEST_ID"
echo "$TRIGGER_RESPONSE" | jq .

# Step 2: Wait for daemon to process (30s polling interval)
echo ""
echo "Step 2: Waiting for daemon to process upgrade request..."
echo "(Daemon polls every 30 seconds, max wait: 2 minutes)"

for i in {1..40}; do
  sleep 3
  echo -n "."

  # Check if result exists
  RESULT=$(curl -s "$SERVER_URL/api/devices/$DEVICE_ID/upgrade/$REQUEST_ID" \
    -H "Authorization: Bearer $JWT" 2>/dev/null || echo "")

  if echo "$RESULT" | jq -e '.request_id' > /dev/null 2>&1; then
    echo ""
    echo ""
    echo "✅ Upgrade result received!"
    echo ""
    echo "=== Full Result ==="
    echo "$RESULT" | jq .

    echo ""
    echo "=== Summary ==="
    SUCCESS=$(echo "$RESULT" | jq -r '.success')
    PREV_VERSION=$(echo "$RESULT" | jq -r '.previous_version')
    NEW_VERSION=$(echo "$RESULT" | jq -r '.new_version')
    ERROR_MSG=$(echo "$RESULT" | jq -r '.error_message')

    if [ "$SUCCESS" = "true" ]; then
      echo "✓ Upgrade successful!"
      echo "  Previous version: $PREV_VERSION"
      echo "  New version: $NEW_VERSION"
    else
      echo "❌ Upgrade failed!"
      echo "  Version: $PREV_VERSION"
      echo "  Error: $ERROR_MSG"
    fi

    exit 0
  fi
done

echo ""
echo "⏱️  Timeout waiting for result (2 minutes)"
echo ""
echo "Troubleshooting:"
echo "1. Check if daemon is running on device:"
echo "   ps aux | grep roamie"
echo ""
echo "2. Check daemon logs:"
echo "   journalctl --user -f | grep -i upgrade"
echo ""
echo "3. Manually check pending upgrades:"
echo "   curl -H \"Authorization: Bearer \$JWT\" $SERVER_URL/api/devices/upgrades/pending"
