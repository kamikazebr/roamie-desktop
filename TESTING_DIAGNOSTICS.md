# Testing Remote Diagnostics System

## Prerequisites
- Server deployed with `server-v0.0.10` (with Firestore configured)
- Client v0.0.10 installed with daemon running
- Device authenticated and registered

## Architecture Overview
```
Admin (Mobile/API) → Server → Firestore → Server ← Client Daemon (polls every 30s)
                        ↓                       ↓
                   Trigger request         Runs doctor
                        ↓                       ↓
                   Firestore ← Upload report ← Client
```

## Test Flow

### Step 1: Verify Setup

```bash
# On client machine - verify version
./roamie version
# Should show: v0.0.10

# Verify daemon is running
systemctl --user status roamie
# Should show: active (running)

# Check daemon logs to see diagnostics ticker
journalctl --user -u roamie -f | grep -i diagnostics
```

### Step 2: Trigger Remote Diagnostics (from API or mobile)

```bash
# Get device ID from authenticated client
./roamie devices list
# Copy the device ID (UUID format)

# Trigger diagnostics via API
export SERVER_URL="https://your-server.com"  # or http://localhost:8081
export JWT="your-jwt-token"
export DEVICE_ID="your-device-uuid"

curl -X POST "$SERVER_URL/api/devices/$DEVICE_ID/trigger-doctor" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json"

# Response:
# {
#   "message": "Diagnostics request created",
#   "request_id": "550e8400-e29b-41d4-a716-446655440000"
# }
```

### Step 3: Watch Client Daemon Process Request

The daemon polls every 30 seconds, so wait up to 30s and watch logs:

```bash
journalctl --user -u roamie -f
```

You should see:
```
[timestamp] Running diagnostics for request 550e8400-e29b-41d4-a716-446655440000...
[timestamp] ✓ Diagnostics report uploaded successfully for request 550e8400-e29b-41d4-a716-446655440000
```

### Step 4: Retrieve Diagnostics Report

```bash
# Get specific report
curl "$SERVER_URL/api/devices/$DEVICE_ID/diagnostics/$REQUEST_ID" \
  -H "Authorization: Bearer $JWT"

# Or list all reports for device
curl "$SERVER_URL/api/devices/$DEVICE_ID/diagnostics" \
  -H "Authorization: Bearer $JWT"
```

Expected response:
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "device_id": "abc-123-device-uuid",
  "ran_at": "2025-01-08T10:01:00Z",
  "checks": [
    {
      "name": "Config loaded",
      "category": "Authentication",
      "status": "PASSED",
      "message": "Config loaded",
      "fixes": []
    },
    {
      "name": "JWT validity",
      "category": "Authentication",
      "status": "PASSED",
      "message": "JWT valid (167h remaining)",
      "fixes": []
    },
    {
      "name": "Server reachable",
      "category": "Authentication",
      "status": "PASSED",
      "message": "Server reachable (https://...)",
      "fixes": []
    },
    {
      "name": "Daemon running",
      "category": "Services",
      "status": "PASSED",
      "message": "Daemon running",
      "fixes": []
    }
  ],
  "summary": {
    "passed": 8,
    "warnings": 1,
    "errors": 0,
    "info": 0
  },
  "client_version": "v0.0.10",
  "os": "linux",
  "platform": "amd64"
}
```

### Step 5: Verify Firestore Data

Check Firebase Console:
- Collection: `diagnostics_requests/{device_id}/pending`
  - Should be EMPTY after processing (cleaned up)
- Collection: `diagnostics_reports/{device_id}/reports`
  - Should contain the report document with request_id

## Troubleshooting

### Client doesn't pick up request
```bash
# Check daemon is running
systemctl --user status roamie

# Check daemon logs for errors
journalctl --user -u roamie -n 50

# Manually check for pending requests
curl "$SERVER_URL/api/devices/diagnostics/pending" \
  -H "Authorization: Bearer $JWT"
```

### Report not uploaded
```bash
# Check daemon logs
journalctl --user -u roamie | grep -i "diagnostics\|error"

# Verify Firestore credentials on server
echo $FIREBASE_CREDENTIALS_PATH
# Should point to valid service account JSON
```

### Permission errors
Ensure server has Firestore permissions:
- Read/Write to `diagnostics_requests` collection
- Read/Write to `diagnostics_reports` collection

## Local Development Testing

### Without Firestore (quick test)
For quick local testing without Firestore setup, you can:

1. Run server locally: `./roamie-server`
2. Mock the DiagnosticsService to return empty results
3. Test API endpoints with curl
4. Verify client can call endpoints (even if Firestore fails gracefully)

### With Firestore (full integration)
1. Set `FIREBASE_CREDENTIALS_PATH` in server `.env`
2. Ensure service account has Firestore permissions
3. Run full flow as described above

## Performance Notes

- Daemon polls every **30 seconds**
- Multiple requests are processed in sequence
- Each doctor run takes ~1-2 seconds
- Network latency: server → Firestore (varies by region)

## Success Criteria

✅ Client daemon picks up request within 30 seconds
✅ Doctor runs automatically without user action
✅ Report uploads successfully to Firestore
✅ Pending request is deleted after upload
✅ Admin can view report via API
✅ System handles multiple concurrent devices
