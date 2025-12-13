# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Roamie VPN is a WireGuard-based VPN system with email authentication and multi-device support. Written in Go, it provides both a server component (HTTP API + WireGuard management) and a CLI client for end users.

### Key Features
- **Multiple Authentication Methods**:
  - Email-based with 6-digit codes (via Resend API)
  - QR code-based device authorization (no password needed)
  - Firebase authentication (for mobile apps)
  - Biometric authentication support (fingerprint via PAM)
- **Multi-device support**: Up to 5 devices per user with automatic registration
- **Network isolation**: Each user gets a dedicated /29 subnet (6 usable IPs)
- **Automatic network conflict detection**: Scans Docker networks and system routes
- **SSH Reverse Tunnel**: Access devices behind NAT without VPN (port 2222, allocates ports 10000-20000)
- **Device heartbeat system**: Real-time online/offline status tracking
- **REST API** for device management
- **CLI client** for connecting/managing VPN connections and SSH tunnels

## Build & Development Commands

### Quick Start (Docker - Recommended for Development)
```bash
# Setup PostgreSQL in Docker + run migrations
./scripts/docker-dev.sh setup

# Configure Resend API key
nano .env  # Edit RESEND_API_KEY

# Build both server and client
./scripts/build.sh

# Run server (without WireGuard for API-only testing)
./roamie-server
```

### Database Management
```bash
# Docker-based (recommended for dev)
./scripts/docker-dev.sh start    # Start PostgreSQL container
./scripts/docker-dev.sh stop     # Stop container
./scripts/docker-dev.sh logs     # View PostgreSQL logs
./scripts/docker-dev.sh shell    # Open psql shell
./scripts/docker-dev.sh status   # Check container status

# Run migrations (works with both Docker and local PostgreSQL)
./scripts/migrate.sh

# Clean Docker setup (WARNING: destroys data)
./scripts/docker-clean.sh

# Production Database Management
# Production user: roamie | Development user: wireguard (see .env.example)
# Clear all users on production server:
ssh root@178.156.133.88 "docker exec -i roamie-postgres psql -U roamie -d roamie_vpn -c \"TRUNCATE users, devices, auth_codes, device_auth_challenges, refresh_tokens, biometric_auth_requests CASCADE;\""
```

### Building
```bash
# Build both server and client binaries
./scripts/build.sh
# Output: ./roamie-server and ./roamie

# Build individually
go build -o roamie-server ./cmd/server
go build -o roamie ./cmd/client
```

### Running
```bash
# Development (API only, no WireGuard)
./roamie-server

# Production (requires WireGuard setup and root)
sudo ./scripts/setup-server.sh  # One-time WireGuard setup
sudo ./roamie-server                # Run with WireGuard enabled
```

### Testing
```bash
# Health check
curl http://localhost:8080/health

# Request auth code
curl -X POST http://localhost:8080/api/auth/request-code \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}'

# Network conflict scan
curl http://localhost:8080/api/admin/network/scan

# E2E Tests (Docker-based)
./scripts/test-multi-account-isolation.sh  # Critical: tests subnet isolation per email
./scripts/test-e2e-real-client.sh          # Full VPN connectivity test
./scripts/test-device-registration.sh      # Device lifecycle test
./scripts/test-device-auth.sh              # QR code auth flow test
```

## Architecture

### High-Level Structure
```
┌─────────────────┐         ┌──────────────────────────┐
│   roamie CLI    │  HTTPS  │    roamie-server         │
│   (Go/Cobra)    │◄───────►│  ┌─────────────────────┐ │
│                 │         │  │ HTTP API (Chi)      │ │
│  SSH Client     │  SSH    │  │ WireGuard Manager   │ │
│  (port 22)      │◄───────►│  │ SSH Tunnel Server   │ │
└─────────────────┘  :2222  │  │ (port 2222)         │ │
                            │  └─────────────────────┘ │
                            └──────────────────────────┘
                                     │
                            ┌────────┴─────────┬────────────┐
                       ┌────▼────┐    ┌───────▼────┐  ┌────▼────┐
                       │PostgreSQL│    │ Resend API │  │Firebase │
                       │  (sqlx)  │    │   (Email)  │  │ (Auth)  │
                       └──────────┘    └────────────┘  └─────────┘
```

