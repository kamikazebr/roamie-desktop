# Changelog

All notable changes to Roamie VPN Client will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
