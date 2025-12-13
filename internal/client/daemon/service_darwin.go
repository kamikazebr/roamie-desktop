//go:build darwin
// +build darwin

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
	launchdLabel      = "dev.roamie.daemon"
	launchdPlistName  = "dev.roamie.daemon.plist"
)

func getLaunchdPlistPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "LaunchAgents", launchdPlistName)
}

func getLogPath(homeDir string) string {
	return filepath.Join(homeDir, "Library", "Logs", "roamie.log")
}

func setupServicePlatform(cfg ServiceConfig, autoYes bool) error {
	plistPath := getLaunchdPlistPath(cfg.HomeDir)
	logPath := getLogPath(cfg.HomeDir)

	// Ensure LaunchAgents directory exists (with correct ownership)
	launchAgentsDir := filepath.Dir(plistPath)
	if err := utils.MkdirAllWithOwnership(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Ensure Logs directory exists (with correct ownership)
	logsDir := filepath.Dir(logPath)
	if err := utils.MkdirAllWithOwnership(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create Logs directory: %w", err)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>auth</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>%s</string>
    </dict>
</dict>
</plist>`, launchdLabel, cfg.ExePath, logPath, logPath, cfg.HomeDir)

	if !autoYes {
		printServiceFile(plistPath, plistContent)

		fmt.Print("\nCreate this service? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Unload existing service if present
	if isServiceInstalledPlatform() {
		exec.Command("launchctl", "unload", plistPath).Run()
	}

	if err := utils.WriteFileWithOwnership(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	fmt.Println("✓ Plist file created")

	// Load the service
	cmd := exec.Command("launchctl", "load", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load service: %w\nOutput: %s", err, string(output))
	}
	fmt.Println("✓ Service loaded")

	if !autoYes {
		fmt.Println("\nDaemon setup complete!")
		fmt.Printf("Check status: launchctl list | grep %s\n", launchdLabel)
		fmt.Printf("View logs: tail -f %s\n", logPath)
	}

	return nil
}

func uninstallServicePlatform() error {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistPath := getLaunchdPlistPath(homeDir)

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("Service not installed")
		return nil
	}

	// Unload service
	cmd := exec.Command("launchctl", "unload", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: Failed to unload service: %v\nOutput: %s\n", err, string(output))
	} else {
		fmt.Println("✓ Service unloaded")
	}

	// Remove plist file
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}
	fmt.Println("✓ Plist file removed")

	fmt.Println("\nDaemon uninstalled successfully!")
	return nil
}

func isServiceInstalledPlatform() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	plistPath := getLaunchdPlistPath(homeDir)
	_, err = os.Stat(plistPath)
	return err == nil
}

func getServiceStatusPlatform() (string, error) {
	cmd := exec.Command("launchctl", "list", launchdLabel)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Service not running or not found\nOutput: %s", string(output)), nil
	}
	return string(output), nil
}