### Code Organization
- `cmd/server/` - Server entry point, initializes all components
- `cmd/client/` - CLI client entry point, uses Cobra for commands
- `internal/server/api/` - HTTP handlers (Chi router)
  - `auth.go` - Email/Firebase authentication
  - `device_auth.go` - QR code device authorization
  - `biometric_auth.go` - Biometric authentication
  - `tunnel.go` - SSH tunnel management
  - `devices.go` - Device CRUD operations
- `internal/server/services/` - Business logic layer
  - `auth_service.go` - Email code generation/verification + Firebase
  - `device_service.go` - Device registration/management
  - `device_auth_service.go` - QR code challenge/response
  - `subnet_pool.go` - Subnet allocation with conflict detection
  - `network_scanner.go` - Docker/system network scanner
  - `email_service.go` - Resend integration
  - `ssh_service.go` - SSH key management via Firestore
  - `tunnel_port_pool.go` - Port allocation (10000-20000) for SSH tunnels
- `internal/server/storage/` - Repository pattern for database access
  - Uses sqlx for PostgreSQL
  - Prepared statements for SQL injection protection
- `internal/server/wireguard/` - WireGuard peer management
  - `manager.go` - Interface control via `wgctrl`
  - `backup.go` - Automatic config backup system
  - `firewall.go` - IPTables rules for user isolation
- `internal/server/tunnel/` - SSH reverse tunnel server (port 2222)
- `internal/client/` - Client-side logic
  - `auth/` - Authentication flows
  - `tunnel/` - SSH tunnel client
  - `daemon/` - Background token refresh
- `pkg/models/` - Shared data structures
- `pkg/utils/` - JWT, crypto, validation utilities
- `migrations/` - SQL schema migrations (12 files, embedded in binary)

### Database Schema (12 migrations)
The system uses 12 migrations (embedded in binary via `//go:embed`):
1. **users** - User accounts with allocated subnet (email = unique identifier for subnet allocation)
2. **auth_codes** - Temporary 6-digit codes (5min expiration)
3. **devices** - WireGuard devices (public_key, assigned_ip, username, last_seen, tunnel_port)
4. **network_conflicts** - Detected CIDR conflicts from Docker/system
5. **biometric_auth_requests** - Biometric authentication support
6. **device_auth_challenges** - QR code challenges for device authorization
7. **refresh_tokens** - Long-lived tokens for token refresh
8. **Migration 010 (Critical)** - Added heartbeat system (`last_seen`), SSH tunnel infrastructure (`tunnel_port`, `ssh_public_key`), and hardware_id extraction for device deduplication

**Important**: User lookup for subnet allocation is done by **email** (not Firebase UID) to ensure each email account gets a unique subnet, even when using the same physical device. See "Multi-Account Isolation" below.

## Network Architecture

### Subnet Allocation
- Base network: `10.100.0.0/16` (supports 8,192 users)
- Per-user subnet: `/29` (6 usable IPs for devices)
- Example allocation:
  - User 1: `10.100.0.0/29` (devices get 10.100.0.2 - 10.100.0.6)
  - User 2: `10.100.0.8/29` (devices get 10.100.0.10 - 10.100.0.14)
- Fallback networks: `10.200.0.0/16`, `10.150.0.0/16` (if base network conflicts)

### Conflict Detection
The `network_scanner.go` service scans:
- Docker networks (via `docker network ls` + `docker network inspect`)
- System routes (via `ip route`)
- Stores conflicts in database to avoid allocation

