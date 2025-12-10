package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/auth"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
	"github.com/kamikazebr/roamie-desktop/internal/client/daemon"
	"github.com/kamikazebr/roamie-desktop/internal/client/ssh"
	"github.com/kamikazebr/roamie-desktop/internal/client/sshd"
	"github.com/kamikazebr/roamie-desktop/internal/client/tunnel"
	"github.com/kamikazebr/roamie-desktop/internal/client/upgrade"
	"github.com/kamikazebr/roamie-desktop/internal/client/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"github.com/kamikazebr/roamie-desktop/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "roamie",
	Short: "Roamie VPN Client - Device Authentication",
	Long:  "CLI tool for managing Roamie VPN device authentication and JWT tokens",
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var loginCmd = &cobra.Command{
	Use:   "login [server-url]",
	Short: "Login to Roamie VPN via QR code",
	Args:  cobra.MaximumNArgs(1),
	Run:   runLogin,
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run token refresh daemon",
	Run:   runDaemon,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Run:   runStatus,
}

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Manually refresh JWT token",
	Run:   runRefresh,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout and delete credentials",
	Run:   runLogout,
}

var setupDaemonYes bool

var setupDaemonCmd = &cobra.Command{
	Use:   "setup-daemon",
	Short: "Setup systemd service for auto-refresh",
	Run:   runSetupDaemon,
}

var uninstallDaemonCmd = &cobra.Command{
	Use:   "uninstall-daemon",
	Short: "Uninstall systemd service",
	Run:   runUninstallDaemon,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.GetVersion("roamie"))
	},
}

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH key management commands",
}

var sshSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manually sync SSH keys from Firestore",
	Run:   runSSHSync,
}

var sshStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSH sync status",
	Run:   runSSHStatus,
}

var sshEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable automatic SSH key sync",
	Run:   runSSHEnable,
}

var sshDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable automatic SSH key sync",
	Run:   runSSHDisable,
}

var sshSetIntervalCmd = &cobra.Command{
	Use:   "set-interval [duration]",
	Short: "Set SSH sync interval (e.g., 5m, 1h, 30s)",
	Args:  cobra.ExactArgs(1),
	Run:   runSSHSetInterval,
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to VPN using saved configuration",
	Long:  "Connect to VPN using saved configuration. Requires root privileges.\nAlternatively, you can use: sudo wg-quick up roamie",
	Run:   runConnect,
}

var disconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect from VPN",
	Long:  "Disconnect from VPN. Requires root privileges.\nAlternatively, you can use: sudo wg-quick down roamie",
	Run:   runDisconnect,
}

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "SSH reverse tunnel management commands",
}

var tunnelStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start SSH reverse tunnel (for debug/manual use)",
	Long: `Start SSH reverse tunnel manually.

Note: For production use, the daemon manages the tunnel automatically.
This command is for debugging or running without the daemon.

The tunnel will be started automatically by the daemon when:
  - You login with 'roamie auth login' (auto-registers tunnel)
  - The daemon detects tunnel_enabled=true in config

Use 'roamie tunnel disable' to stop the daemon-managed tunnel.`,
	Run: runTunnelStart,
}

var tunnelStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop SSH reverse tunnel",
	Run:   runTunnelStop,
}

var tunnelStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSH tunnel status",
	Run:   runTunnelStatus,
}

var tunnelRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register SSH key and allocate tunnel port",
	Run:   runTunnelRegister,
}

var tunnelDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable SSH tunnel (daemon will stop it within 10 seconds)",
	Long: `Disable the SSH tunnel managed by the daemon.

This sets tunnel_enabled=false in the config file.
The daemon polls the config every 10 seconds and will stop the tunnel automatically.

To re-enable the tunnel, run: roamie tunnel register`,
	Run: runTunnelDisable,
}

var tunnelEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable SSH tunnel (daemon will start it within 10 seconds)",
	Long: `Enable the SSH tunnel managed by the daemon.

This sets tunnel_enabled=true in the config file.
The daemon polls the config every 10 seconds and will start the tunnel automatically.

Note: You must have registered the tunnel first with 'roamie tunnel register'.`,
	Run: runTunnelEnable,
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade roamie to the latest version",
	Long: `Download and install the latest version of roamie from GitHub Releases.

The upgrade process:
  1. Checks for newer version on GitHub
  2. Downloads the binary for your platform
  3. Verifies SHA256 checksum
  4. Backs up current binary
  5. Installs new version
  6. Restarts daemon if running (use --no-restart to skip)

Examples:
  roamie upgrade              # Upgrade and restart daemon automatically
  roamie upgrade --no-restart # Upgrade but don't restart daemon
  roamie upgrade --force      # Reinstall even if already on latest version`,
	Run: runUpgrade,
}

var upgradeCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if a new version is available",
	Long: `Check GitHub Releases for a newer version of roamie.

Shows current version, latest available version, and release notes.
Does not download or install anything.

Example:
  roamie upgrade check`,
	Run: runUpgradeCheck,
}

var upgradeForce bool
var upgradeNoRestart bool

var vpnCmd = &cobra.Command{
	Use:   "vpn",
	Short: "VPN management commands",
}

var vpnInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install WireGuard and enable VPN mode",
	Long: `Install WireGuard and enable VPN mode.

If you chose SSH Tunnel only during login, you can use this command
to install WireGuard and enable VPN later.

After installation, use 'roamie connect' to connect to the VPN.`,
	Run: runVPNInstall,
}

var vpnStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show VPN status",
	Run:   runVPNStatus,
}

