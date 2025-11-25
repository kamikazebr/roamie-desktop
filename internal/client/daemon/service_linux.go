//go:build linux
// +build linux

package daemon

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	systemdServicePath = "/etc/systemd/system/roamie.service"
	serviceName        = "roamie"
)

func setupServicePlatform(cfg ServiceConfig, autoYes bool) error {
	serviceContent := fmt.Sprintf(`[Unit]
Description=Roamie VPN Client Auth Refresh Daemon
After=network.target

[Service]
Type=simple
User=%s
Environment=HOME=%s
ExecStart=%s auth daemon
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target`, cfg.Username, cfg.HomeDir, cfg.ExePath)

	if !autoYes {
		printServiceFile(systemdServicePath, serviceContent)

		fmt.Print("\nCreate this service? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := os.WriteFile(systemdServicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}
	fmt.Println("✓ Service file created")

	// Reload systemd
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	fmt.Println("✓ Systemd reloaded")

	// Enable service
	if err := exec.Command("systemctl", "enable", serviceName).Run(); err != nil {
		fmt.Printf("Warning: Failed to enable service: %v\n", err)
	} else {
		fmt.Println("✓ Service enabled")
	}

	// Restart service
	if err := exec.Command("systemctl", "restart", serviceName).Run(); err != nil {
		fmt.Printf("Warning: Failed to restart service: %v\n", err)
	} else {
		fmt.Println("✓ Service started")
	}

	if !autoYes {
		fmt.Println("\nDaemon setup complete!")
		fmt.Println("Check status: systemctl status roamie")
		fmt.Println("View logs: journalctl -u roamie -f")
	}

	return nil
}

func uninstallServicePlatform() error {
	if _, err := os.Stat(systemdServicePath); os.IsNotExist(err) {
		fmt.Println("Service not installed")
		return nil
	}

	// Stop service
	if err := exec.Command("systemctl", "stop", serviceName).Run(); err != nil {
		fmt.Printf("Warning: Failed to stop service: %v\n", err)
	} else {
		fmt.Println("✓ Service stopped")
	}

	// Disable service
	if err := exec.Command("systemctl", "disable", serviceName).Run(); err != nil {
		fmt.Printf("Warning: Failed to disable service: %v\n", err)
	} else {
		fmt.Println("✓ Service disabled")
	}

	// Remove service file
	if err := os.Remove(systemdServicePath); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	fmt.Println("✓ Service file removed")

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()
	fmt.Println("✓ Systemd reloaded")

	fmt.Println("\nDaemon uninstalled successfully!")
	return nil
}

func isServiceInstalledPlatform() bool {
	_, err := os.Stat(systemdServicePath)
	return err == nil
}

func getServiceStatusPlatform() (string, error) {
	cmd := exec.Command("systemctl", "status", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// systemctl returns non-zero for stopped/failed services
		return string(output), nil
	}
	return string(output), nil
}