### SSH Reverse Tunnel Architecture
Allows remote access to devices behind NAT without VPN:
- **Server**: Listens on port 2222 for SSH connections
- **Port Allocation**: Each device gets a unique port (10000-20000 range)
- **Authentication**: Public key based (keys stored in Firestore)
- **Usage**: `ssh user@server -p <allocated_port>` tunnels directly to device
- **Client Commands**:
  - `roamie tunnel register` - Allocate port and start tunnel
  - `roamie tunnel status` - Check tunnel status
  - `roamie tunnel stop` - Stop tunnel
  - `roamie ssh sync` - Sync SSH public key to Firestore

## Important Technical Details

### Environment Variables
Required variables (see `.env.example`):
- `DATABASE_URL` - PostgreSQL connection string
- `RESEND_API_KEY` - Email service API key
- `JWT_SECRET` - Secret for JWT tokens
- `WG_SERVER_PUBLIC_ENDPOINT` - Public IP:port for WireGuard
- `WG_BASE_NETWORK` - Base CIDR (default: 10.100.0.0/16)
- `WG_SUBNET_SIZE` - Subnet size per user (default: 29)
- `MAX_DEVICES_PER_USER` - Device limit (default: 5)
- `FIREBASE_CREDENTIALS_PATH` - Path to Firebase service account JSON (optional, for mobile app)
- `AUTO_REGISTER_DEVICES` - Enable QR code auto-registration (default: true)
- `SSH_TUNNEL_PORT` - SSH tunnel server port (default: 2222)
- `TUNNEL_PORT_RANGE_START` - Start of tunnel port range (default: 10000)
- `TUNNEL_PORT_RANGE_END` - End of tunnel port range (default: 20000)
- `TUNNEL_SERVER_HOST` - (Optional) Override hostname for SSH tunnels. If not set, falls back to extracting host from `WG_SERVER_PUBLIC_ENDPOINT`. Only needed if SSH tunnel server is on a different host than WireGuard.

### Authentication Flows

#### 1. Email Authentication (Original)
1. User requests code: `POST /api/auth/request-code` → sends email with 6-digit code
2. User verifies code: `POST /api/auth/verify-code` → returns JWT token
3. JWT expires in 7 days (configurable via `JWT_EXPIRATION`)
4. Codes expire in 5 minutes (`AUTH_CODE_EXPIRATION`)

#### 2. Firebase Authentication (Mobile App)
1. Flutter app authenticates with Firebase
2. App sends Firebase ID token: `POST /api/auth/firebase-login`
3. Server verifies token and returns JWT
4. **Critical**: Server uses email from Firebase token (not Firebase UID) for user lookup to ensure proper subnet isolation

#### 3. QR Code Device Authorization (New - Password-less)
1. Device requests authorization: `POST /api/auth/device-request` → returns challenge_id and QR code data
2. Mobile app scans QR code showing challenge_id and device info
3. User approves device in mobile app: `PATCH /api/auth/device-challenges/{id}/approve`
4. Device polls: `GET /api/auth/device-request/{id}` until approved
5. On approval, device receives JWT + refresh token
6. Device auto-registers if `AUTO_REGISTER_DEVICES=true`

#### 4. Biometric Authentication (Optional)
- Install PAM module: `./scripts/install-biometric-auth.sh`
- Allows fingerprint authentication instead of password
- Integrates with sudo for system-level auth

### Device Identification and Deduplication
Devices are identified by format: `os-username-hardwareid` (e.g., `android-john-a1b2c3d4`)
- **os_type**: android, ios, linux, macos, windows
- **hardware_id**: 8-character hex identifier
- **username**: system username (used for SSH)
- **Unique Constraint**: Same user + same hardware_id = same device (prevents duplicates on re-registration)

### Device Heartbeat System
- Devices send periodic heartbeats to update `last_seen` timestamp
- Device considered "online" if `last_seen < 60 seconds ago`
- Enables real-time status display in mobile app
- Endpoint: `POST /api/devices/{id}/heartbeat`

