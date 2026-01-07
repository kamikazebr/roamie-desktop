package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
	"github.com/kamikazebr/roamie-desktop/internal/client/ssh"
	"github.com/kamikazebr/roamie-desktop/internal/client/tunnel"
	"github.com/kamikazebr/roamie-desktop/internal/client/upgrade"
	"github.com/kamikazebr/roamie-desktop/pkg/version"
)

func Run(ctx context.Context) error {
	log.Println("Roamie VPN auth refresh daemon started")

	// JWT refresh ticker (fixed at 1 hour)
	jwtTicker := time.NewTicker(1 * time.Hour)
	defer jwtTicker.Stop()

	// Heartbeat ticker (fixed at 30 seconds)
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Config check ticker (every 10 seconds to detect tunnel enable/disable)
	configTicker := time.NewTicker(10 * time.Second)
	defer configTicker.Stop()

	// Tunnel health check ticker (every 30 seconds to detect broken connections)
	tunnelHealthTicker := time.NewTicker(30 * time.Second)
	defer tunnelHealthTicker.Stop()

	// Load config to get SSH sync interval
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config: %v", err)
	}

	// SSH sync ticker (configurable interval, default 5 minutes)
	sshInterval := 5 * time.Minute
	if cfg != nil && cfg.SSHSyncInterval > 0 {
		sshInterval = cfg.SSHSyncInterval
	}
	sshTicker := time.NewTicker(sshInterval)
	defer sshTicker.Stop()

	// Auto-upgrade ticker (check every 24 hours)
	upgradeTicker := time.NewTicker(24 * time.Hour)
	defer upgradeTicker.Stop()

	// Tunnel state management
	var tunnelClient *tunnel.Client
	var tunnelCancel context.CancelFunc
	tunnelEnabled := false

	// Start tunnel if enabled in config
	if cfg != nil && cfg.TunnelEnabled {
		log.Println("Tunnel enabled in config, starting...")
		tunnelClient, tunnelCancel = startTunnel(ctx, cfg)
		if tunnelClient != nil {
			tunnelEnabled = true
		}
	}

	// Do initial checks immediately
	if err := checkAndRefresh(); err != nil {
		log.Printf("Initial JWT refresh check failed: %v", err)
	}
	if err := syncSSH(); err != nil {
		log.Printf("Initial SSH sync failed: %v", err)
	}
	// Send initial heartbeat
	if err := sendHeartbeat(); err != nil {
		log.Printf("Initial heartbeat failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Daemon stopping...")
			// Stop tunnel if running
			if tunnelCancel != nil {
				tunnelCancel()
			}
			if tunnelClient != nil {
				tunnelClient.Disconnect()
			}
			log.Println("Daemon stopped")
			return nil

		case <-configTicker.C:
			// Check if tunnel config changed
			newCfg, err := config.Load()
			if err != nil {
				log.Printf("Warning: failed to reload config: %v", err)
				continue
			}
			if newCfg == nil {
				continue
			}

			// Check if tunnel state changed
			if newCfg.TunnelEnabled != tunnelEnabled {
				if newCfg.TunnelEnabled {
					// Start tunnel
					log.Println("Tunnel enabled, starting...")
					tunnelClient, tunnelCancel = startTunnel(ctx, newCfg)
					if tunnelClient != nil {
						tunnelEnabled = true
					}
				} else {
					// Stop tunnel
					log.Println("Tunnel disabled, stopping...")
					if tunnelCancel != nil {
						tunnelCancel()
					}
					if tunnelClient != nil {
						tunnelClient.Disconnect()
						tunnelClient = nil
					}
					tunnelCancel = nil
					tunnelEnabled = false
				}
			}

			// Update cfg reference for other operations
			cfg = newCfg

		case <-jwtTicker.C:
			if err := checkAndRefresh(); err != nil {
				log.Printf("JWT refresh check failed: %v", err)
			}

		case <-heartbeatTicker.C:
			if err := sendHeartbeat(); err != nil {
				// Log but don't spam - heartbeat failures are common when VPN is disconnected
				// Only log in debug mode or periodically
			}

		case <-sshTicker.C:
			if err := syncSSH(); err != nil {
				log.Printf("SSH sync failed: %v", err)
			}

		case <-tunnelHealthTicker.C:
			// Check if tunnel should be running but isn't connected
			if tunnelEnabled && tunnelClient != nil {
				if !tunnelClient.IsConnected() {
					log.Println("Tunnel health check: connection lost, restarting...")
					// Stop the old tunnel
					if tunnelCancel != nil {
						tunnelCancel()
					}
					tunnelClient.Disconnect()

					// Reload config and restart tunnel
					newCfg, err := config.Load()
					if err != nil {
						log.Printf("Failed to reload config for tunnel restart: %v", err)
					} else if newCfg != nil && newCfg.TunnelEnabled {
						tunnelClient, tunnelCancel = startTunnel(ctx, newCfg)
						if tunnelClient == nil {
							tunnelEnabled = false
							log.Println("Tunnel restart failed")
						} else {
							log.Println("✓ Tunnel restarted successfully")
						}
					}
				}
			}

		case <-upgradeTicker.C:
			// Reload config to get latest auto-upgrade setting
			upgradeCfg, err := config.Load()
			if err != nil {
				log.Printf("Warning: failed to load config for upgrade check: %v", err)
				continue
			}
			if upgradeCfg != nil && upgradeCfg.AutoUpgradeEnabled {
				if err := checkAndAutoUpgrade(upgradeCfg); err != nil {
					log.Printf("Auto-upgrade check failed: %v", err)
				}
			}
		}
	}
}

