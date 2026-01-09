package diagnostics

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
	"github.com/kamikazebr/roamie-desktop/internal/client/upgrade"
	"github.com/kamikazebr/roamie-desktop/internal/client/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/version"
)

// CheckStatus represents the result status of a health check
type CheckStatus int

const (
	CheckPassed CheckStatus = iota
	CheckWarning
	CheckError
	CheckInfo
)

// String returns the human-readable status
func (s CheckStatus) String() string {
	switch s {
	case CheckPassed:
		return "PASSED"
	case CheckWarning:
		return "WARNING"
	case CheckError:
		return "ERROR"
	case CheckInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// Symbol returns the display symbol for the status
func (s CheckStatus) Symbol() string {
	switch s {
	case CheckPassed:
		return "✓"
	case CheckWarning:
		return "⚠️"
	case CheckError:
		return "✗"
	case CheckInfo:
		return "ℹ️"
	default:
		return "?"
	}
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Name     string      `json:"name"`
	Category string      `json:"category"`
	Status   CheckStatus `json:"status"`
	Message  string      `json:"message"`
	Fixes    []string    `json:"fixes,omitempty"`
}

// DoctorReport represents the complete diagnostics report
type DoctorReport struct {
	Checks        []CheckResult `json:"checks"`
	Summary       DoctorSummary `json:"summary"`
	ClientVersion string        `json:"client_version"`
	OS            string        `json:"os"`
	Platform      string        `json:"platform"`
	RanAt         time.Time     `json:"ran_at"`
}

// DoctorSummary provides counts of check results
type DoctorSummary struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
	Info     int `json:"info"`
}

// CheckCategory represents a group of related checks
type CheckCategory struct {
	Name   string
	Checks []func(*config.Config) CheckResult
}

// checkConfigLoaded validates that configuration is loaded
func checkConfigLoaded(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:     "Config loaded",
			Category: "Authentication",
			Status:   CheckError,
			Message:  "Not logged in",
			Fixes:    []string{"Run: roamie auth login"},
		}
	}

	return CheckResult{
		Name:     "Config loaded",
		Category: "Authentication",
		Status:   CheckPassed,
		Message:  "Config loaded",
	}
}

// checkJWTValidity validates JWT token expiration
func checkJWTValidity(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:     "JWT validity",
			Category: "Authentication",
			Status:   CheckError,
			Message:  "Config not loaded",
		}
	}

	if cfg.IsExpired() {
		return CheckResult{
			Name:     "JWT validity",
			Category: "Authentication",
			Status:   CheckError,
			Message:  "JWT expired",
			Fixes:    []string{"Run: roamie auth refresh"},
		}
	}

	expiresIn := cfg.ExpiresIn()
	if expiresIn < 24*time.Hour {
		return CheckResult{
			Name:     "JWT validity",
			Category: "Authentication",
			Status:   CheckWarning,
			Message:  fmt.Sprintf("JWT expires soon (%s)", expiresIn.Round(time.Hour)),
			Fixes:    []string{"Run: roamie auth refresh"},
		}
	}

	return CheckResult{
		Name:     "JWT validity",
		Category: "Authentication",
		Status:   CheckPassed,
		Message:  fmt.Sprintf("JWT valid (%s remaining)", expiresIn.Round(time.Hour)),
	}
}

// checkServerReachable validates server connectivity
func checkServerReachable(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:     "Server reachable",
			Category: "Authentication",
			Status:   CheckError,
			Message:  "Config not loaded",
		}
	}

	client := api.NewClient(cfg.ServerURL)
	if err := client.HealthCheck(); err != nil {
		return CheckResult{
			Name:     "Server reachable",
			Category: "Authentication",
			Status:   CheckError,
			Message:  fmt.Sprintf("Server unreachable: %v", err),
		}
	}

	return CheckResult{
		Name:     "Server reachable",
		Category: "Authentication",
		Status:   CheckPassed,
		Message:  fmt.Sprintf("Server reachable (%s)", cfg.ServerURL),
	}
}

// checkWireGuardInstalled validates WireGuard installation
func checkWireGuardInstalled(cfg *config.Config) CheckResult {
	if wireguard.CheckInstalled() {
		return CheckResult{
			Name:     "WireGuard installed",
			Category: "Network",
			Status:   CheckPassed,
			Message:  "WireGuard installed",
		}
	}

	return CheckResult{
		Name:     "WireGuard installed",
		Category: "Network",
		Status:   CheckWarning,
		Message:  "WireGuard not installed",
		Fixes:    []string{"Run: roamie vpn install"},
	}
}

// checkVPNConfigured validates VPN configuration
func checkVPNConfigured(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:     "VPN configured",
			Category: "Network",
			Status:   CheckWarning,
			Message:  "Config not loaded",
		}
	}

	if !cfg.VPNEnabled {
		return CheckResult{
			Name:     "VPN configured",
			Category: "Network",
			Status:   CheckWarning,
			Message:  "VPN disabled (SSH tunnel only)",
		}
	}

	if cfg.VpnIP != "" {
		return CheckResult{
			Name:     "VPN configured",
			Category: "Network",
			Status:   CheckPassed,
			Message:  fmt.Sprintf("VPN configured (IP: %s)", cfg.VpnIP),
		}
	}

	return CheckResult{
		Name:     "VPN configured",
		Category: "Network",
		Status:   CheckWarning,
		Message:  "VPN enabled but not configured",
	}
}