### WireGuard Integration
- Server generates keypair on first run (stored in `/etc/wireguard/`)
- Interface name: `wg0` (configurable via `WG_INTERFACE`)
- Port: `51820` (configurable via `WG_PORT`)
- **Automatic Backup**: If existing wg0 configuration detected, server automatically backs up to `/etc/wireguard/backups/roamie-backup-TIMESTAMP/`
  - Backs up config files, keys, and peer information
  - Creates restore script for easy rollback
  - See `RESTORE.txt` in backup directory for instructions
- Each device registration:
  1. Generates client keypair
  2. Allocates IP from user's subnet
  3. Adds peer to wg0 interface
  4. Returns WireGuard config file to client

### Client Storage
- Credentials stored in `~/.config/roamie-vpn/credentials.json`
- Contains JWT token for API authentication
- Private keys stored in `~/.config/roamie-vpn/keys/`
- WireGuard config written to `~/.config/roamie-vpn/wg0.conf`

### Multi-Account Isolation Bug Fix (Critical)
**Problem**: When users switched accounts on the same device, both accounts shared the same subnet because the server used Firebase UID (device-based) for user lookup.

**Solution** (in `auth_service.go`):
```go
// WRONG (caused subnet sharing):
user, err := s.userRepo.GetByFirebaseUID(ctx, firebaseUID)

// CORRECT (ensures per-email isolation):
user, err := s.userRepo.GetByEmail(ctx, email)
```

This ensures each email account gets a unique subnet, even when using the same physical device. The `test-multi-account-isolation.sh` script verifies this behavior.

## Client Commands

### Authentication
- `roamie auth login` - Start email authentication flow
- `roamie auth device-login` - Start QR code device authorization (password-less)
- `roamie auth logout` - Clear credentials
- `roamie auth status` - Check authentication status
- `roamie auth refresh` - Refresh JWT token

### Device Management
- `roamie connect` - Connect to VPN (requires sudo)
- `roamie disconnect` - Disconnect from VPN (requires sudo)
- `roamie status` - Show connection status (requires sudo)
- `roamie devices list` - List registered devices
- `roamie devices remove <device-id>` - Remove device

### SSH Tunnel
- `roamie tunnel register` - Allocate port and start reverse SSH tunnel
- `roamie tunnel status` - Check tunnel status and allocated port
- `roamie tunnel stop` - Stop tunnel
- `roamie ssh sync` - Sync SSH public key to Firestore

### Background Services
- `roamie daemon start` - Start token refresh daemon
- `roamie daemon stop` - Stop daemon
- `roamie daemon status` - Check daemon status

### Maintenance
- `roamie upgrade` (or `roamie update`) - Upgrade to latest version
- `roamie auto-upgrade [on|off|status]` - Control automatic background upgrades (enabled by default)
- `roamie doctor` - Run system health diagnostics (auth, server, WireGuard, daemon status)

### macOS Client Installation

When running `sudo roamie auth login` on macOS:

| Scenario | Behavior |
|----------|----------|
| WireGuard installed | Proceeds normally |
| Brew installed, WireGuard missing | Auto-runs `brew install wireguard-tools` |
| Brew not installed | Shows instructions to install brew + wireguard, then exit |

**Note**: `sudo` is required because `wg-quick up` needs root to create `utun` network interface.

## API Endpoints

### Public
- `POST /api/auth/request-code` - Request auth code via email
- `POST /api/auth/verify-code` - Verify code, get JWT token
- `POST /api/auth/firebase-login` - Login with Firebase ID token
- `POST /api/auth/device-request` - Request QR code device authorization
- `GET /api/auth/device-request/:id` - Poll for device authorization approval
- `GET /api/auth/device-challenges` - List pending device challenges (for mobile app)
- `PATCH /api/auth/device-challenges/:id/approve` - Approve device authorization

### Authenticated (requires JWT in Authorization header)
- `GET /api/devices` - List user's devices
- `POST /api/devices` - Register new device
- `DELETE /api/devices/:id` - Remove device
- `GET /api/devices/:id/config` - Get WireGuard config
- `POST /api/devices/:id/heartbeat` - Update device last_seen timestamp
- `POST /api/devices/:id/tunnel/register` - Allocate SSH tunnel port
- `GET /api/devices/:id/tunnel/status` - Get tunnel status

