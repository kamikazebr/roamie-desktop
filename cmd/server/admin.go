package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/internal/server/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands",
	Long:  "Administrative commands for managing devices, users, and WireGuard peers",
}

var addDeviceCmd = &cobra.Command{
	Use:   "add-device",
	Short: "Register a new device for a user",
	Run:   runAddDeviceCommand,
}

var deleteDeviceCmd = &cobra.Command{
	Use:   "delete-device",
	Short: "Delete a device by name",
	Run:   runDeleteDeviceCommand,
}

var listDevicesCmd = &cobra.Command{
	Use:   "list-devices",
	Short: "List all devices for a user",
	Run:   runListDevicesCommand,
}

var listPeersCmd = &cobra.Command{
	Use:   "list-peers",
	Short: "List WireGuard peers (actual interface config)",
	Run:   runListPeersCommand,
}

var syncPeersCmd = &cobra.Command{
	Use:   "sync-peers",
	Short: "Remove orphaned WireGuard peers not in database",
	Run:   runSyncPeersCommand,
}

var listChallengesCmd = &cobra.Command{
	Use:   "list-challenges",
	Short: "List pending device authorization challenges",
	Run:   runListChallengesCommand,
}

var approveDeviceCmd = &cobra.Command{
	Use:   "approve-device",
	Short: "Approve a device challenge (for testing)",
	Run:   runApproveDeviceCommand,
}

var validateKeyDecryptionCmd = &cobra.Command{
	Use:   "validate-key-decryption",
	Short: "Validate encrypted SSH keys can be decrypted from Firestore",
	Long:  "Validates that encrypted SSH private keys stored in Firestore can be decrypted with the provided password. Does not expose private keys, only confirms decryption success.",
	Run:   runValidateKeyDecryptionCommand,
}

var listFirestoreDataCmd = &cobra.Command{
	Use:   "list-firestore-data",
	Short: "List Firestore data for a user",
	Long:  "Shows what data exists in Firestore for a user (encryption config, SSH keys, etc.)",
	Run:   runListFirestoreDataCommand,
}

func init() {
	// Add flags to commands
	addDeviceCmd.Flags().String("email", "", "User email (required)")
	addDeviceCmd.Flags().String("device-name", "", "Device name (required)")
	addDeviceCmd.Flags().String("public-key", "", "WireGuard public key (required)")
	addDeviceCmd.MarkFlagRequired("email")
	addDeviceCmd.MarkFlagRequired("device-name")
	addDeviceCmd.MarkFlagRequired("public-key")

	deleteDeviceCmd.Flags().String("email", "", "User email (required)")
	deleteDeviceCmd.Flags().String("device-name", "", "Device name to delete (required)")
	deleteDeviceCmd.MarkFlagRequired("email")
	deleteDeviceCmd.MarkFlagRequired("device-name")

	listDevicesCmd.Flags().String("email", "", "User email (required)")
	listDevicesCmd.MarkFlagRequired("email")

	approveDeviceCmd.Flags().String("challenge-id", "", "Challenge ID to approve (required)")
	approveDeviceCmd.Flags().String("email", "", "User email to associate with device (required)")
	approveDeviceCmd.MarkFlagRequired("challenge-id")
	approveDeviceCmd.MarkFlagRequired("email")

	validateKeyDecryptionCmd.Flags().String("email", "", "User email (required)")
	validateKeyDecryptionCmd.Flags().String("password", "", "User's encryption password (required)")
	validateKeyDecryptionCmd.MarkFlagRequired("email")
	validateKeyDecryptionCmd.MarkFlagRequired("password")

	listFirestoreDataCmd.Flags().String("email", "", "User email (required)")
	listFirestoreDataCmd.MarkFlagRequired("email")

	// Add subcommands to admin command
	adminCmd.AddCommand(
		addDeviceCmd,
		deleteDeviceCmd,
		listDevicesCmd,
		listPeersCmd,
		syncPeersCmd,
		listChallengesCmd,
		approveDeviceCmd,
		validateKeyDecryptionCmd,
		listFirestoreDataCmd,
	)
}

