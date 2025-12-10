package auth

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
	"github.com/kamikazebr/roamie-desktop/internal/client/sshd"
	"github.com/kamikazebr/roamie-desktop/internal/client/tunnel"
	"github.com/kamikazebr/roamie-desktop/internal/client/ui"
	"github.com/kamikazebr/roamie-desktop/internal/client/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

func Login(serverURL string) error {
	// Use default if not provided
	if serverURL == "" {
		serverURL = "http://10.100.0.1:8081"
	}

	fmt.Println("Roamie VPN Client - Login")
	fmt.Println("=========================")

	// SSH Tunnel is always available - VPN is optional
	fmt.Println("\n→ SSH Tunnel will be enabled automatically.")
	fmt.Println("  VPN provides full network encryption but requires WireGuard.")

	enableVPN, err := ui.Confirm("Also enable VPN? (You can install later with 'roamie vpn install')")
	if err != nil {
		enableVPN = false // Default to tunnel-only on cancel/error
	}

	if enableVPN {
		fmt.Println("\n→ Checking WireGuard...")
		installed, err := wireguard.PromptInstall()
		if err != nil || !installed {
			fmt.Println("⚠️  WireGuard not installed. Continuing with SSH Tunnel only.")
			fmt.Println("   You can install VPN later: roamie vpn install")
			enableVPN = false
		} else {
			fmt.Println("✓ WireGuard is available")
		}
	}

	// Generate device ID
	deviceID := uuid.New()
	hostname, _ := os.Hostname()

	// Get system username for SSH host creation
	// Check SUDO_USER first (when running with sudo), then fall back to USER
	username := os.Getenv("SUDO_USER")
	if username == "" {
		username = os.Getenv("USER")
	}
	if username == "" {
		username = os.Getenv("USERNAME") // Windows fallback
	}

	// Detect OS and hardware ID
	osType := utils.DetectOS()
	hardwareID := utils.GetHardwareID()

	fmt.Printf("Device ID: %s\n", deviceID)
	fmt.Printf("Hostname: %s\n", hostname)
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("OS: %s\n", osType)
	fmt.Printf("Hardware ID: %s\n", hardwareID)
	fmt.Printf("Server: %s\n\n", serverURL)

	// Generate WireGuard keypair for auto-registration
	fmt.Println("→ Generating WireGuard keys...")
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate WireGuard keys: %w", err)
	}
	fmt.Println("✓ Keys generated")

	// Create API client
	client := api.NewClient(serverURL)

	// Request device challenge with public key, username, os_type, and hardware_id
	fmt.Println("→ Requesting device authorization...")
	challenge, err := client.CreateDeviceRequest(deviceID.String(), hostname, username, publicKey, osType, hardwareID)
	if err != nil {
		return fmt.Errorf("failed to create challenge: %w", err)
	}

	fmt.Printf("✓ Challenge created: %s\n", challenge.ChallengeID)
	fmt.Printf("✓ Expires in: %d seconds\n\n", challenge.ExpiresIn)

	// Display QR code
	fmt.Println("Scan this QR code with Roamie app:")
	fmt.Println()
	displayQRCode(challenge.QRData)

	fmt.Printf("\nOr open this URL manually:\n%s\n\n", challenge.QRData)
	fmt.Println("Waiting for authorization...")

	// Poll for approval with private key for auto-connection
	return pollForApproval(client, challenge.ChallengeID, deviceID.String(), privateKey, publicKey, serverURL, enableVPN)
}

func displayQRCode(data string) {
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		fmt.Println("Failed to generate QR code:", err)
		return
	}

	// Print as ASCII art (false = inverted colors for better visibility in terminal)
	fmt.Println(qr.ToSmallString(false))
}

