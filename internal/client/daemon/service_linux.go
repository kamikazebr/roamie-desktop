//go:build linux
// +build linux

package daemon

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

const (
	serviceName = "roamie"
)

// getUserServicePath returns the path for user systemd service
func getUserServicePath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "systemd", "user", "roamie.service")
}

// getSystemServicePath returns the path for system-wide systemd service
func getSystemServicePath() string {
	return "/etc/systemd/system/roamie.service"
}

func setupServicePlatform(cfg ServiceConfig, autoYes bool) error {
	// Use user systemd service (starts after login)
	userServiceDir := filepath.Join(cfg.HomeDir, ".config", "systemd", "user")
	userServicePath := getUserServicePath(cfg.HomeDir)

	serviceContent := fmt.Sprintf(`[Unit]
Description=Roamie VPN Client Auth Refresh Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s auth daemon
Restart=always
RestartSec=10

[Install]
WantedBy=default.target`, cfg.ExePath)

	if !autoYes {
		printServiceFile(userServicePath, serviceContent)

		fmt.Print("\nCreate this service? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Create user systemd directory if it doesn't exist
	if err := os.MkdirAll(userServiceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	// Write service file
	if err := os.WriteFile(userServicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}
	fmt.Printf("✓ Service file created: %s\n", userServicePath)

	// Get the actual user for running systemctl --user commands
	sudoUser := os.Getenv("SUDO_USER")
	uid := os.Getenv("SUDO_UID")

	// Reload user systemd
	var reloadCmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		// Running as root via sudo, need to run systemctl as the actual user
		reloadCmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "daemon-reload")
	} else {
		reloadCmd = exec.Command("systemctl", "--user", "daemon-reload")
	}
	if err := reloadCmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	fmt.Println("✓ Systemd user daemon reloaded")

	// Enable service for user login
	var enableCmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		enableCmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "enable", serviceName)
	} else {
		enableCmd = exec.Command("systemctl", "--user", "enable", serviceName)
	}
	if err := enableCmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to enable service: %v\n", err)
	} else {
		fmt.Println("✓ Service enabled (will start on login)")
	}

	// Enable lingering so user services can run without active session (optional but recommended)
	if sudoUser != "" {
		lingerCmd := exec.Command("loginctl", "enable-linger", sudoUser)
		if err := lingerCmd.Run(); err != nil {
			fmt.Printf("Note: Could not enable lingering: %v\n", err)
		} else {
			fmt.Println("✓ User lingering enabled (service persists across sessions)")
		}
	}

	// Start service now
	var startCmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		startCmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "restart", serviceName)
	} else {
		startCmd = exec.Command("systemctl", "--user", "restart", serviceName)
	}
	if err := startCmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to start service: %v\n", err)
	} else {
		fmt.Println("✓ Service started")
	}

	if !autoYes {
		fmt.Println("\nDaemon setup complete!")
		fmt.Println("The daemon will automatically start when you log in.")
		fmt.Println("\nUseful commands:")
		fmt.Println("  Check status: systemctl --user status roamie")
		fmt.Println("  View logs:    journalctl --user -u roamie -f")
		fmt.Println("  Stop:         systemctl --user stop roamie")
		fmt.Println("  Start:        systemctl --user start roamie")
	}

	return nil
}

func uninstallServicePlatform() error {
	// Get user home directory
	_, homeDir, err := utils.GetActualUser()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	servicePath := getUserServicePath(homeDir)

	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		fmt.Println("Service not installed")
		return nil
	}

	// Get SUDO_USER and UID for running systemctl --user commands
	sudoUser := os.Getenv("SUDO_USER")
	uid := os.Getenv("SUDO_UID")

	// Stop service
	var stopCmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		stopCmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "stop", serviceName)
	} else {
		stopCmd = exec.Command("systemctl", "--user", "stop", serviceName)
	}
	if err := stopCmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to stop service: %v\n", err)
	} else {
		fmt.Println("✓ Service stopped")
	}

	// Disable service
	var disableCmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		disableCmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "disable", serviceName)
	} else {
		disableCmd = exec.Command("systemctl", "--user", "disable", serviceName)
	}
	if err := disableCmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to disable service: %v\n", err)
	} else {
		fmt.Println("✓ Service disabled")
	}

	// Remove service file
	if err := os.Remove(servicePath); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	fmt.Println("✓ Service file removed")

	// Reload systemd
	var reloadCmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		reloadCmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "daemon-reload")
	} else {
		reloadCmd = exec.Command("systemctl", "--user", "daemon-reload")
	}
	reloadCmd.Run()
	fmt.Println("✓ Systemd reloaded")

	fmt.Println("\nDaemon uninstalled successfully!")
	return nil
}

func isServiceInstalledPlatform() bool {
	_, homeDir, err := utils.GetActualUser()
	if err != nil {
		return false
	}
	servicePath := getUserServicePath(homeDir)
	_, err = os.Stat(servicePath)
	return err == nil
}

func getServiceStatusPlatform() (string, error) {
	// Get SUDO_USER and UID for running systemctl --user commands
	sudoUser := os.Getenv("SUDO_USER")
	uid := os.Getenv("SUDO_UID")

	var cmd *exec.Cmd
	if sudoUser != "" && uid != "" {
		cmd = exec.Command("sudo", "-u", sudoUser,
			"XDG_RUNTIME_DIR=/run/user/"+uid,
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+uid+"/bus",
			"systemctl", "--user", "status", serviceName)
	} else {
		cmd = exec.Command("systemctl", "--user", "status", serviceName)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// systemctl returns non-zero for stopped/failed services
		return string(output), nil
	}
	return string(output), nil
}