func init() {
	setupDaemonCmd.Flags().BoolVarP(&setupDaemonYes, "yes", "y", false, "Skip confirmation prompt")
	upgradeCmd.Flags().BoolVarP(&upgradeForce, "force", "f", false, "Force upgrade even if already on latest version")
	upgradeCmd.Flags().BoolVar(&upgradeNoRestart, "no-restart", false, "Do not restart daemon after upgrade")
	upgradeCmd.AddCommand(upgradeCheckCmd)
	authCmd.AddCommand(loginCmd, daemonCmd, statusCmd, refreshCmd, logoutCmd)
	sshCmd.AddCommand(sshSyncCmd, sshStatusCmd, sshEnableCmd, sshDisableCmd, sshSetIntervalCmd)
	tunnelCmd.AddCommand(tunnelStartCmd, tunnelStopCmd, tunnelStatusCmd, tunnelRegisterCmd, tunnelDisableCmd, tunnelEnableCmd)
	vpnCmd.AddCommand(vpnInstallCmd, vpnStatusCmd)
	rootCmd.AddCommand(authCmd, sshCmd, tunnelCmd, vpnCmd, setupDaemonCmd, uninstallDaemonCmd, versionCmd, connectCmd, disconnectCmd, upgradeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runLogin(cmd *cobra.Command, args []string) {
	// Default to public HTTP endpoint for initial connection
	// After VPN is connected, use internal gateway
	serverURL := "http://felipenovaesrocha.xyz:8081"

	// Check environment variable first
	if envURL := os.Getenv("VPN_SERVER"); envURL != "" {
		serverURL = envURL
	}

	// Command line argument overrides all
	if len(args) > 0 {
		serverURL = args[0]
	}

	if err := auth.Login(serverURL); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	if err := daemon.Run(ctx); err != nil {
		fmt.Printf("Daemon failed: %v\n", err)
		os.Exit(1)
	}
}

func runStatus(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Status: Not logged in")
		fmt.Println("\nRun 'roamie auth login' to authenticate")
		return
	}

	fmt.Println("Roamie VPN Client - Status")
	fmt.Println("==========================")
	fmt.Printf("Server:       %s\n", cfg.ServerURL)
	fmt.Printf("Device ID:    %s\n", cfg.DeviceID)
	fmt.Printf("Created:      %s\n", cfg.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Expires:      %s\n", cfg.ExpiresAt.Format("2006-01-02 15:04:05"))

	expiresIn := cfg.ExpiresIn()
	if cfg.IsExpired() {
		fmt.Printf("Status:       ❌ EXPIRED\n")
		fmt.Println("\nRun 'roamie auth refresh' to renew")
	} else if expiresIn < 24*time.Hour {
		fmt.Printf("Status:       ⚠️  EXPIRES SOON (%s)\n", expiresIn.Round(time.Hour))
		fmt.Println("\nRun 'roamie auth refresh' to renew")
	} else {
		fmt.Printf("Status:       ✓ VALID (%s remaining)\n", expiresIn.Round(time.Hour))
	}
}

func runRefresh(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		fmt.Println("Error: Not logged in")
		os.Exit(1)
	}

	fmt.Println("Refreshing JWT token...")

	client := api.NewClient(cfg.ServerURL)
	resp, err := client.RefreshJWT(cfg.RefreshToken)
	if err != nil {
		fmt.Printf("Refresh failed: %v\n", err)
		os.Exit(1)
	}

	expiresAt, _ := time.Parse("2006-01-02T15:04:05Z", resp.ExpiresAt)

	cfg.JWT = resp.JWT
	cfg.ExpiresAt = expiresAt

	if err := cfg.Save(); err != nil {
		fmt.Printf("Failed to save: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ JWT refreshed (expires: %s)\n", expiresAt.Format("2006-01-02 15:04:05"))
}

func runLogout(cmd *cobra.Command, args []string) {
	// Step 1: Try to delete device from server
	cfg, err := config.Load()
	if err == nil && cfg != nil {
		// We have config, try to delete device from server
		fmt.Println("→ Removing device from server...")
		client := api.NewClient(cfg.ServerURL)
		if err := client.DeleteDevice(cfg.DeviceID, cfg.JWT); err != nil {
			fmt.Printf("⚠️  Warning: Failed to remove device from server: %v\n", err)
			fmt.Println("   (Device may still exist on server)")
		} else {
			fmt.Println("✓ Device removed from server")
		}
	}

	// Step 2: Stop daemon if running
	fmt.Println("→ Stopping daemon...")
	exec.Command("systemctl", "stop", "roamie").Run()
	exec.Command("systemctl", "disable", "roamie").Run()
	fmt.Println("✓ Daemon stopped")

	// Step 3: Disconnect VPN if connected
	fmt.Println("→ Disconnecting VPN...")
	if err := wireguard.Disconnect("roamie"); err != nil {
		// Check if error is because interface doesn't exist
		if !strings.Contains(err.Error(), "does not exist") &&
			!strings.Contains(err.Error(), "No such device") {
			fmt.Printf("⚠️  Warning: Failed to disconnect VPN: %v\n", err)
		}
	} else {
		fmt.Println("✓ VPN disconnected")
	}

	// Step 4: Clean local files
	fmt.Println("→ Cleaning local files...")

	// Remove config
	if err := config.Delete(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("⚠️  Warning: Failed to remove config: %v\n", err)
	}

	// Remove device info directory
	configDir, _ := config.GetConfigDir()
	devicesDir := filepath.Join(strings.Replace(configDir, ".roamie", ".roamie-vpn", 1), "devices")
	os.RemoveAll(devicesDir)

	// Remove WireGuard config
	wgConfigPath := wireguard.GetWireGuardConfigPath("roamie")
	os.Remove(wgConfigPath)

	fmt.Println("✓ Logged out successfully")
}

func runSetupDaemon(cmd *cobra.Command, args []string) {
	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to detect binary path: %v\n", err)
		os.Exit(1)
	}

	// Get actual user (detects SUDO_USER automatically)
	username, homeDir, err := utils.GetActualUser()
	if err != nil {
		fmt.Printf("Failed to get user info: %v\n", err)
		os.Exit(1)
	}

	// Setup platform-specific daemon service
	cfg := daemon.ServiceConfig{
		ExePath:  exePath,
		Username: username,
		HomeDir:  homeDir,
	}

	if err := daemon.SetupService(cfg, setupDaemonYes); err != nil {
		fmt.Printf("Failed to setup daemon: %v\n", err)
		os.Exit(1)
	}
}

func runUninstallDaemon(cmd *cobra.Command, args []string) {
	// Check if service is installed
	if !daemon.IsServiceInstalled() {
		fmt.Println("Service not installed")
		return
	}

	fmt.Println("Uninstalling daemon service...")
	fmt.Print("\nUninstall the service? [y/N]: ")

	var response string
	fmt.Scanln(&response)

	if response != "y" && response != "Y" {
		fmt.Println("Cancelled")
		return
	}

	if err := daemon.UninstallService(); err != nil {
		fmt.Printf("Failed to uninstall daemon: %v\n", err)
		os.Exit(1)
	}
}

func runSSHSync(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated. Please run 'roamie auth login' first")
		os.Exit(1)
	}

	if cfg.JWT == "" {
		fmt.Println("Error: Not authenticated")
		os.Exit(1)
	}

	// Create SSH manager
	sshManager, err := ssh.NewManager(cfg.ServerURL)
	if err != nil {
		fmt.Printf("Error: Failed to create SSH manager: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Syncing SSH keys from Firestore...")

	// Sync keys
	result, err := sshManager.SyncKeys(cfg)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Display results
	if len(result.Added) > 0 {
		fmt.Printf("✓ Added %d key(s)\n", len(result.Added))
	}
	if len(result.Removed) > 0 {
		fmt.Printf("✓ Removed %d key(s)\n", len(result.Removed))
	}
	if len(result.Added) == 0 && len(result.Removed) == 0 {
		fmt.Println("✓ No changes (already in sync)")
	}
	fmt.Printf("✓ Total keys: %d\n", result.Total)
}

func runSSHStatus(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated")
		os.Exit(1)
	}

	fmt.Println("SSH Sync Status")
	fmt.Println("===============")
	fmt.Printf("Enabled:  %v\n", cfg.SSHSyncEnabled)
	fmt.Printf("Interval: %v\n", cfg.SSHSyncInterval)

	// Get current keys
	sshManager, err := ssh.NewManager(cfg.ServerURL)
	if err != nil {
		fmt.Printf("\nError getting current keys: %v\n", err)
		return
	}

	keys, err := sshManager.GetCurrentKeys()
	if err != nil {
		fmt.Printf("\nError reading authorized_keys: %v\n", err)
		return
	}

	fmt.Printf("\nCurrent Roamie-managed keys: %d\n", len(keys))
	if len(keys) > 0 {
		fmt.Println("\nKeys:")
		for i, key := range keys {
			// Show first 50 chars of key
			keyPreview := key
			if len(keyPreview) > 50 {
				keyPreview = keyPreview[:50] + "..."
			}
			fmt.Printf("  %d. %s\n", i+1, keyPreview)
		}
	}
}

