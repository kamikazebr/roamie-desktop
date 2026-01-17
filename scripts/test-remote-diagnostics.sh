#!/bin/bash
# Test Remote Diagnostics System
# Usage: ./scripts/test-remote-diagnostics.sh [device-id]

set -e

SERVER_URL="${ROAMIE_SERVER_URL:-http://178.156.133.88:8081}"
DEVICE_ID="${1:-18b01818-ce05-442c-90f4-a4c072e7d517}"
CONFIG_FILE="${HOME}/.config/roamie/config.json"

echo "=== Roamie Remote Diagnostics Test ==="
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

# Step 1: Trigger diagnostics
echo ""
echo "Step 1: Triggering remote diagnostics..."
TRIGGER_RESPONSE=$(curl -s -X POST "$SERVER_URL/api/devices/$DEVICE_ID/trigger-doctor" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json")

REQUEST_ID=$(echo "$TRIGGER_RESPONSE" | jq -r '.request_id')

if [ "$REQUEST_ID" = "null" ] || [ -z "$REQUEST_ID" ]; then
  echo "❌ Failed to trigger diagnostics"
  echo "$TRIGGER_RESPONSE" | jq .
  exit 1
fi

echo "✓ Diagnostics triggered!"
echo "Request ID: $REQUEST_ID"
echo "$TRIGGER_RESPONSE" | jq .

# Step 2: Wait for daemon to process (30s polling interval)
echo ""
echo "Step 2: Waiting for daemon to process request..."
echo "(Daemon polls every 30 seconds, max wait: 2 minutes)"

for i in {1..40}; do
  sleep 3
  echo -n "."

  # Check if report exists
  REPORT=$(curl -s "$SERVER_URL/api/devices/$DEVICE_ID/diagnostics/$REQUEST_ID" \
    -H "Authorization: Bearer $JWT" 2>/dev/null || echo "")

  if echo "$REPORT" | jq -e '.request_id' > /dev/null 2>&1; then
    echo ""
    echo ""
    echo "✅ Diagnostics report received!"
    echo ""
    echo "=== Full Report ==="
    echo "$REPORT" | jq .

    echo ""
    echo "=== Summary ==="
    echo "$REPORT" | jq '.summary'

    echo ""
    echo "=== System Info ==="
    echo "$REPORT" | jq '{client_version, os, platform, ran_at}'

    echo ""
    echo "=== Failed/Warning Checks ==="
    echo "$REPORT" | jq '.checks[] | select(.status != 0) | {category, name, status, message, fixes}'

    exit 0
  fi
done

echo ""
echo "⏱️  Timeout waiting for report (2 minutes)"
echo ""
echo "Troubleshooting:"
echo "1. Check if daemon is running on device:"
echo "   ssh device 'systemctl --user status roamie'"
echo ""
echo "2. Check daemon logs:"
echo "   ssh device 'journalctl --user -u roamie -f | grep diagnostics'"
echo ""
echo "3. Manually check pending requests:"
echo "   curl -H \"Authorization: Bearer \$JWT\" $SERVER_URL/api/devices/diagnostics/pending"