func checkAndRefresh() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("no configuration found, please run 'roamie auth login'")
	}

	// Grace period: Skip device validation if config was created recently (within 2 minutes)
	// This prevents race conditions where the daemon starts before the login flow completes
	gracePeriod := 2 * time.Minute
	configAge := time.Since(cfg.CreatedAt)
	if configAge < gracePeriod {
		log.Printf("Config created %s ago (grace period: %s), skipping device validation",
			configAge.Round(time.Second), gracePeriod)
	} else {
		// Validate device still exists on server (with retry for transient failures)
		if err := validateDeviceExistsWithRetry(cfg, 3); err != nil {
			if errors.Is(err, api.ErrDeviceDeleted) {
				log.Println("Device was deleted remotely. Cleaning up local configuration...")
				if err := performLocalCleanup(cfg); err != nil {
					log.Printf("Warning: cleanup failed: %v", err)
				}
				return fmt.Errorf("device was deleted remotely")
			}
			log.Printf("Warning: device validation check failed: %v", err)
			// Don't return error - allow refresh to continue even if validation fails
		}
	}

	// Check if JWT expires in < 24 hours
	expiresIn := cfg.ExpiresIn()

	if expiresIn < 24*time.Hour {
		log.Printf("JWT expires in %s, refreshing...", expiresIn.Round(time.Hour))

		client := api.NewClient(cfg.ServerURL)
		resp, err := client.RefreshJWT(cfg.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to refresh JWT: %w", err)
		}

		// Parse new expires_at
		expiresAt, _ := time.Parse("2006-01-02T15:04:05Z", resp.ExpiresAt)

		// Update config
		cfg.JWT = resp.JWT
		cfg.ExpiresAt = expiresAt

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		log.Printf("✓ JWT refreshed successfully (expires: %s)", expiresAt.Format("2006-01-02 15:04:05"))
	} else {
		log.Printf("JWT valid for %s, no refresh needed", expiresIn.Round(time.Hour))
	}

	return nil
}

func validateDeviceExists(cfg *config.Config) error {
	if cfg.DeviceID == "" {
		return nil // Skip validation if device ID not set
	}

	client := api.NewClient(cfg.ServerURL)
	_, err := client.ValidateDevice(cfg.DeviceID, cfg.JWT)
	return err
}

// validateDeviceExistsWithRetry validates device with retries for transient failures
// This helps avoid false "device deleted" errors due to network issues or server restarts
func validateDeviceExistsWithRetry(cfg *config.Config, maxRetries int) error {
	if cfg.DeviceID == "" {
		return nil // Skip validation if device ID not set
	}

	client := api.NewClient(cfg.ServerURL)
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := client.ValidateDevice(cfg.DeviceID, cfg.JWT)

		if err == nil {
			return nil // Success
		}

		lastErr = err

		// For device_deleted, still retry to handle potential race conditions
		// or server restarts, but with a fixed 5s delay
		if errors.Is(err, api.ErrDeviceDeleted) {
			if attempt < maxRetries {
				log.Printf("Device validation returned 404, retrying in 5s (attempt %d/%d)...", attempt, maxRetries)
				time.Sleep(5 * time.Second)
				continue
			}
		} else {
			// For other errors (network, timeout), retry with backoff
			if attempt < maxRetries {
				backoff := time.Duration(attempt*2) * time.Second
				log.Printf("Device validation failed: %v, retrying in %s (attempt %d/%d)...", err, backoff, attempt, maxRetries)
				time.Sleep(backoff)
				continue
			}
		}
	}

	return lastErr
}

func performLocalCleanup(cfg *config.Config) error {
	log.Println("Disconnecting VPN...")
	// Note: In a real implementation, this would disconnect the VPN
	// For now, we'll just clean up the config files

	log.Println("Removing local configuration...")
	if err := config.Delete(); err != nil {
		return fmt.Errorf("failed to delete config: %w", err)
	}

	return nil
}

