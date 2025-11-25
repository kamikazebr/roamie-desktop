package daemon

import (
	"fmt"
	"os"
	"runtime"
)

// ServiceConfig contains configuration for setting up the daemon service
type ServiceConfig struct {
	ExePath  string
	Username string
	HomeDir  string
}

// SetupService sets up the platform-specific daemon service
func SetupService(cfg ServiceConfig, autoYes bool) error {
	return setupServicePlatform(cfg, autoYes)
}

// UninstallService removes the platform-specific daemon service
func UninstallService() error {
	return uninstallServicePlatform()
}

// IsServiceInstalled checks if the daemon service is already installed
func IsServiceInstalled() bool {
	return isServiceInstalledPlatform()
}

// GetServiceStatus returns the status of the daemon service
func GetServiceStatus() (string, error) {
	return getServiceStatusPlatform()
}

// GetServiceInstructions returns platform-specific instructions for the daemon service
func GetServiceInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return `
Daemon setup on macOS uses launchd.

Commands:
  • Setup:     sudo roamie setup-daemon
  • Status:    launchctl list | grep roamie
  • Logs:      cat ~/Library/Logs/roamie.log
  • Stop:      launchctl unload ~/Library/LaunchAgents/dev.roamie.daemon.plist
  • Start:     launchctl load ~/Library/LaunchAgents/dev.roamie.daemon.plist`

	case "windows":
		return `
Daemon setup on Windows uses Task Scheduler.

Commands:
  • Setup:     roamie setup-daemon
  • Status:    schtasks /query /tn "Roamie VPN Daemon"
  • Logs:      Check %LOCALAPPDATA%\Roamie\logs\
  • Stop:      schtasks /end /tn "Roamie VPN Daemon"
  • Start:     schtasks /run /tn "Roamie VPN Daemon"`

	default: // Linux
		return `
Daemon setup on Linux uses systemd.

Commands:
  • Setup:     sudo roamie setup-daemon
  • Status:    systemctl status roamie
  • Logs:      journalctl -u roamie -f
  • Stop:      sudo systemctl stop roamie
  • Start:     sudo systemctl start roamie`
	}
}

// checkRoot checks if the current process has root/admin privileges
func checkRoot() bool {
	if runtime.GOOS == "windows" {
		// On Windows, we don't need root for Task Scheduler user tasks
		return true
	}
	return os.Geteuid() == 0
}

// printServiceFile prints the service file content for review
func printServiceFile(path, content string) {
	fmt.Printf("\nService file: %s\n", path)
	fmt.Println("\nContent:")
	fmt.Println("---")
	fmt.Println(content)
	fmt.Println("---")
}