// checkDaemonRunning validates daemon status
func checkDaemonRunning(cfg *config.Config) CheckResult {
	if isServiceRunning("roamie") {
		return CheckResult{
			Name:     "Daemon running",
			Category: "Services",
			Status:   CheckPassed,
			Message:  "Daemon running",
		}
	}

	return CheckResult{
		Name:     "Daemon running",
		Category: "Services",
		Status:   CheckWarning,
		Message:  "Daemon not running",
		Fixes:    []string{"Run: roamie setup-daemon"},
	}
}

// checkTunnelStatus validates tunnel configuration
func checkTunnelStatus(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:     "Tunnel status",
			Category: "Services",
			Status:   CheckWarning,
			Message:  "Config not loaded",
		}
	}

	if cfg.TunnelEnabled && cfg.TunnelPort > 0 {
		return CheckResult{
			Name:     "Tunnel status",
			Category: "Services",
			Status:   CheckPassed,
			Message:  fmt.Sprintf("Tunnel enabled (port %d)", cfg.TunnelPort),
		}
	}

	if cfg.TunnelPort > 0 {
		return CheckResult{
			Name:     "Tunnel status",
			Category: "Services",
			Status:   CheckWarning,
			Message:  fmt.Sprintf("Tunnel registered but disabled (port %d)", cfg.TunnelPort),
			Fixes:    []string{"Run: roamie tunnel enable"},
		}
	}

	return CheckResult{
		Name:     "Tunnel status",
		Category: "Services",
		Status:   CheckWarning,
		Message:  "Tunnel not registered",
		Fixes:    []string{"Run: roamie tunnel register"},
	}
}

// checkAutoUpgrade validates auto-upgrade status
func checkAutoUpgrade(cfg *config.Config) CheckResult {
	if cfg == nil {
		return CheckResult{
			Name:     "Auto-upgrade",
			Category: "Services",
			Status:   CheckWarning,
			Message:  "Config not loaded",
		}
	}

	if cfg.AutoUpgradeEnabled {
		return CheckResult{
			Name:     "Auto-upgrade",
			Category: "Services",
			Status:   CheckPassed,
			Message:  "Auto-upgrade enabled",
		}
	}

	return CheckResult{
		Name:     "Auto-upgrade",
		Category: "Services",
		Status:   CheckWarning,
		Message:  "Auto-upgrade disabled",
		Fixes:    []string{"Run: roamie auto-upgrade on"},
	}
}

// checkUpdateAvailable checks for available updates
func checkUpdateAvailable(cfg *config.Config) CheckResult {
	result, err := upgrade.CheckForUpdates()
	if err != nil {
		return CheckResult{
			Name:     "Updates",
			Category: "Updates",
			Status:   CheckWarning,
			Message:  fmt.Sprintf("Could not check for updates: %v", err),
		}
	}

	if result.UpdateAvailable {
		return CheckResult{
			Name:     "Updates",
			Category: "Updates",
			Status:   CheckWarning,
			Message:  fmt.Sprintf("Update available: %s → %s", result.CurrentVersion, result.LatestVersion),
			Fixes:    []string{"Run: roamie upgrade"},
		}
	}

	return CheckResult{
		Name:     "Updates",
		Category: "Updates",
		Status:   CheckPassed,
		Message:  fmt.Sprintf("Up to date (%s)", result.CurrentVersion),
	}
}

// GetDoctorChecks returns all diagnostic checks organized by category
func GetDoctorChecks() []CheckCategory {
	return []CheckCategory{
		{
			Name: "Authentication",
			Checks: []func(*config.Config) CheckResult{
				checkConfigLoaded,
				checkJWTValidity,
				checkServerReachable,
			},
		},
		{
			Name: "Network",
			Checks: []func(*config.Config) CheckResult{
				checkWireGuardInstalled,
				checkVPNConfigured,
			},
		},
		{
			Name: "Services",
			Checks: []func(*config.Config) CheckResult{
				checkDaemonRunning,
				checkTunnelStatus,
				checkAutoUpgrade,
			},
		},
		{
			Name: "Updates",
			Checks: []func(*config.Config) CheckResult{
				checkUpdateAvailable,
			},
		},
	}
}

// RunDoctorProgrammatic runs all diagnostic checks and returns structured results
// This is used by the daemon for remote diagnostics
func RunDoctorProgrammatic() DoctorReport {
	cfg, _ := config.Load()

	var allChecks []CheckResult
	summary := DoctorSummary{}

	categories := GetDoctorChecks()
	for _, category := range categories {
		for _, checkFunc := range category.Checks {
			result := checkFunc(cfg)
			result.Category = category.Name
			allChecks = append(allChecks, result)

			switch result.Status {
			case CheckPassed:
				summary.Passed++
			case CheckWarning:
				summary.Warnings++
			case CheckError:
				summary.Errors++
			case CheckInfo:
				summary.Info++
			}
		}
	}

	report := DoctorReport{
		Checks:        allChecks,
		Summary:       summary,
		ClientVersion: version.Version,
		OS:            runtime.GOOS,
		Platform:      runtime.GOARCH,
		RanAt:         time.Now().UTC(),
	}

	return report
}

// isServiceRunning checks if a systemd user service is running
func isServiceRunning(name string) bool {
	sudoUser := os.Getenv("SUDO_USER")
	uid := os.Getenv("SUDO_UID")

	var cmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		cmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "is-active", "--quiet", name)
	} else {
		cmd = exec.Command("systemctl", "--user", "is-active", "--quiet", name)
	}
	return cmd.Run() == nil
}
