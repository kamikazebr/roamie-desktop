#!/bin/bash

set -e

echo "Building Roamie VPN..."

# Get build information
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo "dev")}"
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_DIRTY="false"

# Check if working directory has uncommitted changes
if ! git diff-index --quiet HEAD -- 2>/dev/null; then
    GIT_DIRTY="true"
fi

# Build ldflags
LDFLAGS="-X github.com/kamikazebr/roamie-desktop/pkg/version.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X github.com/kamikazebr/roamie-desktop/pkg/version.GitCommit=${GIT_COMMIT}"
LDFLAGS="${LDFLAGS} -X github.com/kamikazebr/roamie-desktop/pkg/version.BuildTime=${BUILD_TIME}"
LDFLAGS="${LDFLAGS} -X github.com/kamikazebr/roamie-desktop/pkg/version.GitDirty=${GIT_DIRTY}"

echo "Build info:"
echo "  Version: ${VERSION}"
echo "  Commit:  ${GIT_COMMIT}"
echo "  Time:    ${BUILD_TIME}"
echo "  Dirty:   ${GIT_DIRTY}"
echo ""

# Build server
echo "Building server..."
go build -ldflags "${LDFLAGS}" -o roamie-server ./cmd/server

# Build client
echo "Building client..."
go build -ldflags "${LDFLAGS}" -o roamie ./cmd/client

echo ""
echo "Build complete!"
echo "  Server binary: ./roamie-server"
echo "  Client binary: ./roamie"
echo ""
echo "To install system-wide:"
echo "  sudo cp roamie-server /usr/local/bin/"
echo "  sudo cp roamie /usr/local/bin/"
