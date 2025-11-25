#!/bin/bash

set -e

# Get version from argument or git tag
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo "dev")}"
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building Roamie Client ${VERSION}"
echo "  Commit: ${GIT_COMMIT}"
echo "  Time:   ${BUILD_TIME}"
echo ""

# Build ldflags
LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X github.com/kamikazebr/roamie-desktop/pkg/version.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X github.com/kamikazebr/roamie-desktop/pkg/version.GitCommit=${GIT_COMMIT}"
LDFLAGS="${LDFLAGS} -X github.com/kamikazebr/roamie-desktop/pkg/version.BuildTime=${BUILD_TIME}"

# Create dist directory
DIST_DIR="dist"
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

# Platforms to build
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    OUTPUT_NAME="roamie-${GOOS}-${GOARCH}"
    if [ "${GOOS}" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    echo "Building ${OUTPUT_NAME}..."

    CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build \
        -ldflags "${LDFLAGS}" \
        -o "${DIST_DIR}/${OUTPUT_NAME}" \
        ./cmd/client
done

echo ""
echo "Build complete! Binaries in ${DIST_DIR}/"
ls -lh "${DIST_DIR}/"

# Create checksums
echo ""
echo "Creating checksums..."
cd "${DIST_DIR}"
sha256sum roamie-* > checksums.txt
cat checksums.txt
