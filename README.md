# Roamie Desktop

**Break Free. Code Anywhere.**

CLI client for [Roamie](https://roamie.dev) - the remote development platform that lets you control Claude Code from your phone.

## Features

- Secure device authentication via QR code
- WireGuard tunnel management
- SSH key sync across devices
- Automatic connection handling

## Installation

```bash
# Download and install
curl -fsSL https://roamie.dev/install.sh | bash

# Or build from source
go build -o roamie ./cmd/client
sudo cp roamie /usr/local/bin/
```

## Quick Start

```bash
# Login (scan QR code with Roamie mobile app)
roamie auth login

# Connect to your network
sudo roamie connect

# Check status
roamie auth status

# Disconnect
sudo roamie disconnect
```

## Requirements

- Linux, macOS, or Windows
- WireGuard installed

## Troubleshooting

### Debug Logging

Enable detailed debug logs to troubleshoot connection issues:

**Client Side:**
```bash
# Enable debug mode for tunnel commands
ROAMIE_DEBUG=1 roamie tunnel connect

# Or export for persistent session
export ROAMIE_DEBUG=1
roamie tunnel connect
```

**Server Side:**
```bash
# Edit systemd service
sudo systemctl edit roamie-server

# Add this line under [Service]:
Environment="ROAMIE_DEBUG=1"

# Restart service
sudo systemctl restart roamie-server

# View logs with debug output
journalctl -u roamie-server -f | grep DEBUG
```

Debug logs include:
- SSH key loading and generation
- Connection attempts with retry delays
- Authentication details with key fingerprints
- Port forwarding setup
- Data transfer byte counts
- Keepalive status

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues and log patterns.

## License

MIT

## Author

[@kamikazebr](https://github.com/kamikazebr)