func runSSHEnable(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated")
		os.Exit(1)
	}

	cfg.SSHSyncEnabled = true
	if err := cfg.Save(); err != nil {
		fmt.Printf("Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ SSH sync enabled")
	fmt.Println("\nNote: Restart the daemon for changes to take effect:")
	fmt.Println("  sudo systemctl restart roamie-client")
}

func runSSHDisable(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated")
		os.Exit(1)
	}

	cfg.SSHSyncEnabled = false
	if err := cfg.Save(); err != nil {
		fmt.Printf("Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ SSH sync disabled")
}

func runSSHSetInterval(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated")
		os.Exit(1)
	}

	// Parse duration
	duration, err := time.ParseDuration(args[0])
	if err != nil {
		fmt.Printf("Error: Invalid duration format. Use formats like: 5m, 1h, 30s\n")
		os.Exit(1)
	}

	cfg.SSHSyncInterval = duration
	if err := cfg.Save(); err != nil {
		fmt.Printf("Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ SSH sync interval set to %v\n", duration)
	fmt.Println("\nNote: Restart the daemon for changes to take effect:")
	fmt.Println("  sudo systemctl restart roamie-client")
}

func runConnect(cmd *cobra.Command, args []string) {
	// Check if running as root
	if os.Geteuid() != 0 {
		fmt.Println("Error: This command requires root privileges")
		fmt.Println("Please run: sudo roamie connect")
		os.Exit(1)
	}

	// Pre-flight check: Ensure WireGuard is installed
	if !wireguard.CheckInstalled() {
		fmt.Println("Error: WireGuard is not installed")
		fmt.Println()
		fmt.Println(wireguard.GetInstallInstructions())
		os.Exit(1)
	}

	// Try to migrate from legacy storage first
	config.MigrateFromLegacyStorage()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not logged in")
		fmt.Println("Please run 'roamie auth login' first (without sudo)")
		os.Exit(1)
	}

	// Check if device info is available
	if cfg.PrivateKey == "" || cfg.VpnIP == "" {
		fmt.Println("Error: No device registration found")
		fmt.Println("Please run 'roamie auth login' first")
		os.Exit(1)
	}

	fmt.Println("Connecting to VPN...")
	fmt.Printf("  Device: %s\n", cfg.DeviceName)
	fmt.Printf("  VPN IP: %s\n", cfg.VpnIP)

	// Prepare WireGuard config
	wgConfig := wireguard.WireGuardConfig{
		PrivateKey: cfg.PrivateKey,
		Address:    cfg.VpnIP,
		ServerKey:  cfg.ServerPublicKey,
		Endpoint:   cfg.ServerEndpoint,
		AllowedIPs: cfg.AllowedIPs,
	}

	// Connect (generates config file and connects)
	if err := wireguard.Connect("roamie", wgConfig); err != nil {
		fmt.Printf("Error: Failed to connect: %v\n", err)
		os.Exit(1)
	}

	configPath := wireguard.GetWireGuardConfigPath("roamie")
	fmt.Println("\n✅ Successfully connected to VPN!")
	fmt.Printf("   Config: %s\n", configPath)
	fmt.Printf("   Interface: roamie\n")
	fmt.Printf("   VPN IP: %s\n", cfg.VpnIP)
	fmt.Println("\nUseful commands:")
	fmt.Println("  • Check status: sudo wg show roamie")
	fmt.Println("  • Disconnect: sudo roamie disconnect")
	fmt.Println("  • Or: sudo wg-quick down roamie")
}