### Admin (no auth currently)
- `GET /api/admin/network/scan` - Trigger network conflict scan
- `GET /api/admin/network/conflicts` - List detected conflicts
- `POST /api/admin/network/conflicts` - Manually add conflict

## Testing

### E2E Test Environment
The project includes comprehensive Docker-based E2E tests using `docker-compose.e2e.yml`:
- **e2e-postgres**: PostgreSQL 15 (port 5436)
- **e2e-server**: VPN server with WireGuard (privileged container)
- **alice-device-1/2**: Client containers simulating Alice's devices
- **bob-device-1/2**: Client containers simulating Bob's devices

### Test Scripts
- `./scripts/test-multi-account-isolation.sh` - **Critical**: Verifies that different email accounts get different subnets even on the same device (tests the Firebase UID vs email bug fix)
- `./scripts/test-e2e-real-client.sh` - Full VPN connectivity test with multiple users
- `./scripts/test-device-registration.sh` - Device lifecycle (register, list, delete)
- `./scripts/test-device-auth.sh` - QR code device authorization flow

### GitHub Actions macOS CI
Workflow: `.github/workflows/macos-client.yml`

**What works on GitHub Actions macOS runner:**
- `brew install wireguard-tools` - installs wg, wg-quick, wireguard-go
- `sudo` - available without password
- `wg-quick up` - creates `utun` interface successfully
- `wg show` - works after interface is up
- Multi-arch build (amd64 + arm64)

**What doesn't work:**
- Real VPN handshake (needs server connectivity)
- Manual `ifconfig utun create` (returns SIOCIFCREATE2 error)

## Common Development Tasks

### Adding a new API endpoint
1. Add handler in `internal/server/api/` (e.g., `devices.go`)
2. Register route in `cmd/server/main.go` router setup
3. Add business logic in `internal/server/services/`
4. Add database methods in `internal/server/storage/`
5. Update `pkg/models/` if new data structures needed

### Adding a new CLI command
1. Add command in `cmd/client/main.go` using Cobra
2. Implement logic in `internal/client/` subdirectories
3. Use `internal/client/api/client.go` for API calls

### Modifying database schema
1. Create new migration file: `migrations/00X_description.sql`
2. Run: `./scripts/migrate.sh`
3. Update repository methods in `internal/server/storage/`
4. Update models in `pkg/models/`

### Testing WireGuard changes
- Server must run with `sudo` to modify wg0 interface
- Use `sudo wg show` to inspect current peers
- Use `sudo wg-quick down wg0` / `up wg0` to restart interface
- Check logs: `sudo journalctl -u wg-quick@wg0 -f`

## Dependencies

### Key Go Libraries
- `github.com/go-chi/chi/v5` - HTTP router with middleware
- `github.com/jmoiron/sqlx` - PostgreSQL with struct scanning
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `golang.zx2c4.com/wireguard/wgctrl` - WireGuard control
- `github.com/resendlabs/resend-go` - Email service
- `github.com/spf13/cobra` - CLI framework
- `github.com/joho/godotenv` - .env file loading
- `firebase.google.com/go/v4` - Firebase Admin SDK
- `golang.org/x/crypto/ssh` - SSH tunnel implementation

### System Dependencies
- PostgreSQL 15+ (or Docker)
- WireGuard kernel module (for production)
- Go 1.23+
- Docker (for development and E2E tests)

## Documentation

### Additional Documentation Files
- **CHANGELOG.md** - Release history following Keep a Changelog format
- **FLUTTER_INTEGRATION.md** - Complete Flutter/mobile app integration guide with API endpoints and Dart examples
- **FLUTTER_MIGRATION_GUIDE.md** - Migration guide from email auth to QR code device authorization
- **QUICKSTART.md** - Fast setup guide for new developers
- **DOCKER.md** - Docker usage and configuration details
- **TESTING.md** - Comprehensive test infrastructure documentation
- **README.md** - Portuguese language project overview

## Troubleshooting

