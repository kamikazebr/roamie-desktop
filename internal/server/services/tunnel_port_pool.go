package services

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
)

// TunnelPortPool manages allocation of SSH reverse tunnel ports
// Similar to SubnetPool but for port numbers
type TunnelPortPool struct {
	startPort  int
	endPort    int
	deviceRepo *storage.DeviceRepository
}

// NewTunnelPortPool creates a new tunnel port pool
// Default range: 10000-20000 (10,000 available ports)
// Can be configured via environment variables:
// - TUNNEL_PORT_START
// - TUNNEL_PORT_END
func NewTunnelPortPool(deviceRepo *storage.DeviceRepository) (*TunnelPortPool, error) {
	startPort := 10000
	endPort := 20000

	// Read from environment if set
	if envStart := os.Getenv("TUNNEL_PORT_START"); envStart != "" {
		if p, err := strconv.Atoi(envStart); err == nil && p > 1024 && p < 65535 {
			startPort = p
		}
	}

	if envEnd := os.Getenv("TUNNEL_PORT_END"); envEnd != "" {
		if p, err := strconv.Atoi(envEnd); err == nil && p > startPort && p < 65535 {
			endPort = p
		}
	}

	return &TunnelPortPool{
		startPort:  startPort,
		endPort:    endPort,
		deviceRepo: deviceRepo,
	}, nil
}

// AllocatePort finds and returns the next available port in the range
// Returns error if no ports are available
func (p *TunnelPortPool) AllocatePort(ctx context.Context) (int, error) {
	// Get all currently allocated ports
	allocatedPorts, err := p.deviceRepo.GetAllTunnelPorts(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get allocated ports: %w", err)
	}

	// Create a map for O(1) lookup
	allocated := make(map[int]bool)
	for _, port := range allocatedPorts {
		allocated[port] = true
	}

	// Find first available port
	for port := p.startPort; port <= p.endPort; port++ {
		if !allocated[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d (%d total ports, all allocated)",
		p.startPort, p.endPort, p.endPort-p.startPort+1)
}

// GetAvailableCount returns the number of available ports
func (p *TunnelPortPool) GetAvailableCount(ctx context.Context) (int, error) {
	allocatedPorts, err := p.deviceRepo.GetAllTunnelPorts(ctx)
	if err != nil {
		return 0, err
	}

	totalPorts := p.endPort - p.startPort + 1
	return totalPorts - len(allocatedPorts), nil
}
