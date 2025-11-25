# Testing Guide

This document describes the testing infrastructure for Roamie VPN, specifically for verifying multi-account subnet isolation.

## Overview

We have two comprehensive testing approaches:

1. **Docker-based Local Testing** - Runs complete VPN infrastructure locally in Docker
2. **Production Testing** - Tests against live production server

## 1. Docker-based Local Testing (Recommended)

### What It Tests

- Complete VPN server with WireGuard support
- PostgreSQL database
- Multiple client simulations
- Multi-account subnet isolation
- Account switching scenarios
- No impact on production data

### Prerequisites

```bash
# Ensure Docker and Docker Compose are installed
docker --version
docker compose version

# Install jq (for JSON parsing)
sudo apt-get install jq  # Ubuntu/Debian
brew install jq          # macOS
```

### Running the Test

```bash
# Run the full Docker-based test suite
./scripts/test-docker-multi-account.sh

# With verbose logs
SHOW_LOGS=true ./scripts/test-docker-multi-account.sh

# Keep containers running after test
CLEANUP_ON_EXIT=false ./scripts/test-docker-multi-account.sh
```

### What the Test Does

1. **Starts Services**: Launches PostgreSQL, VPN server, and client containers
2. **Account A Login**: Creates first user account and gets subnet allocation
3. **Register Device A**: Registers a device for Account A
4. **Account B Login**: Simulates switching to different account on same device
5. **Register Device B**: Registers a device for Account B
6. **Verify Isolation**: Confirms both accounts have DIFFERENT subnets
7. **Verify Persistence**: Confirms re-login reuses the same subnet
8. **Cleanup**: Removes all test containers and volumes

### Expected Output

```
========================================
  Roamie VPN Docker Multi-Account Test
========================================

Test Accounts:
  Account A: alice-test-1234567890@example.com
  Account B: bob-test-1234567890@example.com

✓ Account A Subnet: 10.200.0.0/29
✓ Account B Subnet: 10.200.0.8/29
✓ Subnets are different
✓ Re-login reuses subnet

========================================
     ALL TESTS PASSED! ✓
========================================
```

### Troubleshooting Docker Tests

**Container won't start:**
```bash
# Check logs
docker compose -f docker-compose.test.yml logs

# Rebuild containers
docker compose -f docker-compose.test.yml build --no-cache
docker compose -f docker-compose.test.yml up -d
```

**Port conflicts:**
```bash
# Check what's using ports 8082 or 5433
sudo lsof -i :8082
sudo lsof -i :5433

# Edit docker-compose.test.yml to use different ports if needed
```

**Clean slate:**
```bash
# Remove all test containers and volumes
docker compose -f docker-compose.test.yml down -v

# Remove test images
docker rmi roamie-test-server
```

## 2. Production Testing

### What It Tests

- Live production server (felipenovaesrocha.xyz:8081)
- Real database with production data
- Actual network allocation
- End-to-end production flow

### Prerequisites

```bash
# Ensure you have SSH access to production server
ssh root@178.156.133.88 "echo 'Connection successful'"

# Install jq and curl
sudo apt-get install jq curl
```

### Running the Test

```bash
# Run production test
./scripts/test-multi-account-isolation.sh

# Keep test accounts (don't cleanup)
CLEANUP_ON_SUCCESS=false ./scripts/test-multi-account-isolation.sh

# Use custom server URL
VPN_SERVER=http://localhost:8080 ./scripts/test-multi-account-isolation.sh
```

### What the Test Does

1. Creates two temporary test accounts (alice-test-XXX, bob-test-XXX)
2. Simulates device switching between accounts
3. Verifies subnet isolation
4. Verifies subnet persistence on re-login
5. Cleans up test data from production database

### Expected Output

```
=== Multi-Account Subnet Isolation Test ===
Server: http://felipenovaesrocha.xyz:8081

✓ Account A (alice-test-1234567890@example.com):
  Subnet:    10.100.0.16/29
  Device IP: 10.100.0.18

✓ Account B (bob-test-1234567890@example.com):
  Subnet:    10.100.0.24/29
  Device IP: 10.100.0.26

✓ Subnet Isolation: WORKING
Multi-Account Subnet Isolation: WORKING ✓
```

