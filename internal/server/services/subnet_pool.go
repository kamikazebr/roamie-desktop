package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
)

type SubnetPool struct {
	baseNetwork      *net.IPNet
	subnetSize       int
	fallbackNetworks []*net.IPNet
	userRepo         *storage.UserRepository
	conflictRepo     *storage.ConflictRepository
}

func NewSubnetPool(userRepo *storage.UserRepository, conflictRepo *storage.ConflictRepository) (*SubnetPool, error) {
	// Get base network from env
	baseNetworkStr := os.Getenv("WG_BASE_NETWORK")
	if baseNetworkStr == "" {
		baseNetworkStr = "10.100.0.0/16"
	}

	_, baseNetwork, err := net.ParseCIDR(baseNetworkStr)
	if err != nil {
		return nil, fmt.Errorf("invalid WG_BASE_NETWORK: %w", err)
	}

	// Get subnet size
	subnetSize := 29
	if envSize := os.Getenv("WG_SUBNET_SIZE"); envSize != "" {
		if size, err := strconv.Atoi(envSize); err == nil {
			subnetSize = size
		}
	}

	// Parse fallback networks
	var fallbackNetworks []*net.IPNet
	if envFallbacks := os.Getenv("WG_FALLBACK_NETWORKS"); envFallbacks != "" {
		for _, fbStr := range strings.Split(envFallbacks, ",") {
			_, fbNet, err := net.ParseCIDR(strings.TrimSpace(fbStr))
			if err == nil {
				fallbackNetworks = append(fallbackNetworks, fbNet)
			}
		}
	}

	return &SubnetPool{
		baseNetwork:      baseNetwork,
		subnetSize:       subnetSize,
		fallbackNetworks: fallbackNetworks,
		userRepo:         userRepo,
		conflictRepo:     conflictRepo,
	}, nil
}

func (p *SubnetPool) AllocateSubnet(ctx context.Context) (string, error) {
	// Get all existing subnets
	existingSubnets, err := p.userRepo.GetAllSubnets(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get existing subnets: %w", err)
	}

	// Get network conflicts
	conflicts, err := p.conflictRepo.GetAllCIDRs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get conflicts: %w", err)
	}

	// Try base network first
	subnet, err := p.findAvailableSubnet(p.baseNetwork, existingSubnets, conflicts)
	if err == nil {
		return subnet, nil
	}

	// Try fallback networks
	for _, fallback := range p.fallbackNetworks {
		subnet, err := p.findAvailableSubnet(fallback, existingSubnets, conflicts)
		if err == nil {
			return subnet, nil
		}
	}

	return "", fmt.Errorf("no available subnets in any configured network range")
}

func (p *SubnetPool) findAvailableSubnet(baseNet *net.IPNet, existing, conflicts []string) (string, error) {
	baseIP := baseNet.IP.To4()
	if baseIP == nil {
		return "", fmt.Errorf("IPv6 not supported")
	}

	baseMask, _ := baseNet.Mask.Size()
	subnetIncrement := 1 << (32 - p.subnetSize)

	// Calculate maximum number of subnets
	maxSubnets := 1 << (p.subnetSize - baseMask)

	for i := 0; i < maxSubnets; i++ {
		// Calculate next subnet
		offset := i * subnetIncrement
		candidateIP := make(net.IP, 4)
		copy(candidateIP, baseIP)

		// Add offset to IP
		ipInt := ipToInt(candidateIP)
		ipInt += uint32(offset)
		candidateIP = intToIP(ipInt)

		candidateSubnet := fmt.Sprintf("%s/%d", candidateIP.String(), p.subnetSize)

		// Check if already allocated
		if contains(existing, candidateSubnet) {
			continue
		}

		// Check for conflicts
		if p.hasConflict(candidateSubnet, conflicts) {
			continue
		}

		return candidateSubnet, nil
	}

	return "", fmt.Errorf("no available subnet in range %s", baseNet.String())
}

func (p *SubnetPool) hasConflict(candidateSubnet string, conflicts []string) bool {
	_, candidateNet, err := net.ParseCIDR(candidateSubnet)
	if err != nil {
		return true
	}

	for _, conflictCIDR := range conflicts {
		_, conflictNet, err := net.ParseCIDR(conflictCIDR)
		if err != nil {
			continue
		}

		// Check if networks overlap
		if subnetsOverlap(candidateNet, conflictNet) {
			return true
		}
	}

	return false
}

func (p *SubnetPool) GetNextAvailableIP(ctx context.Context, userSubnet string, existingIPs []string) (string, error) {
	_, subnet, err := net.ParseCIDR(userSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet: %w", err)
	}

	ip := subnet.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("IPv6 not supported")
	}

	// Start from .2 (skip .0 network address and .1 gateway)
	startIP := make(net.IP, 4)
	copy(startIP, ip)
	ipInt := ipToInt(startIP) + 2

	// Calculate last usable IP (broadcast - 1)
	mask := subnet.Mask
	ones, bits := mask.Size()
	hostBits := bits - ones
	maxHosts := (1 << hostBits) - 2 // -2 for network and broadcast

	for i := 0; i < maxHosts; i++ {
		candidateIP := intToIP(ipInt + uint32(i))
		candidateStr := candidateIP.String()

		if !contains(existingIPs, candidateStr) {
			return candidateStr, nil
		}
	}

	return "", fmt.Errorf("no available IPs in subnet %s", userSubnet)
}

// Helper functions
func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func intToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func subnetsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
