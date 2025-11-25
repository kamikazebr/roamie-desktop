package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
	"github.com/kamikazebr/roamie-desktop/internal/client/ssh"
)

func Run(ctx context.Context) error {
	log.Println("Roamie VPN auth refresh daemon started")

	// JWT refresh ticker (fixed at 1 hour)
	jwtTicker := time.NewTicker(1 * time.Hour)
	defer jwtTicker.Stop()

	// Heartbeat ticker (fixed at 30 seconds)
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Load config to get SSH sync interval
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: failed to load config: %v", err)
	}

	// SSH sync ticker (configurable interval, default 1 hour)
	sshInterval := 1 * time.Hour
	if cfg != nil && cfg.SSHSyncInterval > 0 {
		sshInterval = cfg.SSHSyncInterval
	}
	sshTicker := time.NewTicker(sshInterval)
	defer sshTicker.Stop()

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
			log.Println("Daemon stopped")
			return nil

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

	// Validate device still exists on server
	if err := validateDeviceExists(cfg); err != nil {
		if err.Error() == "device_deleted" {
			log.Println("Device was deleted remotely. Cleaning up local configuration...")
			if err := performLocalCleanup(cfg); err != nil {
				log.Printf("Warning: cleanup failed: %v", err)
			}
			return fmt.Errorf("device was deleted remotely")
		}
		log.Printf("Warning: device validation check failed: %v", err)
		// Don't return error - allow refresh to continue even if validation fails
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