func pollForApproval(client *api.Client, challengeID, deviceID, privateKey, publicKey, serverURL string, enableVPN bool) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout: authorization not received within 5 minutes")

		case <-ticker.C:
			resp, err := client.PollChallenge(challengeID)
			if err != nil {
				fmt.Printf("Poll error: %v\n", err)
				continue
			}

			switch resp.Status {
			case "approved":
				fmt.Println("\n✓ Device authorized!")

				// Parse expires_at
				expiresAt, _ := time.Parse("2006-01-02T15:04:05Z", resp.ExpiresAt)

				// Save config
				cfg := &config.Config{
					ServerURL:       serverURL,
					DeviceID:        deviceID,
					JWT:             resp.JWT,
					RefreshToken:    resp.RefreshToken,
					ExpiresAt:       expiresAt,
					CreatedAt:       time.Now(),
					SSHSyncEnabled:  true,             // Enable SSH sync by default
					SSHSyncInterval: 5 * time.Minute,  // Default 5 minute interval
					VPNEnabled:      enableVPN,        // User's VPN choice
				}

				// Check if device was auto-registered and save device info
				if resp.AutoRegistered && resp.Device != nil {
					cfg.DeviceName = resp.Device.DeviceName
					cfg.PrivateKey = privateKey
					cfg.PublicKey = publicKey
					cfg.VpnIP = resp.Device.VpnIP
					cfg.Subnet = resp.AllowedIPs
					cfg.ServerPublicKey = resp.ServerPublicKey
					cfg.ServerEndpoint = resp.ServerEndpoint
					cfg.AllowedIPs = resp.AllowedIPs
				}

				if err := cfg.Save(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				configDir, _ := config.GetConfigDir()
				fmt.Printf("✓ Configuration saved to %s\n", configDir)
				fmt.Printf("✓ JWT expires: %s (%s)\n",
					expiresAt.Format("2006-01-02 15:04:05"),
					time.Until(expiresAt).Round(time.Hour),
				)

				// Show device registration info
				if resp.AutoRegistered && resp.Device != nil {
					fmt.Println("\n✓ Device automatically registered in VPN!")
					fmt.Printf("  Device Name: %s\n", resp.Device.DeviceName)
					fmt.Printf("  VPN IP: %s\n", resp.Device.VpnIP)

					// Auto-register SSH tunnel
					tunnelPort, err := autoRegisterTunnel(cfg)
					if err != nil {
						fmt.Printf("\n⚠️  Failed to register SSH tunnel: %v\n", err)
						fmt.Println("You can manually register with: roamie tunnel register")
					} else {
						cfg.TunnelEnabled = true
						cfg.TunnelPort = tunnelPort
						if err := cfg.Save(); err != nil {
							fmt.Printf("⚠️  Failed to save tunnel config: %v\n", err)
						} else {
							fmt.Printf("✓ SSH tunnel registered (port %d)\n", tunnelPort)
						}
					}

					// VPN auto-connect only if user chose VPN mode
					if enableVPN {
						if os.Geteuid() == 0 {
							// Auto-connect to VPN when running with sudo
							if err := autoConnectVPN(cfg); err != nil {
								fmt.Printf("\n⚠️  Failed to auto-connect to VPN: %v\n", err)
								fmt.Println("You can manually connect with: sudo roamie connect")
							}
						} else {
							// Show manual connection instructions when not using sudo
							fmt.Println("\nVPN mode enabled. Next steps:")
							fmt.Println("  • Connect to VPN: sudo roamie connect")
							fmt.Println("  • Or manually: sudo wg-quick up roamie")
						}
					} else {
						fmt.Println("\n✓ SSH Tunnel mode - VPN not enabled")
						fmt.Println("  To enable VPN later: roamie vpn install")
					}
					// Always setup daemon (works without sudo for user systemd)
					autoSetupDaemon()
				} else {
					// No auto-registration - still setup daemon
					autoSetupDaemon()
					fmt.Println("\nNext steps:")
					fmt.Println("  1. Check status: roamie auth status")
				}

				return nil

			case "denied":
				return fmt.Errorf("authorization denied by user")

			case "expired":
				return fmt.Errorf("authorization request expired")

			case "pending":
				// Continue polling
				fmt.Print(".")
			}
		}
	}
}

// autoConnectVPN automatically connects to the VPN using saved configuration
// This is called when login is run with sudo
func autoConnectVPN(cfg *config.Config) error {
	fmt.Println("\n→ Connecting to VPN...")

	wgConfig := wireguard.WireGuardConfig{
		PrivateKey: cfg.PrivateKey,
		Address:    cfg.VpnIP,
		ServerKey:  cfg.ServerPublicKey,
		Endpoint:   cfg.ServerEndpoint,
		AllowedIPs: cfg.AllowedIPs,
	}

	if err := wireguard.Connect("roamie", wgConfig); err != nil {
		return err
	}

	configPath := wireguard.GetWireGuardConfigPath("roamie")
	fmt.Println("\n✅ Successfully connected to VPN!")
	fmt.Printf("   Config: %s\n", configPath)
	fmt.Printf("   Interface: roamie\n")
	fmt.Printf("   VPN IP: %s\n", cfg.VpnIP)
	fmt.Println("\nUseful commands:")
	fmt.Println("  • Check status: sudo wg show roamie")
	fmt.Println("  • Disconnect: sudo roamie disconnect")

	return nil
}

// autoSetupDaemon automatically sets up the systemd daemon service
func autoSetupDaemon() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("\n⚠️  Failed to setup daemon: %v\n", err)
		return
	}

	fmt.Println("\n→ Setting up auto-refresh daemon...")
	cmd := exec.Command(exePath, "setup-daemon", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to setup daemon: %v\n", err)
		fmt.Println("You can manually setup with: sudo roamie setup-daemon -y")
	}
}

// autoRegisterTunnel registers the SSH tunnel key and allocates a port
// Returns the allocated tunnel port on success
func autoRegisterTunnel(cfg *config.Config) (int, error) {
	// Pre-flight check: Ensure SSH daemon is available
	fmt.Println("\n→ Checking SSH server availability...")
	if !sshd.IsRunning() {
		fmt.Println("⚠️  SSH server (sshd) is not running on this machine.")
		fmt.Println("   The SSH tunnel requires sshd to accept incoming connections.")
		fmt.Println()
		fmt.Println(sshd.GetInstallInstructions())
		fmt.Println()
		return 0, fmt.Errorf("SSH server not available - install and start sshd, then run 'roamie tunnel register'")
	}
	fmt.Println("✓ SSH server is available")

	fmt.Println("\n→ Registering SSH tunnel...")

	// Create tunnel client to generate/load SSH key
	tunnelClient, err := tunnel.NewClient(cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize tunnel client: %w", err)
	}

	// Register SSH key with server
	if err := tunnelClient.RegisterKey(); err != nil {
		return 0, fmt.Errorf("failed to register SSH key: %w", err)
	}

	// Allocate tunnel port
	apiClient := api.NewClient(cfg.ServerURL)
	registerResp, err := apiClient.RegisterTunnel(cfg.DeviceID, cfg.JWT)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate tunnel port: %w", err)
	}

	// Enable tunnel on server
	if err := apiClient.EnableTunnel(cfg.DeviceID, cfg.JWT); err != nil {
		return 0, fmt.Errorf("failed to enable tunnel: %w", err)
	}

	return registerResp.TunnelPort, nil
}
