package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/client/claude/hooks"
	"github.com/kamikazebr/roamie-desktop/internal/client/claude/installer"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"github.com/spf13/cobra"
)

// install-hooks command
var installHooksCmd = &cobra.Command{
	Use:   "install-hooks",
	Short: "Install Claude Code notification hooks",
	Long: `Install Claude Code notification hooks.

This will:
  1. Create backup of existing settings.json
  2. Update settings.json (preserving existing configs)
  3. Configure hooks: Stop, Notification

Safe to run multiple times - preserves all existing settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get real user (works with sudo)
		username, homeDir, err := utils.GetActualUser()
		if err != nil {
			return fmt.Errorf("failed to get user directory: %w", err)
		}

		// Get ABSOLUTE path to roamie binary (don't move it)
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		execPath, _ = filepath.EvalSymlinks(execPath)

		claudeDir := filepath.Join(homeDir, ".claude")
		settingsPath := filepath.Join(claudeDir, "settings.json")

		// Show what we're going to do
		fmt.Println("🔧 Claude Code Hooks - Installation")
		fmt.Printf("   Binary: %s\n", execPath)
		fmt.Printf("   Config: %s\n", settingsPath)
		fmt.Printf("   User: %s\n", username)
		fmt.Printf("   Hooks: Stop, Notification\n\n")

		// Check if already exists
		if _, err := os.Stat(settingsPath); err == nil {
			fmt.Println("⚠️  File settings.json already exists")
			fmt.Println("   Automatic backup will be created before modifying")
			fmt.Println("")
		}

		// Create directory if needed
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			return fmt.Errorf("failed to create .claude dir: %w", err)
		}

		// Automatic backup
		backupPath := installer.BackupSettings(settingsPath)
		if backupPath != "" {
			fmt.Printf("📦 Backup: %s\n", backupPath)
		}

		// Install (intelligent merge)
		if err := installer.InstallHooks(settingsPath, execPath); err != nil {
			return err
		}

		// Fix ownership (uses utils.FixFileOwnership)
		utils.FixFileOwnership(settingsPath)

		fmt.Println("✅ Installation completed successfully!")
		fmt.Println("")
		fmt.Println("Active hooks:")
		fmt.Println("  • Stop → Notifies when Claude finishes")
		fmt.Println("  • Notification → Notifies Claude alerts")
		fmt.Println("")
		fmt.Println("Notification types:")
		fmt.Println("  • permission_prompt → Permission requests (critical)")
		fmt.Println("  • idle_prompt → Claude waiting 60+ seconds (critical)")
		fmt.Println("  • auth_success → Authentication success")
		fmt.Println("  • elicitation_dialog → Input required")
		fmt.Println("")
		fmt.Printf("Logs: tail -f ~/.roamie/logs/claude-hooks.log\n")

		return nil
	},
}

// uninstall-hooks command
var uninstallHooksCmd = &cobra.Command{
	Use:   "uninstall-hooks",
	Short: "Remove Claude Code hooks (preserves other configs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print("Remove hooks Stop and Notification? (yes/no): ")
		var confirm string
		fmt.Scanln(&confirm)

		if strings.ToLower(confirm) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}

		_, homeDir, err := utils.GetActualUser()
		if err != nil {
			return err
		}

		settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

		// Backup before removing
		backupPath := installer.BackupSettings(settingsPath)

		if err := installer.UninstallHooks(settingsPath); err != nil {
			return err
		}

		utils.FixFileOwnership(settingsPath)

		fmt.Println("✅ Hooks removed!")
		fmt.Printf("   Backup: %s\n", backupPath)
		return nil
	},
}

// restore-hooks command
var restoreHooksCmd = &cobra.Command{
	Use:   "restore-hooks",
	Short: "Restore hooks configuration from backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, homeDir, err := utils.GetActualUser()
		if err != nil {
			return err
		}

		settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

		backups, err := installer.ListBackups(settingsPath)
		if err != nil || len(backups) == 0 {
			return fmt.Errorf("no backups found")
		}

		fmt.Println("Available backups:")
		for i, b := range backups {
			fmt.Printf("  %d. %s\n", i+1, filepath.Base(b))
		}

		latest := backups[len(backups)-1]
		fmt.Printf("\nRestore: %s? (yes/no): ", filepath.Base(latest))
		var confirm string
		fmt.Scanln(&confirm)

		if strings.ToLower(confirm) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}

		// Backup current state before restoring
		installer.BackupSettings(settingsPath)

		if err := installer.RestoreFromBackup(latest, settingsPath); err != nil {
			return err
		}

		utils.FixFileOwnership(settingsPath)
		fmt.Println("✅ Configuration restored!")
		return nil
	},
}

// claude-hooks command (internal)
var claudeHooksCmd = &cobra.Command{
	Use:    "claude-hooks",
	Short:  "Execute hooks (internal use by Claude Code)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return hooks.Run()
	},
}

func init() {
	rootCmd.AddCommand(installHooksCmd, uninstallHooksCmd, restoreHooksCmd, claudeHooksCmd)
}
