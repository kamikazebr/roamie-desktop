package wireguard

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	BackupDir      = "/etc/wireguard/backups"
	WireGuardDir   = "/etc/wireguard"
	BackupDirPerm  = 0700
	BackupFilePerm = 0600
)

type BackupInfo struct {
	Timestamp  time.Time
	BackupPath string
	HasConfig  bool
	HasKeys    bool
	PeerCount  int
}

// BackupExistingConfig backs up existing WireGuard configuration before Roamie VPN takes over
func BackupExistingConfig(interfaceName string) (*BackupInfo, error) {
	// Check if interface exists
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}
	defer client.Close()

	device, err := client.Device(interfaceName)
	if err != nil {
		// Interface doesn't exist, no backup needed
		return nil, nil
	}

	// Check if config file exists
	configPath := filepath.Join(WireGuardDir, interfaceName+".conf")
	configExists := fileExists(configPath)

	// If no peers and no config file, no backup needed
	if len(device.Peers) == 0 && !configExists {
		return nil, nil
	}

	timestamp := time.Now().UTC()
	backupName := fmt.Sprintf("roamie-backup-%s", timestamp.Format("20060102-150405"))
	backupPath := filepath.Join(BackupDir, backupName)

	// Create backup directory
	if err := os.MkdirAll(backupPath, BackupDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	log.Printf("⚠️  Existing WireGuard configuration detected on %s", interfaceName)
	log.Printf("📦 Creating backup in: %s", backupPath)

	backupInfo := &BackupInfo{
		Timestamp:  timestamp,
		BackupPath: backupPath,
		PeerCount:  len(device.Peers),
	}

	// Backup config file
	if configExists {
		if err := backupConfigFile(configPath, backupPath, interfaceName); err != nil {
			log.Printf("Warning: Failed to backup config file: %v", err)
		} else {
			backupInfo.HasConfig = true
			log.Printf("✓ Backed up: %s.conf", interfaceName)
		}
	}

	// Backup WireGuard interface state
	if err := backupInterfaceState(device, backupPath, interfaceName); err != nil {
		log.Printf("Warning: Failed to backup interface state: %v", err)
	} else {
		log.Printf("✓ Backed up: interface state (%d peers)", len(device.Peers))
	}

	// Backup any key files (non-Culodi keys)
	if err := backupKeyFiles(backupPath); err != nil {
		log.Printf("Warning: Failed to backup key files: %v", err)
	} else {
		backupInfo.HasKeys = true
		log.Printf("✓ Backed up: key files")
	}

	// Create restore instructions
	if err := createRestoreInstructions(backupPath, interfaceName, backupInfo); err != nil {
		log.Printf("Warning: Failed to create restore instructions: %v", err)
	}

	log.Printf("✓ Backup complete!")
	log.Printf("📝 To restore: see %s/RESTORE.txt", backupPath)

	return backupInfo, nil
}

func backupConfigFile(srcPath, backupPath, interfaceName string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	destPath := filepath.Join(backupPath, interfaceName+".conf")
	return os.WriteFile(destPath, data, BackupFilePerm)
}

func backupInterfaceState(device *wgtypes.Device, backupPath, interfaceName string) error {
	// Save interface state as text file for reference
	statePath := filepath.Join(backupPath, "interface-state.txt")

	content := fmt.Sprintf("Interface: %s\n", interfaceName)
	content += fmt.Sprintf("Public Key: %s\n", device.PublicKey.String())
	content += fmt.Sprintf("Listen Port: %d\n", device.ListenPort)
	content += fmt.Sprintf("Peers: %d\n\n", len(device.Peers))

	for i, peer := range device.Peers {
		content += fmt.Sprintf("Peer %d:\n", i+1)
		content += fmt.Sprintf("  Public Key: %s\n", peer.PublicKey.String())
		content += fmt.Sprintf("  Endpoint: %s\n", peer.Endpoint)
		content += fmt.Sprintf("  Allowed IPs: %v\n", peer.AllowedIPs)
		content += fmt.Sprintf("  Last Handshake: %s\n\n", peer.LastHandshakeTime)
	}

	return os.WriteFile(statePath, []byte(content), BackupFilePerm)
}