func runDisconnect(cmd *cobra.Command, args []string) {
	// Check if running as root
	if os.Geteuid() != 0 {
		fmt.Println("Error: This command requires root privileges")
		fmt.Println("Please run: sudo roamie disconnect")
		os.Exit(1)
	}

	fmt.Println("Disconnecting from VPN...")

	// Disconnect
	if err := wireguard.Disconnect("roamie"); err != nil {
		fmt.Printf("Error: Failed to disconnect: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Disconnected from VPN")
}

// Tunnel command implementations

func runTunnelRegister(cmd *cobra.Command, args []string) {
	// Load config
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		fmt.Println("Error: Not authenticated. Please run 'roamie auth login' first.")
		os.Exit(1)
	}

	// Pre-flight check: Ensure SSH daemon is available
	fmt.Println("→ Checking SSH server availability...")
	sshdAvailable, err := sshd.PromptInstall()
	if err != nil {
		fmt.Printf("Error: SSH check failed: %v\n", err)
		os.Exit(1)
	}
	if !sshdAvailable {
		fmt.Println("\nSSH server is required for the tunnel to work.")
		fmt.Println("Please install and start SSH server, then try again.")
		os.Exit(1)
	}
	fmt.Println("✓ SSH server is available")

	fmt.Println("\nRegistering SSH tunnel...")

	// Create tunnel client
	tunnelClient, err := tunnel.NewClient(cfg)
	if err != nil {
		fmt.Printf("Error: Failed to initialize tunnel client: %v\n", err)
		os.Exit(1)
	}

	// Register SSH key with server
	fmt.Println("Registering SSH public key...")
	if err := tunnelClient.RegisterKey(); err != nil {
		fmt.Printf("Error: Failed to register SSH key: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ SSH key registered")

	// Allocate tunnel port
	fmt.Println("Allocating tunnel port...")
	apiClient := api.NewClient(cfg.ServerURL)
	registerResp, err := apiClient.RegisterTunnel(cfg.DeviceID, cfg.JWT)
	if err != nil {
		fmt.Printf("Error: Failed to allocate tunnel port: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Tunnel port allocated: %d\n", registerResp.TunnelPort)

	// Enable tunnel on server
	fmt.Println("Enabling tunnel...")
	if err := apiClient.EnableTunnel(cfg.DeviceID, cfg.JWT); err != nil {
		fmt.Printf("Error: Failed to enable tunnel: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Tunnel enabled on server")

	// Save tunnel config locally
	cfg.TunnelEnabled = true
	cfg.TunnelPort = registerResp.TunnelPort
	if err := cfg.Save(); err != nil {
		fmt.Printf("Warning: Failed to save tunnel config: %v\n", err)
	} else {
		fmt.Println("✓ Tunnel config saved")
	}

	fmt.Println("\n✅ SSH tunnel registered successfully!")
	fmt.Println("\nThe daemon will start the tunnel automatically within 10 seconds.")
	fmt.Println("\nUseful commands:")
	fmt.Println("  • Check status: roamie tunnel status")
	fmt.Println("  • Disable tunnel: roamie tunnel disable")
	fmt.Println("  • Manual start (debug): roamie tunnel start")
}

func runTunnelStart(cmd *cobra.Command, args []string) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		fmt.Println("Please run 'roamie auth login' first.")
		os.Exit(1)
	}
	if cfg == nil {
		fmt.Println("Error: Not authenticated. Please run 'roamie auth login' first.")
		os.Exit(1)
	}

	fmt.Println("Starting SSH tunnel...")

	// Create tunnel client
	tunnelClient, err := tunnel.NewClient(cfg)
	if err != nil {
		fmt.Printf("Error: Failed to initialize tunnel client: %v\n", err)
		os.Exit(1)
	}

	// Connect tunnel
	if err := tunnelClient.Connect(); err != nil {
		fmt.Printf("Error: Failed to start tunnel: %v\n", err)
		fmt.Println("\nTip: Make sure you've registered first with: roamie tunnel register")
		os.Exit(1)
	}

	fmt.Println("✅ SSH tunnel started successfully!")
	fmt.Println("\nThe tunnel will maintain connection with auto-reconnect.")
	fmt.Println("Press Ctrl+C to stop.")

	// Wait for interrupt
	select {}
}

func runTunnelStop(cmd *cobra.Command, args []string) {
	fmt.Println("Note: Tunnel processes should be stopped by interrupting 'roamie tunnel start'")
	fmt.Println("Or managed via the daemon: roamie daemon")
}

func runTunnelStatus(cmd *cobra.Command, args []string) {
	// Load config
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		fmt.Println("Error: Not authenticated. Please run 'roamie auth login' first.")
		os.Exit(1)
	}

	apiClient := api.NewClient(cfg.ServerURL)
	status, err := apiClient.GetTunnelStatus(cfg.JWT)
	if err != nil {
		fmt.Printf("Error: Failed to get tunnel status: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("SSH Tunnel Status")
	fmt.Println("=================")
	fmt.Printf("Local config: tunnel_enabled=%v, tunnel_port=%d\n", cfg.TunnelEnabled, cfg.TunnelPort)

	if len(status.Tunnels) == 0 {
		fmt.Println("\nNo tunnels registered on server.")
		fmt.Println("Run: roamie tunnel register")
		return
	}

	for _, t := range status.Tunnels {
		if t.DeviceID == cfg.DeviceID {
			fmt.Printf("\nDevice: %s\n", t.DeviceName)
			fmt.Printf("Port: %d\n", t.Port)
			fmt.Printf("Server enabled: %v\n", t.Enabled)
			fmt.Printf("Connected: %v\n", t.Connected)

			if cfg.TunnelEnabled && t.Enabled {
				fmt.Println("\n✓ Tunnel is enabled (daemon will manage it)")
			} else if !cfg.TunnelEnabled {
				fmt.Println("\n⚠️  Tunnel is disabled in local config")
				fmt.Println("Enable with: roamie tunnel enable")
			} else {
				fmt.Println("\n⚠️  Tunnel is disabled on server")
				fmt.Println("Enable with: roamie tunnel register")
			}
			return
		}
	}

	fmt.Println("\nTunnel not registered for this device.")
	fmt.Println("Run: roamie tunnel register")
}

func runTunnelDisable(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated. Please run 'roamie auth login' first.")
		os.Exit(1)
	}

	cfg.TunnelEnabled = false
	if err := cfg.Save(); err != nil {
		fmt.Printf("Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Tunnel disabled")
	fmt.Println("\nThe daemon will stop the tunnel within 10 seconds.")
	fmt.Println("To re-enable: roamie tunnel enable")
}

func runTunnelEnable(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not authenticated. Please run 'roamie auth login' first.")
		os.Exit(1)
	}

	// Check if tunnel port is allocated
	if cfg.TunnelPort == 0 {
		fmt.Println("Error: Tunnel not registered. Please run 'roamie tunnel register' first.")
		os.Exit(1)
	}

	cfg.TunnelEnabled = true
	if err := cfg.Save(); err != nil {
		fmt.Printf("Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Tunnel enabled")
	fmt.Printf("  Port: %d\n", cfg.TunnelPort)
	fmt.Println("\nThe daemon will start the tunnel within 10 seconds.")
	fmt.Println("To disable: roamie tunnel disable")
}

// Upgrade command implementations

func runUpgradeCheck(cmd *cobra.Command, args []string) {
	fmt.Println("Checking for updates...")

	result, err := upgrade.CheckForUpdates()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nCurrent version: %s\n", result.CurrentVersion)
	fmt.Printf("Latest version:  %s\n", result.LatestVersion)

	if !result.UpdateAvailable {
		fmt.Println("\n✓ You are running the latest version!")
		return
	}

	fmt.Println("\n🆕 A new version is available!")

	if result.ReleaseNotes != "" {
		fmt.Println("\nRelease notes:")
		fmt.Println("─────────────────────────────────")
		// Truncate long release notes
		notes := result.ReleaseNotes
		if len(notes) > 500 {
			notes = notes[:500] + "...\n\nSee full notes at: " + result.ReleaseURL
		}
		fmt.Println(notes)
		fmt.Println("─────────────────────────────────")
	}

	fmt.Println("\nRun 'roamie upgrade' to update.")
}

func runUpgrade(cmd *cobra.Command, args []string) {
	fmt.Println("Checking for updates...")

	result, err := upgrade.CheckForUpdates()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nCurrent version: %s\n", result.CurrentVersion)
	fmt.Printf("Latest version:  %s\n", result.LatestVersion)

	if !result.UpdateAvailable && !upgradeForce {
		fmt.Println("\n✓ You are running the latest version!")
		fmt.Println("Use --force to reinstall anyway.")
		return
	}

	if result.DownloadURL == "" {
		fmt.Printf("\nError: No compatible binary found for your platform.\n")
		fmt.Printf("Please download manually from: %s\n", result.ReleaseURL)
		os.Exit(1)
	}

	fmt.Printf("\nUpgrading to %s...\n\n", result.LatestVersion)

	if err := upgrade.Upgrade(result); err != nil {
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Upgrade successful!")
	fmt.Printf("Now running: %s\n", result.LatestVersion)

	// Check if daemon is running and restart it (unless --no-restart flag is set)
	if isServiceRunning("roamie") {
		if upgradeNoRestart {
			fmt.Println("\nDaemon is running. Skipping restart (--no-restart flag set).")
			fmt.Println("Restart manually: sudo systemctl restart roamie")
		} else {
			fmt.Println("\nRestarting daemon...")
			if err := exec.Command("systemctl", "restart", "roamie").Run(); err != nil {
				fmt.Printf("Warning: Failed to restart daemon: %v\n", err)
				fmt.Println("Please restart manually: sudo systemctl restart roamie")
			} else {
				fmt.Println("✓ Daemon restarted")
			}
		}
	}
}

func isServiceRunning(name string) bool {
	err := exec.Command("systemctl", "is-active", "--quiet", name).Run()
	return err == nil
}

func runVPNInstall(cmd *cobra.Command, args []string) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Error: Not logged in")
		fmt.Println("Please run 'roamie auth login' first")
		os.Exit(1)
	}

	// Check if VPN is already enabled
	if cfg.VPNEnabled && wireguard.CheckInstalled() {
		fmt.Println("✓ VPN is already enabled and WireGuard is installed")
		fmt.Println("\nTo connect: sudo roamie connect")
		return
	}

	fmt.Println("Installing WireGuard for VPN mode...")
	fmt.Println()

	// Try to install WireGuard
	installed, err := wireguard.PromptInstall()
	if err != nil {
		fmt.Printf("Error: WireGuard installation failed: %v\n", err)
		os.Exit(1)
	}

	if !installed {
		fmt.Println("WireGuard installation cancelled or failed")
		os.Exit(1)
	}

	// Update config
	cfg.VPNEnabled = true
	if err := cfg.Save(); err != nil {
		fmt.Printf("Error: Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ VPN mode enabled!")
	fmt.Println("\nNext steps:")
	fmt.Println("  • Connect to VPN: sudo roamie connect")
	fmt.Println("  • Check status: roamie vpn status")
}

func runVPNStatus(cmd *cobra.Command, args []string) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg == nil {
		fmt.Println("Status: Not logged in")
		fmt.Println("\nRun 'roamie auth login' to authenticate")
		return
	}

	fmt.Println("VPN Status")
	fmt.Println("==========")

	// VPN enabled in config?
	if cfg.VPNEnabled {
		fmt.Println("VPN Mode: Enabled")
	} else {
		fmt.Println("VPN Mode: Disabled (SSH Tunnel only)")
		fmt.Println("\nTo enable VPN: roamie vpn install")
		return
	}

	// WireGuard installed?
	if wireguard.CheckInstalled() {
		fmt.Println("WireGuard: Installed")
	} else {
		fmt.Println("WireGuard: Not installed")
		fmt.Println("\nTo install: roamie vpn install")
		return
	}

	// Device registered?
	if cfg.VpnIP != "" {
		fmt.Printf("VPN IP: %s\n", cfg.VpnIP)
		fmt.Printf("Device: %s\n", cfg.DeviceName)
	} else {
		fmt.Println("Device: Not registered")
	}

	// Connection status (requires root)
	if os.Geteuid() == 0 {
		status, _ := wireguard.GetStatus("roamie")
		if status != "" {
			fmt.Println("\nConnection:")
			fmt.Println(status)
		} else {
			fmt.Println("\nConnection: Disconnected")
			fmt.Println("  Connect with: sudo roamie connect")
		}
	} else {
		fmt.Println("\n(Run with sudo to see connection status)")
	}
}
