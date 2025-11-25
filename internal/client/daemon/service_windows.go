//go:build windows
// +build windows

package daemon

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	taskName = "Roamie VPN Daemon"
)

func getLogPath() string {
	appData := os.Getenv("LOCALAPPDATA")
	if appData == "" {
		appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(appData, "Roamie", "logs", "daemon.log")
}

func setupServicePlatform(cfg ServiceConfig, autoYes bool) error {
	logPath := getLogPath()

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Build the schtasks command
	// Create a task that runs at logon and restarts on failure
	taskXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Roamie VPN Client Auth Refresh Daemon - Keeps your VPN authentication tokens fresh</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>%s</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>%s</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <DisallowStartOnRemoteAppSession>false</DisallowStartOnRemoteAppSession>
    <UseUnifiedSchedulingEngine>true</UseUnifiedSchedulingEngine>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%s</Command>
      <Arguments>auth daemon</Arguments>
    </Exec>
  </Actions>
</Task>`, cfg.Username, cfg.Username, cfg.ExePath)

	if !autoYes {
		fmt.Println("\nTask Scheduler configuration:")
		fmt.Printf("  Task Name: %s\n", taskName)
		fmt.Printf("  Command: %s auth daemon\n", cfg.ExePath)
		fmt.Printf("  Log Path: %s\n", logPath)
		fmt.Println("  Triggers: Run at logon")
		fmt.Println("  Restart on failure: Yes (3 times, 1 min interval)")

		fmt.Print("\nCreate this scheduled task? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Write XML to temp file
	tempFile, err := os.CreateTemp("", "roamie-task-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(taskXML); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write task XML: %w", err)
	}
	tempFile.Close()

	// Delete existing task if present
	exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()

	// Create new task from XML
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/xml", tempFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %w\nOutput: %s", err, string(output))
	}
	fmt.Println("✓ Scheduled task created")

	// Start the task immediately
	cmd = exec.Command("schtasks", "/run", "/tn", taskName)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: Failed to start task immediately: %v\nOutput: %s\n", err, string(output))
		fmt.Println("The task will start automatically at next logon.")
	} else {
		fmt.Println("✓ Task started")
	}

	if !autoYes {
		fmt.Println("\nDaemon setup complete!")
		fmt.Printf("Check status: schtasks /query /tn \"%s\"\n", taskName)
		fmt.Printf("View logs: type \"%s\"\n", logPath)
	}

	return nil
}

func uninstallServicePlatform() error {
	if !isServiceInstalledPlatform() {
		fmt.Println("Service not installed")
		return nil
	}

	// End the task if running
	cmd := exec.Command("schtasks", "/end", "/tn", taskName)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: Failed to stop task: %v\nOutput: %s\n", err, string(output))
	} else {
		fmt.Println("✓ Task stopped")
	}

	// Delete the task
	cmd = exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete task: %w\nOutput: %s", err, string(output))
	}
	fmt.Println("✓ Scheduled task deleted")

	fmt.Println("\nDaemon uninstalled successfully!")
	return nil
}

func isServiceInstalledPlatform() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", taskName)
	err := cmd.Run()
	return err == nil
}

func getServiceStatusPlatform() (string, error) {
	cmd := exec.Command("schtasks", "/query", "/tn", taskName, "/v", "/fo", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Task not found or error querying: %v", err), nil
	}
	return string(output), nil
}