func runAddDeviceCommand(cmd *cobra.Command, args []string) {
	// Get flags
	email, _ := cmd.Flags().GetString("email")
	deviceName, _ := cmd.Flags().GetString("device-name")
	publicKey, _ := cmd.Flags().GetString("public-key")

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Connect to database
	log.Println("Connecting to database...")
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := storage.NewUserRepository(db)
	deviceRepo := storage.NewDeviceRepository(db)
	conflictRepo := storage.NewConflictRepository(db)
	deviceAuthRepo := storage.NewDeviceAuthRepository(db)

	// Initialize services
	subnetPool, err := services.NewSubnetPool(userRepo, conflictRepo)
	if err != nil {
		log.Fatalf("Failed to initialize subnet pool: %v", err)
	}

	deviceService := services.NewDeviceService(deviceRepo, userRepo, subnetPool, deviceAuthRepo)

	// Initialize WireGuard manager
	wgManager, err := wireguard.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize WireGuard manager: %v", err)
	}
	defer wgManager.Close()

	ctx := context.Background()

	// Find user by email
	fmt.Printf("Looking up user: %s\n", email)
	user, err := userRepo.GetByEmail(ctx, email)
	if err != nil {
		log.Fatalf("Failed to find user: %v", err)
	}
	if user == nil {
		log.Fatalf("User not found: %s", email)
	}

	fmt.Printf("Found user: %s (subnet: %s)\n", user.Email, user.Subnet)

	// Ensure firewall rules are configured for user's subnet
	fmt.Println("Configuring firewall rules...")
	outInterface := wireguard.GetDefaultOutInterface()
	if err := wireguard.EnsureMasqueradeRule(user.Subnet, outInterface); err != nil {
		log.Printf("Warning: Failed to configure NAT rules: %v", err)
	}
	if err := wireguard.EnsureForwardRule(user.Subnet); err != nil {
		log.Printf("Warning: Failed to configure FORWARD rules: %v", err)
	}

	// Register device
	fmt.Printf("Registering device '%s'...\n", deviceName)
	device, err := deviceService.RegisterDevice(ctx, user.ID, deviceName, publicKey, nil, nil, nil, nil, nil)
	if err != nil {
		log.Fatalf("Failed to register device: %v", err)
	}

	// Add device to WireGuard (handles replacement, no rollback for admin commands)
	fmt.Println("Adding peer to WireGuard...")
	if err := services.AddDeviceToWireGuard(ctx, wgManager, deviceRepo, device, false); err != nil {
		log.Printf("Warning: Failed to add peer to WireGuard: %v", err)
		log.Println("Device registered in database but not in WireGuard interface")
	} else {
		fmt.Println("✓ Peer added to WireGuard")
	}

	// Display success and config
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✓ Device registered successfully!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Device ID: %s\n", device.Device.ID)
	fmt.Printf("Device Name: %s\n", device.Device.DeviceName)
	fmt.Printf("VPN IP: %s\n", device.Device.VpnIP)
	fmt.Printf("User Subnet: %s\n", user.Subnet)
	fmt.Println()

	// Generate WireGuard config for the device
	serverEndpoint := os.Getenv("WG_SERVER_PUBLIC_ENDPOINT")
	if serverEndpoint == "" {
		serverEndpoint = "YOUR_SERVER_IP:51820"
	}

	fmt.Println("WireGuard configuration for device:")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf(`[Interface]
PrivateKey = <DEVICE_PRIVATE_KEY>
Address = %s/32
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, device.Device.VpnIP, wgManager.GetPublicKey(), serverEndpoint, user.Subnet)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("Copy this config to your device and replace <DEVICE_PRIVATE_KEY> with the device's private key.")
}

func runDeleteDeviceCommand(cmd *cobra.Command, args []string) {
	// Get flags
	email, _ := cmd.Flags().GetString("email")
	deviceName, _ := cmd.Flags().GetString("device-name")

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize database
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := storage.NewUserRepository(db)
	deviceRepo := storage.NewDeviceRepository(db)

	// Initialize WireGuard manager
	wgManager, err := wireguard.NewManager()
	if err != nil {
		log.Println("Warning: WireGuard manager initialization failed:", err)
		log.Println("Will delete from database only")
	}

	ctx := context.Background()

	// Get user by email
	user, err := userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		log.Fatalf("User not found: %s", email)
	}

	fmt.Printf("Found user: %s (%s)\n", user.Email, user.ID)

	// Get device by name
	device, err := deviceRepo.GetByUserAndName(ctx, user.ID, deviceName)
	if err != nil {
		log.Fatalf("Failed to find device: %v", err)
	}
	if device == nil {
		log.Fatalf("Device not found: %s", deviceName)
	}

	fmt.Printf("\nDevice found:\n")
	fmt.Printf("  ID: %s\n", device.ID)
	fmt.Printf("  Name: %s\n", device.DeviceName)
	fmt.Printf("  VPN IP: %s\n", device.VpnIP)
	fmt.Printf("  Public Key: %s\n", device.PublicKey)
	fmt.Printf("  Active: %v\n", device.Active)

	// Confirm deletion
	fmt.Print("\nAre you sure you want to delete this device? (yes/no): ")
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "yes" {
		fmt.Println("Deletion cancelled.")
		return
	}

	// Remove from WireGuard
	if wgManager != nil {
		fmt.Println("\nRemoving peer from WireGuard...")
		if err := wgManager.RemovePeer(device.PublicKey); err != nil {
			log.Printf("Warning: Failed to remove peer from WireGuard: %v", err)
		} else {
			fmt.Println("✓ Peer removed from WireGuard")
		}
	}

	// Delete from database
	fmt.Println("Deleting device from database...")
	if err := deviceRepo.Delete(ctx, device.ID); err != nil {
		log.Fatalf("Failed to delete device: %v", err)
	}

	fmt.Println("✓ Device deleted successfully!")
}

func runListDevicesCommand(cmd *cobra.Command, args []string) {
	// Get flag
	email, _ := cmd.Flags().GetString("email")

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize database
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := storage.NewUserRepository(db)
	deviceRepo := storage.NewDeviceRepository(db)

	ctx := context.Background()

	// Get user by email
	user, err := userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		log.Fatalf("User not found: %s", email)
	}

	fmt.Printf("User: %s (%s)\n", user.Email, user.ID)
	fmt.Printf("Subnet: %s\n", user.Subnet)
	fmt.Printf("Max Devices: %d\n\n", user.MaxDevices)

	// Get devices
	devices, err := deviceRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		log.Fatalf("Failed to get devices: %v", err)
	}

	if len(devices) == 0 {
		fmt.Println("No devices registered for this user.")
		return
	}

	fmt.Printf("Devices (%d):\n", len(devices))
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("%-36s %-20s %-15s %-8s\n", "ID", "Name", "VPN IP", "Active")
	fmt.Println(strings.Repeat("=", 80))

	for _, device := range devices {
		activeStatus := "Yes"
		if !device.Active {
			activeStatus = "No"
		}
		fmt.Printf("%-36s %-20s %-15s %-8s\n",
			device.ID,
			device.DeviceName,
			device.VpnIP,
			activeStatus,
		)
	}
	fmt.Println(strings.Repeat("=", 80))
}

func runListPeersCommand(cmd *cobra.Command, args []string) {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize WireGuard manager
	wgManager, err := wireguard.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize WireGuard manager: %v", err)
	}
	defer wgManager.Close()

	// Get interface info
	fmt.Printf("WireGuard Interface: %s\n", "wg0")
	fmt.Printf("Public Key: %s\n", wgManager.GetPublicKey())
	fmt.Printf("Endpoint: %s\n\n", wgManager.GetEndpoint())

	// List peers
	peers, err := wgManager.ListPeers()
	if err != nil {
		log.Fatalf("Failed to list peers: %v", err)
	}

	if len(peers) == 0 {
		fmt.Println("No peers configured.")
		return
	}

	fmt.Printf("Peers (%d):\n", len(peers))
	fmt.Println(strings.Repeat("=", 95))
	fmt.Printf("%-44s %-30s %-20s\n", "Public Key", "Allowed IPs", "Last Handshake")
	fmt.Println(strings.Repeat("=", 95))

	for _, peer := range peers {
		// Format allowed IPs
		allowedIPs := ""
		for i, ip := range peer.AllowedIPs {
			if i > 0 {
				allowedIPs += ", "
			}
			allowedIPs += ip.String()
		}

		// Format last handshake
		lastHandshake := "(never)"
		if !peer.LastHandshakeTime.IsZero() {
			duration := time.Since(peer.LastHandshakeTime)
			if duration < time.Minute {
				lastHandshake = fmt.Sprintf("%.0fs ago", duration.Seconds())
			} else if duration < time.Hour {
				lastHandshake = fmt.Sprintf("%.0fm ago", duration.Minutes())
			} else {
				lastHandshake = fmt.Sprintf("%.1fh ago", duration.Hours())
			}
		}

		fmt.Printf("%-44s %-30s %-20s\n",
			peer.PublicKey.String(),
			allowedIPs,
			lastHandshake,
		)
	}
	fmt.Println(strings.Repeat("=", 95))
}

func runSyncPeersCommand(cmd *cobra.Command, args []string) {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize database
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	deviceRepo := storage.NewDeviceRepository(db)

	// Initialize WireGuard manager
	wgManager, err := wireguard.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize WireGuard manager: %v", err)
	}
	defer wgManager.Close()

	ctx := context.Background()

	fmt.Println("Syncing WireGuard peers with database...")
	fmt.Println("")

	// Get all peers from WireGuard
	peers, err := wgManager.ListPeers()
	if err != nil {
		log.Fatalf("Failed to list WireGuard peers: %v", err)
	}

	fmt.Printf("Found %d peers in WireGuard\n", len(peers))

	// Check each peer against database
	removed := 0
	for _, peer := range peers {
		publicKey := peer.PublicKey.String()

		// Check if this public key exists in database
		device, err := deviceRepo.GetByPublicKey(ctx, publicKey)
		if err != nil {
			log.Printf("Warning: Failed to check device %s: %v", publicKey, err)
			continue
		}

		if device == nil {
			// Peer exists in WireGuard but not in database - remove it
			fmt.Printf("Removing orphaned peer: %s\n", publicKey)
			if err := wgManager.RemovePeer(publicKey); err != nil {
				log.Printf("Warning: Failed to remove peer: %v", err)
			} else {
				removed++
			}
		}
	}

	fmt.Println("")
	if removed > 0 {
		fmt.Printf("✓ Removed %d orphaned peer(s)\n", removed)
	} else {
		fmt.Println("✓ No orphaned peers found")
	}
}

func runListChallengesCommand(cmd *cobra.Command, args []string) {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize database
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	deviceAuthRepo := storage.NewDeviceAuthRepository(db)

	ctx := context.Background()

	// Get all pending challenges
	challenges, err := deviceAuthRepo.ListPendingChallenges(ctx)
	if err != nil {
		log.Fatalf("Failed to get challenges: %v", err)
	}

	if len(challenges) == 0 {
		fmt.Println("No pending device authorization challenges")
		return
	}

	fmt.Printf("Pending Device Authorization Challenges (%d):\n", len(challenges))
	fmt.Println(strings.Repeat("=", 110))
	fmt.Printf("%-36s %-20s %-30s %-20s\n", "Challenge ID", "Device ID", "Hostname", "Created At")
	fmt.Println(strings.Repeat("=", 110))

	for _, challenge := range challenges {
		fmt.Printf("%-36s %-20s %-30s %-20s\n",
			challenge.ID,
			challenge.DeviceID,
			challenge.Hostname,
			challenge.CreatedAt.Format("2006-01-02 15:04:05"),
		)
	}
	fmt.Println(strings.Repeat("=", 110))
}

func runApproveDeviceCommand(cmd *cobra.Command, args []string) {
	// Get flags
	challengeID, _ := cmd.Flags().GetString("challenge-id")
	email, _ := cmd.Flags().GetString("email")

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize database
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := storage.NewUserRepository(db)
	deviceRepo := storage.NewDeviceRepository(db)
	deviceAuthRepo := storage.NewDeviceAuthRepository(db)
	conflictRepo := storage.NewConflictRepository(db)

	// Initialize services
	subnetPool, err := services.NewSubnetPool(userRepo, conflictRepo)
	if err != nil {
		log.Fatalf("Failed to initialize subnet pool: %v", err)
	}

	deviceService := services.NewDeviceService(deviceRepo, userRepo, subnetPool, deviceAuthRepo)

	// Initialize WireGuard manager
	wgManager, err := wireguard.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize WireGuard manager: %v", err)
	}
	defer wgManager.Close()

	ctx := context.Background()

	// Get challenge
	fmt.Printf("Getting challenge: %s\n", challengeID)
	challengeUUID, err := uuid.Parse(challengeID)
	if err != nil {
		log.Fatalf("Invalid challenge ID: %v", err)
	}

	challenge, err := deviceAuthRepo.GetChallenge(ctx, challengeUUID)
	if err != nil {
		log.Fatalf("Failed to get challenge: %v", err)
	}
	if challenge == nil {
		log.Fatalf("Challenge not found: %s", challengeID)
	}

	fmt.Printf("Challenge found:\n")
	fmt.Printf("  Device ID: %s\n", challenge.DeviceID)
	fmt.Printf("  Hostname: %s\n", challenge.Hostname)
	if challenge.PublicKey != nil {
		fmt.Printf("  Public Key: %s\n", *challenge.PublicKey)
	}
	fmt.Println()

	// Get or create user
	fmt.Printf("Looking up user: %s\n", email)
	user, err := userRepo.GetByEmail(ctx, email)
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}

	if user == nil {
		// Create user
		fmt.Printf("User not found, creating new user...\n")
		subnet, err := subnetPool.AllocateSubnet(ctx)
		if err != nil {
			log.Fatalf("Failed to allocate subnet: %v", err)
		}

		user = &models.User{
			ID:         uuid.New(), // Generate ID before creating
			Email:      email,
			Subnet:     subnet,
			MaxDevices: 5,
			Active:     true, // IMPORTANT: set active to true
		}

		if err := userRepo.Create(ctx, user); err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		fmt.Printf("✓ User created: %s (subnet: %s, ID: %s)\n", user.Email, user.Subnet, user.ID)

		// Reload user from database to ensure it was saved correctly
		user, err = userRepo.GetByID(ctx, user.ID)
		if err != nil || user == nil {
			log.Fatalf("Failed to reload user after creation: %v", err)
		}
		fmt.Printf("✓ User reloaded from database\n")
	} else {
		fmt.Printf("✓ User found: %s (subnet: %s)\n", user.Email, user.Subnet)
	}

	// Configure firewall rules
	fmt.Println("Configuring firewall rules...")
	outInterface := wireguard.GetDefaultOutInterface()
	if err := wireguard.EnsureMasqueradeRule(user.Subnet, outInterface); err != nil {
		log.Printf("Warning: Failed to configure NAT rules: %v", err)
	}
	if err := wireguard.EnsureForwardRule(user.Subnet); err != nil {
		log.Printf("Warning: Failed to configure FORWARD rules: %v", err)
	}

	// Register device (use hostname as device name if no public key provided, otherwise use device ID)
	deviceName := challenge.Hostname
	if challenge.PublicKey == nil {
		log.Fatalf("Challenge does not have a public key for auto-registration")
	}

	fmt.Printf("Registering device '%s'...\n", deviceName)
	result, err := deviceService.RegisterDevice(ctx, user.ID, deviceName, *challenge.PublicKey, challenge.Username, nil, nil, nil, nil)
	if err != nil {
		log.Fatalf("Failed to register device: %v", err)
	}

	// Add device to WireGuard (handles replacement, no rollback for admin commands)
	fmt.Println("Adding peer to WireGuard...")
	if err := services.AddDeviceToWireGuard(ctx, wgManager, deviceRepo, result, false); err != nil {
		log.Printf("Warning: Failed to add peer to WireGuard: %v", err)
	} else {
		fmt.Println("✓ Peer added to WireGuard")
	}

	// Approve challenge
	fmt.Println("Approving challenge...")
	if err := deviceAuthRepo.UpdateChallengeStatus(ctx, challenge.ID, "approved", &user.ID); err != nil {
		log.Fatalf("Failed to approve challenge: %v", err)
	}

	if err := deviceAuthRepo.UpdateChallengeDeviceID(ctx, challenge.ID, &result.Device.ID); err != nil {
		log.Fatalf("Failed to update challenge device ID: %v", err)
	}

	fmt.Println("")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✓ Device approved successfully!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("User: %s\n", user.Email)
	fmt.Printf("Device ID: %s\n", result.Device.ID)
	fmt.Printf("Device Name: %s\n", result.Device.DeviceName)
	fmt.Printf("VPN IP: %s\n", result.Device.VpnIP)
	fmt.Printf("Subnet: %s\n", user.Subnet)
	fmt.Println("")
	fmt.Println("Client can now poll and receive JWT token")
}

func runValidateKeyDecryptionCommand(cmd *cobra.Command, args []string) {
	// Get flags
	email, _ := cmd.Flags().GetString("email")
	password, _ := cmd.Flags().GetString("password")

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	ctx := context.Background()

	// Initialize SSH service (connects to Firestore)
	fmt.Println("Connecting to Firestore...")
	sshService, err := services.NewSSHService(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize SSH service: %v", err)
	}
	defer sshService.Close()

	fmt.Printf("Validating encrypted SSH keys for: %s\n", email)
	fmt.Println(strings.Repeat("=", 60))

	// Validate key decryption
	summary, err := sshService.ValidateKeyDecryption(ctx, email, password)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	// Display results
	if !summary.EncryptionOK {
		fmt.Println("✗ Encryption configuration not found")
		fmt.Println("  User has not set up encryption yet")
		return
	}

	fmt.Printf("Encryption Config: ✓ Found (salt: %s...)\n", summary.Salt[:min(16, len(summary.Salt))])
	fmt.Printf("Total SSH Keys: %d\n", summary.TotalKeys)
	fmt.Println("")

	if summary.TotalKeys == 0 {
		fmt.Println("No SSH keys found for this user")
		return
	}

	// Display per-key results
	fmt.Println("Key Validation Results:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("%-30s %-15s %-10s %-25s\n", "Name", "Type", "Status", "Error")
	fmt.Println(strings.Repeat("=", 80))

	for _, result := range summary.Results {
		status := "✓ Valid"
		errorMsg := "-"
		if !result.Valid {
			status = "✗ Invalid"
			errorMsg = truncateString(result.Error, 25)
		}

		fmt.Printf("%-30s %-15s %-10s %-25s\n",
			truncateString(result.Name, 30),
			result.Type,
			status,
			errorMsg,
		)
	}
	fmt.Println(strings.Repeat("=", 80))

	// Summary
	fmt.Println("")
	if summary.ValidKeys == summary.TotalKeys {
		fmt.Printf("✓ All %d key(s) validated successfully!\n", summary.ValidKeys)
		fmt.Println("  Password is correct and all keys can be decrypted")
	} else if summary.ValidKeys > 0 {
		fmt.Printf("⚠ Partial success: %d/%d key(s) validated\n", summary.ValidKeys, summary.TotalKeys)
		fmt.Printf("  %d key(s) failed validation\n", summary.InvalidKeys)
	} else {
		fmt.Printf("✗ Validation failed: 0/%d key(s) validated\n", summary.TotalKeys)
		fmt.Println("  Either password is incorrect or keys are corrupted")
	}
}

// Helper function to truncate strings for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Helper function for min (Go 1.21+)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runListFirestoreDataCommand(cmd *cobra.Command, args []string) {
	// Get flag
	email, _ := cmd.Flags().GetString("email")

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	ctx := context.Background()

	// Initialize SSH service (connects to Firestore)
	fmt.Println("Connecting to Firestore...")
	sshService, err := services.NewSSHService(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize SSH service: %v", err)
	}
	defer sshService.Close()

	fmt.Printf("\nFirestore data for: %s\n", email)
	fmt.Println(strings.Repeat("=", 60))

	// Check encryption config
	fmt.Println("\n1. Encryption Configuration:")
	config, err := sshService.GetEncryptionConfig(ctx, email, "")
	if err != nil {
		fmt.Printf("   ✗ Not found: %v\n", err)
	} else {
		fmt.Println("   ✓ Found")
		fmt.Printf("   Algorithm: %s\n", config.Algorithm)
		fmt.Printf("   Iterations: %d\n", config.Iterations)
		if len(config.Salt) > 20 {
			fmt.Printf("   Salt: %s...\n", config.Salt[:20])
		} else {
			fmt.Printf("   Salt: %s\n", config.Salt)
		}
	}

	// Check SSH keys (without encrypted data)
	fmt.Println("\n2. SSH Keys:")
	keys, err := sshService.GetUserSSHKeys(ctx, email)
	if err != nil {
		fmt.Printf("   ✗ Error: %v\n", err)
	} else if len(keys) == 0 {
		fmt.Println("   ✗ No SSH keys found")
	} else {
		fmt.Printf("   ✓ Found %d key(s)\n\n", len(keys))
		for i, key := range keys {
			fmt.Printf("   Key %d:\n", i+1)
			fmt.Printf("     Name: %s\n", key.Name)
			fmt.Printf("     Type: %s\n", key.Type)
			fmt.Printf("     Fingerprint: %s\n", key.Fingerprint)
			if len(key.PublicKey) > 50 {
				fmt.Printf("     Public Key: %s...\n", key.PublicKey[:50])
			} else {
				fmt.Printf("     Public Key: %s\n", key.PublicKey)
			}
		}
	}

	// Check encrypted keys
	fmt.Println("\n3. Encrypted SSH Keys:")
	encryptedKeys, err := sshService.GetEncryptedSSHKeys(ctx, email)
	if err != nil {
		fmt.Printf("   ✗ Error: %v\n", err)
	} else if len(encryptedKeys) == 0 {
		fmt.Println("   ✗ No encrypted keys found")
	} else {
		fmt.Printf("   ✓ Found %d encrypted key(s)\n\n", len(encryptedKeys))
		for i, key := range encryptedKeys {
			fmt.Printf("   Key %d:\n", i+1)
			fmt.Printf("     Name: %s\n", key.Name)
			fmt.Printf("     Type: %s\n", key.Type)
			fmt.Printf("     Fingerprint: %s\n", key.Fingerprint)
			hasEncrypted := "No"
			if key.EncryptedPrivateKey != "" {
				hasEncrypted = "Yes"
			}
			fmt.Printf("     Has Encrypted Private Key: %s\n", hasEncrypted)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}
