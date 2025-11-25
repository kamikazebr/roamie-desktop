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

## License

MIT

## Author

[@kamikazebr](https://github.com/kamikazebr)