### "DATABASE_URL not set"
Ensure `.env` file exists (copy from `.env.example`) and contains valid `DATABASE_URL`.

**Note on Database User**:
- Development (Docker): User is `wireguard` (see `docker-compose.yml`)
- Production: User is `roamie` (see production server setup)

### "Failed to initialize WireGuard manager"
- For development/testing API only: ignore this (server still runs)
- For production: run `sudo ./scripts/setup-server.sh` to configure WireGuard

### "Permission denied" when connecting client
Client connect/disconnect requires root: `sudo ./roamie connect`

### "Failed to get status" when running roamie status
The status command needs to read WireGuard interface information. Run with sudo:
```bash
sudo ./roamie status
```
Note: If you have an existing wg0 interface configured outside of Roamie VPN, the client will show that interface's status.

### Database migration errors
Check PostgreSQL is running:
- Docker: `./scripts/docker-dev.sh status`
- Local: `sudo systemctl status postgresql`

### Port 5432 already in use (Docker)
Local PostgreSQL may be running. Either:
- Stop it: `sudo systemctl stop postgresql`
- Change Docker port in `docker-compose.yml` to 5433

## Security Considerations

- All SQL queries use prepared statements (sqlx handles this)
- JWT tokens are signed with `JWT_SECRET`
- Private keys never leave client devices
- Auth codes are single-use and expire after 5 minutes
- Network isolation prevents inter-user device communication
- HTTPS recommended for production (configure `ENABLE_TLS=true`)

## Deployment

### Release Tags (Client vs Server)
Client and server have **separate versioning** with different git tags:

| Component | Tag Pattern | Workflow | Output |
|-----------|-------------|----------|--------|
| **Client** | `v*` (e.g., `v0.0.1`, `v1.0.0`) | `release.yml` | Multi-platform binaries on GitHub Releases |
| **Server** | `server-v*` (e.g., `server-v0.0.1`) | `docker.yml` | Docker image on `ghcr.io/kamikazebr/roamie-desktop/roamie-server` |

**Creating a release (recommended - using /changelog):**
```bash
# In Claude Code, run:
/changelog
```

The `/changelog` command will:
1. Fetch commits since last tag
2. Generate detailed, user-friendly release notes
3. Ask to add to `CHANGELOG.md`
4. Ask to create tag and push (triggers GitHub Action)

The release workflow (`.github/workflows/release.yml`) automatically extracts notes from `CHANGELOG.md` for the GitHub release.

**Creating a release (manual):**
```bash
# Client release (triggers multi-platform build)
git tag v0.0.3 && git push origin v0.0.3

# Server release (triggers Docker image build)
git tag server-v0.0.3 && git push origin server-v0.0.3
```

### Server Deployment (Manual)
Deploy to production server using the deploy script:
```bash
# Build and deploy
./scripts/build.sh && ./scripts/deploy.sh
```

The script:
1. Copies `roamie-server` binary and `.env.production` to server
2. Restarts systemd service
3. Runs health check

### Production Server
- **IP**: `178.156.133.88`
- **API Port**: `8081` (not 8080)
- **SSH**: `root@178.156.133.88`
- **DB User**: `roamie` (development uses `wireguard`)
- **Service**: `systemctl status roamie`
- **Logs**: `journalctl -u roamie -f`

### Pre-Deployment Checklist
1. **Always build first**: `./scripts/build.sh`
2. **Run all tests**: `go test ./...`
3. **Run E2E tests**: `./scripts/test-e2e-real-client.sh`
4. **Test the actual fix**, not assumptions

## Development Best Practices

### Database Operations
- **Check the SQL**: When modifying repository methods, verify the actual INSERT/UPDATE statement includes all needed columns
- **Setting a struct field ≠ using it in database**: The SQL statement determines what gets saved, not the Go struct

### Debugging "Not Found" Errors
1. Check what ID/key is in the client config
2. Check what ID/key is in the database
3. Compare - find where the mismatch occurs
4. Trace the complete data flow from client → server → database → response → client
