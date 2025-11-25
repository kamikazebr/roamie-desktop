package setup

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// CheckAndSetup checks if WireGuard is properly configured and sets it up if needed
// This function is idempotent and can be called multiple times safely
func CheckAndSetup() error {
	log.Println("Checking WireGuard setup...")

	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("server must run as root for WireGuard management. Please run with: sudo ./roamie-server")
	}

	// Check if WireGuard is installed
	if !isWireGuardInstalled() {
		log.Println("WireGuard not found. Installing...")
		if err := installWireGuard(); err != nil {
			return fmt.Errorf("failed to install WireGuard: %w", err)
		}
		log.Println("✓ WireGuard installed")
	} else {
		log.Println("✓ WireGuard already installed")
	}

	// Enable IP forwarding
	if err := enableIPForwarding(); err != nil {
		log.Printf("Warning: Failed to enable IP forwarding: %v", err)
	} else {
		log.Println("✓ IP forwarding enabled")
	}

	// Create WireGuard directory
	if err := setupWireGuardDirectory(); err != nil {
		return fmt.Errorf("failed to setup WireGuard directory: %w", err)
	}
	log.Println("✓ WireGuard directory ready")

	// Generate server keys if needed
	if err := generateServerKeys(); err != nil {
		return fmt.Errorf("failed to generate server keys: %w", err)
	}
	log.Println("✓ Server keys ready")

	// Get WireGuard configuration from environment
	wgInterface := os.Getenv("WG_INTERFACE")
	if wgInterface == "" {
		wgInterface = "wg0"
	}

	wgPort := os.Getenv("WG_PORT")
	if wgPort == "" {
		wgPort = "51820"
	}

	endpoint := os.Getenv("WG_SERVER_PUBLIC_ENDPOINT")
	if endpoint == "" {
		return fmt.Errorf("WG_SERVER_PUBLIC_ENDPOINT not set in .env file")
	}

	// Create or update WireGuard interface config
	if err := createWireGuardConfig(wgInterface, wgPort); err != nil {
		return fmt.Errorf("failed to create WireGuard config: %w", err)
	}
	log.Printf("✓ WireGuard config created/updated (%s)", wgInterface)

	// Start WireGuard interface if not already running
	if err := startWireGuardInterface(wgInterface); err != nil {
		log.Printf("Note: %v", err)
	} else {
		log.Printf("✓ WireGuard interface %s started", wgInterface)
	}

	// Configure firewall (best effort)
	if err := configureFirewall(wgPort); err != nil {
		log.Printf("Warning: Failed to configure firewall: %v", err)
		log.Println("Please manually configure firewall:")
		log.Printf("  sudo ufw allow %s/udp  # WireGuard", wgPort)
		log.Println("  sudo ufw allow 8080/tcp  # API")
	} else {
		log.Println("✓ Firewall configured")
	}

	log.Println("✓ WireGuard setup complete!")
	return nil
}

func isWireGuardInstalled() bool {
	cmd := exec.Command("which", "wg")
	return cmd.Run() == nil
}

func installWireGuard() error {
	log.Println("Updating package lists...")
	if err := runCommand("apt-get", "update", "-qq"); err != nil {
		return err
	}

	log.Println("Installing WireGuard (this may take a minute)...")
	if err := runCommand("apt-get", "install", "-y", "wireguard", "wireguard-tools"); err != nil {
		return err
	}

	return nil
}

func enableIPForwarding() error {
	// Check if already enabled
	if isIPForwardingEnabled() {
		return nil
	}

	// Enable for current session
	if err := runCommand("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return err
	}
	if err := runCommand("sysctl", "-w", "net.ipv6.conf.all.forwarding=1"); err != nil {
		return err
	}

	// Make persistent across reboots
	sysctlConf := "/etc/sysctl.conf"
	content, err := os.ReadFile(sysctlConf)
	if err != nil {
		return err
	}

	lines := string(content)
	if !strings.Contains(lines, "net.ipv4.ip_forward=1") {
		f, err := os.OpenFile(sysctlConf, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := f.WriteString("\n# Added by Roamie VPN\nnet.ipv4.ip_forward=1\n"); err != nil {
			return err
		}
	}

	if !strings.Contains(lines, "net.ipv6.conf.all.forwarding=1") {
		f, err := os.OpenFile(sysctlConf, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := f.WriteString("net.ipv6.conf.all.forwarding=1\n"); err != nil {
			return err
		}
	}

	return runCommand("sysctl", "-p")
}

func isIPForwardingEnabled() bool {
	output, err := exec.Command("sysctl", "net.ipv4.ip_forward").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "net.ipv4.ip_forward = 1")
}

func setupWireGuardDirectory() error {
	dir := "/etc/wireguard"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.Chmod(dir, 0700)
}

func generateServerKeys() error {
	privateKeyPath := "/etc/wireguard/server_private.key"
	publicKeyPath := "/etc/wireguard/server_public.key"

	// Check if keys already exist
	if fileExists(privateKeyPath) && fileExists(publicKeyPath) {
		return nil
	}

	// Generate private key
	cmd := exec.Command("wg", "genkey")
	privateKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Save private key
	if err := os.WriteFile(privateKeyPath, privateKey, 0600); err != nil {
		return err
	}

	// Generate public key from private key
	cmd = exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(string(privateKey))
	publicKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate public key: %w", err)
	}

	// Save public key
	if err := os.WriteFile(publicKeyPath, publicKey, 0644); err != nil {
		return err
	}

	log.Printf("Server public key: %s", strings.TrimSpace(string(publicKey)))
	return nil
}

