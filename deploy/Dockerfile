# Roamie VPN Server - Production Dockerfile
# Multi-stage build for minimal image size

# ============================================
# Stage 1: Build the Go binary
# ============================================
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary
# CGO disabled for static binary, ldflags to strip debug info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o roamie-server \
    ./cmd/server

# ============================================
# Stage 2: Production runtime
# ============================================
FROM alpine:3.19

# Install runtime dependencies
# - wireguard-tools: wg, wg-quick commands
# - iptables: firewall rules for network isolation
# - iproute2: ip command for routing
# - ca-certificates: HTTPS connections (Resend API, Firebase)
RUN apk add --no-cache \
    wireguard-tools \
    iptables \
    iproute2 \
    ca-certificates \
    tzdata

# Create non-root user (WireGuard still needs NET_ADMIN capability)
RUN addgroup -S roamie && adduser -S roamie -G roamie

# Create required directories
RUN mkdir -p /etc/wireguard /var/log/roamie /app \
    && chown -R roamie:roamie /var/log/roamie /app

WORKDIR /app

# Copy binary from builder (migrations are embedded via go:embed)
COPY --from=builder /build/roamie-server .

# Expose ports
# 8080: HTTP API
# 51820: WireGuard UDP
# 2222: SSH Reverse Tunnel
EXPOSE 8080 51820/udp 2222

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run as root for WireGuard (requires NET_ADMIN)
# Security is handled via container capabilities, not user
USER root

# Default command
CMD ["./roamie-server"]
