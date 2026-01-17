# Changelog

All notable changes to Roamie VPN Client will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [server-v0.0.11 / v0.0.11] - 2026-01-17

### Features

- **Remote Upgrade System**: Implements server-as-proxy remote upgrade system allowing administrators to trigger client upgrades via API
  - **Daemon polling**: Automatic check for pending upgrades every 30 seconds
  - **Startup check**: Immediate upgrade check when daemon starts
  - **Server-side API endpoints**:
    - `POST /api/devices/{id}/trigger-upgrade` - Trigger upgrade for specific device (supports optional target version)
    - `GET /api/devices/{id}/upgrade/{request_id}` - Get upgrade execution result
    - `GET /api/devices/upgrades/pending` - List pending upgrades for authenticated user's devices (daemon endpoint)
    - `POST /api/devices/upgrades/result` - Upload upgrade result to Firestore (daemon endpoint)
  - **Firestore collections**:
    - `upgrade_requests/{device}/pending/{request_id}` - Stores pending upgrade requests
    - `upgrade_results/{device}/results/{request_id}` - Stores upgrade execution results with success/failure status
  - **Automatic workflow**:
    - Download and verify checksums from GitHub releases
    - Backup current binary before replacing
    - Install new version with preserved permissions
    - Auto-restart daemon with new binary on success
    - Upload detailed results (previous version, new version, success status, error messages)
  - **Use cases**:
    - Emergency security patches without waiting 24h auto-upgrade cycle
    - Targeted device updates for testing
    - Future support for mass upgrades and version rollback
  - **Documentation**: Complete guide in `REMOTE_UPGRADE.md` with architecture diagrams, API examples, and troubleshooting
  - **Testing**: End-to-end test script `scripts/test-remote-upgrade.sh` for validation

- **Remote Diagnostics Testing**: Added comprehensive testing script and documentation
  - Test script `scripts/test-remote-diagnostics.sh` for end-to-end diagnostics flow validation
  - Documentation `TESTING_DIAGNOSTICS.md` with usage examples and troubleshooting

### Bug Fixes

- **Firebase credentials mounting**: Fixed Docker Compose DEV environment to correctly mount Firebase service account credentials
  - Path corrected from `/app/firebase-credentials.json` to `/app/config/firebase-credentials.json`
  - Ensures DiagnosticsService and remote upgrade system can initialize properly in DEV environment

## [v0.0.9] - 2025-12-18

### Bug Fixes

- **SSH tunnel auto-reconnect**: Fixed daemon not detecting broken SSH tunnel connections
  - Added health check every 30 seconds to monitor tunnel connection state
  - Automatically restarts tunnel when connection is lost (e.g., after network outage or PC restart)
  - Previously, tunnel would break silently and require manual daemon restart

## [v0.0.8] - 2025-12-13

### Features

- **Auto-upgrade system**: The daemon now automatically checks for updates every 24 hours and upgrades in the background
  - Auto-upgrade is **enabled by default** for new installations
  - First-run message informs users about the feature
  - Background update notification shown on next interactive command

- **New command: `roamie auto-upgrade`**: Control automatic background upgrades
  - `roamie auto-upgrade` or `roamie auto-upgrade status` - Show current status
  - `roamie auto-upgrade on` - Enable auto-upgrade
  - `roamie auto-upgrade off` - Disable auto-upgrade

- **New command: `roamie doctor`**: Diagnostic tool to check system health
  - Validates authentication and JWT expiration
  - Checks server connectivity
  - Verifies WireGuard installation
  - Reports daemon status
  - Shows auto-upgrade configuration
  - Checks for available updates

- **New alias: `roamie update`**: Alias for `roamie upgrade` command

### Bug Fixes

- **User service management**: Fixed systemctl commands to use `--user` flag for proper user-level service management
  - Added `runSystemctlUser()` helper that handles SUDO_USER correctly
  - Fixed `isServiceRunning()` to check user service status

## [v0.0.7] - 2025-12-13

- Initial public release with core VPN functionality
- Email-based authentication
- QR code device authorization
- Multi-device support (up to 5 devices)
- WireGuard VPN connection management
- SSH reverse tunnel support
- Background daemon for token refresh

## [v0.0.6] - 2025-12-10

- Pre-release improvements

## [v0.0.5] - 2025-12-10

- Pre-release improvements

## [v0.0.4] - 2025-12-09

- Pre-release improvements