func createWireGuardConfig(interfaceName, port string) error {
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", interfaceName)

	// Read private key
	privateKeyBytes, err := os.ReadFile("/etc/wireguard/server_private.key")
	if err != nil {
		return err
	}
	privateKey := strings.TrimSpace(string(privateKeyBytes))

	// Get base network from env
	baseNetwork := os.Getenv("WG_BASE_NETWORK")
	if baseNetwork == "" {
		baseNetwork = "10.100.0.0/16"
	}

	// Extract first IP for server (e.g., 10.100.0.1)
	serverIP := getServerIPFromNetwork(baseNetwork)

	// Detect primary network interface for PostUp/PostDown
	networkInterface := detectPrimaryInterface()
	if networkInterface == "" {
		networkInterface = "eth0" // fallback
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/%s
ListenPort = %s
PostUp = iptables -A FORWARD -i %s -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE
PostDown = iptables -D FORWARD -i %s -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE

# Peers will be added dynamically by Roamie VPN server
`, privateKey, serverIP, "16", port, interfaceName, networkInterface, interfaceName, networkInterface)

	return os.WriteFile(configPath, []byte(config), 0600)
}

func getServerIPFromNetwork(cidr string) string {
	// Extract IP from CIDR (e.g., "10.100.0.0/16" -> "10.100.0.1")
	parts := strings.Split(cidr, "/")
	if len(parts) == 0 {
		return "10.100.0.1" // fallback
	}

	ipParts := strings.Split(parts[0], ".")
	if len(ipParts) != 4 {
		return "10.100.0.1" // fallback
	}

	// Change last octet to 1
	ipParts[3] = "1"
	return strings.Join(ipParts, ".")
}

func detectPrimaryInterface() string {
	// Try to find the default route interface
	output, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return ""
	}

	// Parse output like: "default via 192.168.1.1 dev eth0 ..."
	fields := strings.Fields(string(output))
	for i, field := range fields {
		if field == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}

	return ""
}

func startWireGuardInterface(interfaceName string) error {
	// Check if interface already exists and is running
	output, err := exec.Command("ip", "link", "show", interfaceName).Output()
	if err == nil && strings.Contains(string(output), "state UP") {
		return fmt.Errorf("interface %s already running", interfaceName)
	}

	// Start interface
	cmd := exec.Command("wg-quick", "up", interfaceName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		// If it's already up, that's okay
		if strings.Contains(string(output), "already exists") {
			return fmt.Errorf("interface %s already exists", interfaceName)
		}
		return fmt.Errorf("failed to start interface: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func configureFirewall(wgPort string) error {
	// Check if ufw is installed
	if !commandExists("ufw") {
		return fmt.Errorf("ufw not installed")
	}

	// Allow WireGuard port
	if err := runCommand("ufw", "allow", wgPort+"/udp"); err != nil {
		return err
	}

	// Allow API port
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "8080"
	}
	if err := runCommand("ufw", "allow", apiPort+"/tcp"); err != nil {
		return err
	}

	// Enable firewall (if not already enabled)
	cmd := exec.Command("ufw", "--force", "enable")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Don't fail if firewall is already enabled
		if !strings.Contains(string(output), "active") {
			return fmt.Errorf("failed to enable firewall: %w\nOutput: %s", err, string(output))
		}
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command '%s %s' failed: %w\nOutput: %s", name, strings.Join(args, " "), err, string(output))
	}
	return nil
}

func commandExists(name string) bool {
	cmd := exec.Command("which", name)
	return cmd.Run() == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
