#!/bin/bash
# Test TUI in all scenarios using pre-built Dockerfiles
#
# Usage:
#   ./scripts/test-tui-all.sh [scenario]
#
# Scenarios:
#   no-sshd      - SSH not installed (should offer to install)
#   stopped      - SSH installed but not running (should offer to start)
#   running      - SSH already running (should pass through)
#   all          - Run all scenarios (default)

set -e

# Detect OS - Docker+systemd tests only work on Linux
if [ "$(uname -s)" = "Darwin" ]; then
    echo ""
    echo "⚠️  macOS detected"
    echo ""
    echo "Docker-based tests require Linux with systemd."
    echo "These tests run automatically in GitHub Actions CI."
    echo ""
    echo "For local macOS testing, run:"
    echo "  go test -v ./internal/client/sshd/"
    echo "  go test -v ./internal/client/wireguard/"
    echo ""
    exit 0
fi

SCENARIO="${1:-all}"
DOCKER_DIR="docker/test-tui"

# Always rebuild test binary to ensure latest code
echo "Building test-tui binary..."
go build -o test-tui ./cmd/test-tui

run_test() {
    local name="$1"
    local dockerfile="$2"
    local description="$3"

    echo ""
    echo "=============================================="
    echo "  Scenario: $name"
    echo "  $description"
    echo "=============================================="
    echo ""

    # Build image
    IMAGE_NAME="roamie-test-$name"
    echo "Building image..."
    docker build -t "$IMAGE_NAME" -f "$dockerfile" . > /dev/null 2>&1

    # Run container
    CONTAINER_ID=$(docker run -d --rm \
        --privileged \
        --cgroupns=host \
        -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
        -v "$(pwd)/test-tui:/test-tui" \
        "$IMAGE_NAME")

    echo "Container: ${CONTAINER_ID:0:12}"
    echo "Waiting for systemd..."
    sleep 3

    # Show current state
    echo ""
    echo "Current state:"
    docker exec "$CONTAINER_ID" bash -c "dpkg -l openssh-server 2>/dev/null | grep -q '^ii' && echo '  openssh-server: installed' || echo '  openssh-server: NOT installed'"
    docker exec "$CONTAINER_ID" bash -c "systemctl is-active ssh 2>/dev/null && echo '  ssh service: running' || echo '  ssh service: not running'"
    echo ""

    # Run test
    echo "Running TUI..."
    echo "----------------------------------------"
    docker exec -it "$CONTAINER_ID" /test-tui || true
    echo "----------------------------------------"

    # Cleanup
    echo "Cleaning up..."
    docker stop "$CONTAINER_ID" > /dev/null 2>&1 || true
}

case "$SCENARIO" in
    no-sshd|no)
        run_test "no-sshd" "$DOCKER_DIR/Dockerfile.no-sshd" "SSH not installed - should offer to install"
        ;;
    stopped|stop)
        run_test "sshd-stopped" "$DOCKER_DIR/Dockerfile.sshd-stopped" "SSH installed but stopped - should offer to start"
        ;;
    running|run)
        run_test "sshd-running" "$DOCKER_DIR/Dockerfile.sshd-running" "SSH already running - should pass through"
        ;;
    all|*)
        run_test "sshd-running" "$DOCKER_DIR/Dockerfile.sshd-running" "SSH already running - should pass through"
        run_test "sshd-stopped" "$DOCKER_DIR/Dockerfile.sshd-stopped" "SSH installed but stopped - should offer to start"
        run_test "no-sshd" "$DOCKER_DIR/Dockerfile.no-sshd" "SSH not installed - should offer to install"
        echo ""
        echo "=============================================="
        echo "  All scenarios completed!"
        echo "=============================================="
        ;;
esac
