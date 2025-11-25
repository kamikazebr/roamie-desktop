package services

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
)

type NetworkScanner struct {
	conflictRepo *storage.ConflictRepository
}

func NewNetworkScanner(conflictRepo *storage.ConflictRepository) *NetworkScanner {
	return &NetworkScanner{
		conflictRepo: conflictRepo,
	}
}

func (s *NetworkScanner) ScanNetworks(ctx context.Context) ([]models.NetworkConflict, error) {
	var conflicts []models.NetworkConflict

	// Scan Docker networks
	dockerNetworks, err := s.scanDockerNetworks()
	if err == nil {
		conflicts = append(conflicts, dockerNetworks...)
	}

	// Scan system routes
	systemRoutes, err := s.scanSystemRoutes()
	if err == nil {
		conflicts = append(conflicts, systemRoutes...)
	}

	// Save to database
	for _, conflict := range conflicts {
		exists, err := s.conflictRepo.Exists(ctx, conflict.CIDR)
		if err != nil {
			continue
		}
		if !exists {
			s.conflictRepo.Create(ctx, &conflict)
		}
	}

	return conflicts, nil
}

func (s *NetworkScanner) scanDockerNetworks() ([]models.NetworkConflict, error) {
	var conflicts []models.NetworkConflict

	// Try to list Docker networks
	cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err // Docker not available or not running
	}

	networks := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, network := range networks {
		if network == "" {
			continue
		}

		// Inspect each network
		inspectCmd := exec.Command("docker", "network", "inspect", network, "--format", "{{range .IPAM.Config}}{{.Subnet}}{{end}}")
		inspectOutput, err := inspectCmd.Output()
		if err != nil {
			continue
		}

		subnet := strings.TrimSpace(string(inspectOutput))
		if subnet == "" || subnet == "<no value>" {
			continue
		}

		// Validate CIDR
		_, _, err = net.ParseCIDR(subnet)
		if err != nil {
			continue
		}

		conflicts = append(conflicts, models.NetworkConflict{
			CIDR:        subnet,
			Source:      "docker",
			Description: fmt.Sprintf("Docker network: %s", network),
			Active:      true,
		})
	}

	return conflicts, nil
}

func (s *NetworkScanner) scanSystemRoutes() ([]models.NetworkConflict, error) {
	var conflicts []models.NetworkConflict

	cmd := exec.Command("ip", "route", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	cidrRegex := regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+/\d+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := cidrRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			cidr := matches[1]

			// Skip common system routes
			if strings.HasPrefix(cidr, "169.254.") || // Link-local
				strings.HasPrefix(cidr, "127.") || // Loopback
				strings.Contains(line, "link") { // Direct link routes
				continue
			}

			// Validate CIDR
			_, _, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}

			conflicts = append(conflicts, models.NetworkConflict{
				CIDR:        cidr,
				Source:      "system",
				Description: fmt.Sprintf("System route: %s", line),
				Active:      true,
			})
		}
	}

	return conflicts, nil
}

func (s *NetworkScanner) GetAllConflicts(ctx context.Context) ([]models.NetworkConflict, error) {
	return s.conflictRepo.GetAll(ctx)
}

func (s *NetworkScanner) AddManualConflict(ctx context.Context, cidr, source, description string) error {
	conflict := &models.NetworkConflict{
		CIDR:        cidr,
		Source:      source,
		Description: description,
		Active:      true,
	}
	return s.conflictRepo.Create(ctx, conflict)
}