### Security Note

⚠️ **Production tests create and delete real user accounts**. Test accounts are named `*-test-*@example.com` and are automatically cleaned up unless `CLEANUP_ON_SUCCESS=false`.

## Understanding the Tests

### Multi-Account Isolation Bug (FIXED)

**The Problem:**
When users switched accounts on the same device, both accounts were sharing the same VPN subnet because the server was using Firebase UID (device-based) instead of email for user lookup.

**The Fix:**
Changed `GetOrCreateUserByFirebaseUID()` to look up users by **email first**:

```go
// Before (Bug)
user, err := s.userRepo.GetByFirebaseUID(ctx, firebaseUID)

// After (Fixed)
user, err := s.userRepo.GetByEmail(ctx, email)
```

### What the Tests Verify

1. **Different Accounts → Different Subnets**
   - alice@example.com gets 10.100.0.0/29
   - bob@example.com gets 10.100.0.8/29
   - Subnets do NOT overlap

2. **Same Account → Same Subnet**
   - alice@example.com logs in again
   - Gets same subnet (10.100.0.0/29)
   - Devices can communicate within same account

3. **Network Isolation**
   - Alice's devices: 10.100.0.2 - 10.100.0.6
   - Bob's devices: 10.100.0.10 - 10.100.0.14
   - Cannot communicate across accounts

### Subnet Allocation

Each user gets a `/29` subnet:
- **8 total IPs** (2^3 = 8)
- **6 usable IPs** for devices
- **2 reserved** (network address + broadcast)

Example allocations:
```
User 1: 10.100.0.0/29   → 10.100.0.0 - 10.100.0.7
User 2: 10.100.0.8/29   → 10.100.0.8 - 10.100.0.15
User 3: 10.100.0.16/29  → 10.100.0.16 - 10.100.0.23
...
```

## Continuous Integration

To add these tests to CI/CD:

```yaml
# Example GitHub Actions workflow
name: VPN Multi-Account Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker
        uses: docker/setup-buildx-action@v2

      - name: Run Docker-based tests
        run: ./scripts/test-docker-multi-account.sh
```

## Manual Testing

### Test Scenario 1: Basic Account Switching

```bash
# Login as Alice
curl -X POST http://localhost:8082/api/auth/request-code \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com"}'

# Get code from DB and verify
# ... (see test scripts for full flow)

# Login as Bob (same device)
curl -X POST http://localhost:8082/api/auth/request-code \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com"}'

# Verify Alice and Bob have different subnets
```

### Test Scenario 2: Device Registration

```bash
# Generate WireGuard keypair
PRIVATE_KEY=$(wg genkey)
PUBLIC_KEY=$(echo "$PRIVATE_KEY" | wg pubkey)

# Register device
curl -X POST http://localhost:8082/api/devices \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT" \
  -d "{\"device_name\":\"laptop\",\"public_key\":\"$PUBLIC_KEY\"}"
```

## Test Coverage

- ✅ Email-based user lookup
- ✅ Subnet allocation for new users
- ✅ Subnet persistence for existing users
- ✅ Multi-account isolation
- ✅ Device registration
- ✅ WireGuard peer configuration
- ✅ IP allocation within subnets
- ✅ Account switching on same device

## Monitoring

View real-time logs during testing:

```bash
# Docker logs
docker compose -f docker-compose.test.yml logs -f

# Production logs
ssh root@178.156.133.88 'journalctl -u roamie -f'
```

## Cleanup

### Docker Environment

```bash
# Stop and remove all test containers
docker compose -f docker-compose.test.yml down -v

# Remove test images
docker rmi roamie-test-server
```

### Production Test Data

Test accounts are automatically cleaned up, but if needed:

```bash
ssh root@178.156.133.88 "docker exec roamie-postgres psql -U roamie -d roamie_vpn -c \"DELETE FROM users WHERE email LIKE '%test%@example.com';\""
```

## Further Reading

- [CIDR Notation Explained](../CLAUDE.md#cidr-notation)
- [Architecture Overview](../CLAUDE.md#architecture)
- [Network Isolation](../CLAUDE.md#network-isolation)