func backupKeyFiles(backupPath string) error {
	// Backup all .key files except Culodi's server keys
	keyFiles, err := filepath.Glob(filepath.Join(WireGuardDir, "*.key"))
	if err != nil {
		return err
	}

	for _, keyFile := range keyFiles {
		filename := filepath.Base(keyFile)
		// Skip Culodi's own keys
		if filename == "server_private.key" || filename == "server_public.key" {
			continue
		}

		data, err := os.ReadFile(keyFile)
		if err != nil {
			continue // Skip files we can't read
		}

		destPath := filepath.Join(backupPath, filename)
		if err := os.WriteFile(destPath, data, BackupFilePerm); err != nil {
			return err
		}
	}

	return nil
}

func createRestoreInstructions(backupPath, interfaceName string, info *BackupInfo) error {
	instructions := fmt.Sprintf(`# Roamie VPN - Configuration Restore Instructions

Backup created: %s
Interface: %s
Peers backed up: %d

## To Restore Your Previous Configuration:

### Option 1: Manual Restore

1. Stop Roamie VPN server
2. Stop the WireGuard interface:
   sudo wg-quick down %s

3. Restore the config file:
   sudo cp %s/%s.conf /etc/wireguard/%s.conf

4. Restore any keys (if needed):
   sudo cp %s/*.key /etc/wireguard/

5. Start WireGuard:
   sudo wg-quick up %s

### Option 2: Quick Restore Script

Run:
   sudo bash %s/restore.sh

## Backup Contents:

`, info.Timestamp.Format("2006-01-02 15:04:05 UTC"),
		interfaceName,
		info.PeerCount,
		interfaceName,
		backupPath, interfaceName, interfaceName,
		backupPath,
		interfaceName,
		backupPath)

	if info.HasConfig {
		instructions += fmt.Sprintf("- %s.conf (WireGuard config)\n", interfaceName)
	}
	if info.HasKeys {
		instructions += "- *.key files (WireGuard keys)\n"
	}
	instructions += "- interface-state.txt (peer information)\n"

	instructionsPath := filepath.Join(backupPath, "RESTORE.txt")
	if err := os.WriteFile(instructionsPath, []byte(instructions), 0644); err != nil {
		return err
	}

	// Create restore script
	restoreScript := fmt.Sprintf(`#!/bin/bash
set -e

echo "=== Restoring WireGuard Configuration ==="
echo "Backup from: %s"
echo ""

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (sudo)"
  exit 1
fi

read -p "Stop %s and restore backup? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

echo "Stopping %s..."
wg-quick down %s || true

echo "Restoring configuration..."
cp %s/%s.conf /etc/wireguard/%s.conf
chmod 600 /etc/wireguard/%s.conf

for keyfile in %s/*.key; do
    if [ -f "$keyfile" ]; then
        cp "$keyfile" /etc/wireguard/
        chmod 600 "/etc/wireguard/$(basename $keyfile)"
    fi
done

echo "Starting %s..."
wg-quick up %s

echo "✓ Restore complete!"
`, info.Timestamp.Format("2006-01-02 15:04:05 UTC"),
		interfaceName,
		interfaceName, interfaceName,
		backupPath, interfaceName, interfaceName, interfaceName,
		backupPath,
		interfaceName, interfaceName)

	restoreScriptPath := filepath.Join(backupPath, "restore.sh")
	if err := os.WriteFile(restoreScriptPath, []byte(restoreScript), 0755); err != nil {
		return err
	}

	return nil
}

// ListBackups returns a list of available backups
func ListBackups() ([]string, error) {
	entries, err := os.ReadDir(BackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

// StopInterface stops a WireGuard interface gracefully
func StopInterface(interfaceName string) error {
	cmd := exec.Command("wg-quick", "down", interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop interface: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
