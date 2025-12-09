// Package sshd provides SSH daemon detection and installation helpers
package sshd

import (
	"fmt"
	"net"
	"time"
)

// PreflightResult contains the result of SSH daemon preflight checks
type PreflightResult struct {
	Installed      bool
	Running        bool
	Port           int
	CanAutoInstall bool
	Message        string
}

// Check performs a preflight check for SSH daemon availability
func Check() (*PreflightResult, error) {
	result := &PreflightResult{
		Port: 22,
	}

	// First check if port 22 is listening (fastest check)
	if isPortListening(22) {
		result.Installed = true
		result.Running = true
		return result, nil
	}

	// Port not listening, do platform-specific checks
	return checkPlatform(result)
}

// isPortListening checks if a port is listening on localhost
func isPortListening(port int) bool {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// PromptInstall checks for SSH daemon and prompts for installation if needed
// Returns (shouldContinue, error)
// - shouldContinue=true: sshd is available or user wants to continue anyway
// - shouldContinue=false: user cancelled or installation failed
func PromptInstall() (bool, error) {
	result, err := Check()
	if err != nil {
		return false, err
	}

	if result.Running {
		return true, nil
	}

	fmt.Println()
	fmt.Println("⚠️  SSH server (sshd) is not running on this machine.")
	fmt.Println("   The SSH tunnel requires sshd to accept incoming connections.")
	fmt.Println()

	return promptInstallPlatform(result)
}

// GetInstallInstructions returns platform-specific installation instructions
func GetInstallInstructions() string {
	return getInstallInstructions()
}

// IsRunning returns true if SSH daemon is running and accepting connections
func IsRunning() bool {
	return isPortListening(22)
}