func syncSSH() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("no configuration found")
	}

	// Check if SSH sync is enabled
	if !cfg.SSHSyncEnabled {
		// Silently skip if disabled
		return nil
	}

	// Check if authenticated
	if cfg.JWT == "" {
		return fmt.Errorf("not authenticated")
	}

	// Create SSH manager
	sshManager, err := ssh.NewManager(cfg.ServerURL)
	if err != nil {
		return fmt.Errorf("failed to create SSH manager: %w", err)
	}

	// Sync keys
	result, err := sshManager.SyncKeys(cfg)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Log only if there were changes
	if len(result.Added) > 0 || len(result.Removed) > 0 {
		log.Printf("SSH sync completed: %d added, %d removed, %d total",
			len(result.Added), len(result.Removed), result.Total)
	}

	return nil
}

func sendHeartbeat() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("no configuration found")
	}

	// Skip if not authenticated
	if cfg.JWT == "" || cfg.DeviceID == "" {
		return nil
	}

	client := api.NewClient(cfg.ServerURL)
	if err := client.SendHeartbeat(cfg.DeviceID, cfg.JWT); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	return nil
}

// startTunnel starts the SSH tunnel with the given config
// Returns the tunnel client and a cancel function to stop it
func startTunnel(ctx context.Context, cfg *config.Config) (*tunnel.Client, context.CancelFunc) {
	// Create a cancellable context for the tunnel
	tunnelCtx, cancel := context.WithCancel(ctx)

	// Create tunnel client with the context
	client, err := tunnel.NewClientWithContext(tunnelCtx, cfg)
	if err != nil {
		log.Printf("Failed to create tunnel client: %v", err)
		cancel()
		return nil, nil
	}

	// Start the tunnel connection in background
	go func() {
		if err := client.Connect(); err != nil {
			log.Printf("Tunnel connection failed: %v", err)
		}
	}()

	log.Printf("✓ Tunnel started (port %d)", cfg.TunnelPort)
	return client, cancel
}

// checkAndAutoUpgrade checks for updates and performs automatic upgrade if available
func checkAndAutoUpgrade(cfg *config.Config) error {
	currentVersion := version.Version

	// Skip if running dev version
	if currentVersion == "dev" {
		log.Println("Auto-upgrade: skipping dev version")
		return nil
	}

	log.Println("Auto-upgrade: checking for updates...")

	// Check for updates
	result, err := upgrade.CheckForUpdates()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Update last check time
	cfg.LastUpgradeCheck = time.Now()
	cfg.Save()

	if !result.UpdateAvailable {
		log.Printf("Auto-upgrade: already on latest version (%s)", currentVersion)
		return nil
	}

	if result.DownloadURL == "" {
		log.Printf("Auto-upgrade: update available (%s) but no compatible binary found", result.LatestVersion)
		return nil
	}

	log.Printf("Auto-upgrade: updating from %s to %s...", currentVersion, result.LatestVersion)

	// Save info for notification after restart
	cfg.LastBackgroundUpdate = &config.BackgroundUpdateInfo{
		FromVersion: currentVersion,
		ToVersion:   result.LatestVersion,
		UpdatedAt:   time.Now(),
		Shown:       false,
	}
	if err := cfg.Save(); err != nil {
		log.Printf("Warning: failed to save update info: %v", err)
	}

	// Perform the upgrade
	if err := upgrade.Upgrade(result); err != nil {
		// Clear the update info on failure
		cfg.LastBackgroundUpdate = nil
		cfg.Save()
		return fmt.Errorf("upgrade failed: %w", err)
	}

	log.Printf("Auto-upgrade: successfully upgraded to %s", result.LatestVersion)

	// Restart the daemon service
	log.Println("Auto-upgrade: restarting daemon...")
	restartDaemon()

	return nil
}

// restartDaemon restarts the daemon service
func restartDaemon() {
	// Try systemctl restart first (Linux)
	cmd := exec.Command("systemctl", "--user", "restart", "roamie")
	if err := cmd.Run(); err != nil {
		// If systemctl fails, try to exec ourselves
		log.Printf("systemctl restart failed, attempting self-restart: %v", err)

		// Get current executable path
		execPath, err := os.Executable()
		if err != nil {
			log.Printf("Failed to get executable path: %v", err)
			return
		}

		// Replace current process with new binary
		log.Println("Restarting daemon process...")
		if err := exec.Command(execPath, "daemon").Start(); err != nil {
			log.Printf("Failed to restart daemon: %v", err)
		}
		os.Exit(0)
	}
}
