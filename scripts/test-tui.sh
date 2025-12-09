#!/bin/bash
# Quick TUI test - runs "no-sshd" scenario by default
# For all scenarios use: ./scripts/test-tui-all.sh

exec ./scripts/test-tui-all.sh "${1:-no-sshd}"
