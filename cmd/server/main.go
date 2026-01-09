package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/api"
	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/setup"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/internal/server/tunnel"
	"github.com/kamikazebr/roamie-desktop/internal/server/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/version"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var rootCmd = &cobra.Command{
	Use:   "roamie-server",
	Short: "Roamie VPN Server - WireGuard VPN with email authentication",
	Long:  "Server component for Roamie VPN providing HTTP API and WireGuard management",
	// Default to serve command if no subcommand provided
	Run: func(cmd *cobra.Command, args []string) {
		runServe(cmd, args)
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the VPN server",
	Long:  "Start the Roamie VPN server with HTTP API and WireGuard management",
	Run:   runServe,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.GetVersion("roamie-server"))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd, adminCmd, versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) {

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Log version information
	log.Printf("=== Roamie VPN Server ===")
	log.Printf("%s", version.GetVersion("roamie-server"))
	log.Println()

	// Step 1: Setup database (auto-install Docker + PostgreSQL if needed)
	log.Println("=== Database Setup ===")
	if err := setup.CheckAndSetupDatabase(); err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}

	// Step 2: Connect to database
	log.Println("Connecting to database...")
	db, err := storage.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected")

	// Step 3: Run embedded migrations
	log.Println("Running database migrations...")
	if err := runEmbeddedMigrations(db.DB.DB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations complete")

	// Initialize repositories
	userRepo := storage.NewUserRepository(db)
	authRepo := storage.NewAuthRepository(db)
	deviceRepo := storage.NewDeviceRepository(db)
	conflictRepo := storage.NewConflictRepository(db)
	biometricAuthRepo := storage.NewBiometricAuthRepository(db)
	deviceAuthRepo := storage.NewDeviceAuthRepository(db)

	// Step 4: Setup WireGuard (auto-install + configure)
	log.Println("=== WireGuard Setup ===")
	if err := setup.CheckAndSetup(); err != nil {
		log.Fatalf("Failed to setup WireGuard: %v", err)
	}

	// Step 5: Initialize WireGuard manager
	log.Println("Initializing WireGuard manager...")
	wgManager, err := wireguard.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize WireGuard manager: %v", err)
	}
	defer wgManager.Close()

	if err := wgManager.InitializeInterface(); err != nil {
		log.Printf("Warning: WireGuard interface initialization failed: %v", err)
		log.Println("Please ensure WireGuard interface is created manually")
	}
	log.Printf("WireGuard manager initialized (public key: %s)", wgManager.GetPublicKey())

	// Initialize services
	emailService, err := services.NewEmailService()
	if err != nil {
		log.Fatalf("Failed to initialize email service: %v", err)
	}

	subnetPool, err := services.NewSubnetPool(userRepo, conflictRepo)
	if err != nil {
		log.Fatalf("Failed to initialize subnet pool: %v", err)
	}

	networkScanner := services.NewNetworkScanner(conflictRepo)
	authService := services.NewAuthService(authRepo, userRepo, emailService, subnetPool)
	deviceService := services.NewDeviceService(deviceRepo, userRepo, subnetPool, deviceAuthRepo)
	biometricAuthService := services.NewBiometricAuthService(biometricAuthRepo, userRepo, deviceRepo)
	deviceAuthService := services.NewDeviceAuthService(deviceAuthRepo, userRepo)

	// Link device service to device auth service (for auto-registration)
	deviceAuthService.SetDeviceService(deviceService)

	// Initialize Firebase service (optional - only if configured)
	var firebaseService *services.FirebaseService
	ctx := context.Background()
	if fbService, err := services.NewFirebaseService(ctx); err != nil {
		log.Printf("Warning: Firebase not configured: %v", err)
		log.Println("Firebase authentication will not be available")
	} else {
		firebaseService = fbService
		log.Println("Firebase authentication initialized")
	}

	// Initialize SSH service (optional - only if Firebase configured)
	var sshService *services.SSHService
	if firebaseService != nil {
		if sshSvc, err := services.NewSSHService(ctx); err != nil {
			log.Printf("Warning: SSH service initialization failed: %v", err)
			log.Println("SSH key sync will not be available")
		} else {
			sshService = sshSvc
			log.Println("SSH service initialized")
		}
	}

	// Scan networks on startup
	log.Println("Scanning for network conflicts...")
	conflicts, _ := networkScanner.ScanNetworks(ctx)
	log.Printf("Found %d network conflicts", len(conflicts))

	// Initialize device cache for heartbeat tracking
	deviceCache := services.NewDeviceCache()
	log.Println("Device cache initialized")

	// Initialize tunnel port pool
	tunnelPortPool, err := services.NewTunnelPortPool(deviceRepo)
	if err != nil {
		log.Fatalf("Failed to initialize tunnel port pool: %v", err)
	}
	log.Println("Tunnel port pool initialized")

	// Initialize DiagnosticsService (optional - only if Firebase is configured)
	var diagnosticsService *services.DiagnosticsService
	if os.Getenv("FIREBASE_CREDENTIALS_PATH") != "" {
		diagnosticsService, err = services.NewDiagnosticsService(ctx)
		if err != nil {
			log.Printf("Warning: Failed to initialize DiagnosticsService: %v", err)
			log.Println("Remote diagnostics will not be available")
		} else {
			log.Println("DiagnosticsService initialized successfully")
		}
	} else {
		log.Println("Firebase not configured - remote diagnostics will not be available")
	}

	// Initialize handlers
	authHandler := api.NewAuthHandler(authService)
	deviceHandler := api.NewDeviceHandler(deviceService, userRepo, deviceRepo, wgManager, deviceCache, diagnosticsService)
	adminHandler := api.NewAdminHandler(networkScanner)
	biometricAuthHandler := api.NewBiometricAuthHandler(biometricAuthService)
	deviceAuthHandler := api.NewDeviceAuthHandler(deviceAuthService, authService, firebaseService, deviceService, wgManager, userRepo, deviceRepo)
	tunnelService := services.NewTunnelService(deviceRepo)
	tunnelHandler := api.NewTunnelHandler(deviceRepo, deviceService, tunnelPortPool, tunnelService)

	// SSH handler (only if SSH service initialized)
	var sshHandler *api.SSHHandler
	if sshService != nil {
		sshHandler = api.NewSSHHandler(sshService, userRepo)
	}

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(api.CORSMiddleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"roamie-desktop"}`))
	})

	// Public routes
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/request-code", authHandler.RequestCode)
		r.Post("/verify-code", authHandler.VerifyCode)

		// Device authorization (public endpoints)
		r.Post("/device-request", deviceAuthHandler.CreateDeviceRequest)
		r.Get("/device-poll/{challenge_id}", deviceAuthHandler.PollChallenge)
		r.Post("/refresh", deviceAuthHandler.RefreshJWT)
		r.Post("/login", deviceAuthHandler.Login)
	})

	// Protected routes
	r.Route("/api", func(r chi.Router) {
		r.Use(api.AuthMiddleware)

		// Device management
		r.Route("/devices", func(r chi.Router) {
			r.Get("/", deviceHandler.ListDevices)
			r.Post("/", deviceHandler.RegisterDevice)
			r.Get("/validate", deviceHandler.ValidateDevice)
			r.Delete("/{device_id}", deviceHandler.DeleteDevice)
			r.Get("/{device_id}/config", deviceHandler.GetDeviceConfig)
			r.Post("/heartbeat", deviceHandler.Heartbeat)

			// Diagnostics endpoints
			r.Post("/{device_id}/trigger-doctor", deviceHandler.TriggerDoctor)
			r.Get("/{device_id}/diagnostics/{request_id}", deviceHandler.GetDiagnosticsReport)
			r.Get("/{device_id}/diagnostics", deviceHandler.GetAllDiagnosticsReports)

			// Daemon diagnostics endpoints (server-as-proxy)
			r.Get("/diagnostics/pending", deviceHandler.GetPendingDiagnostics)
			r.Post("/diagnostics/report", deviceHandler.UploadDiagnosticsReport)
		})

		// SSH Tunnel management
		r.Route("/tunnel", func(r chi.Router) {
			r.Post("/register", tunnelHandler.Register)
			r.Post("/register-key", tunnelHandler.RegisterKey)
			r.Get("/status", tunnelHandler.GetStatus)
			r.Get("/authorized-keys", tunnelHandler.GetAuthorizedKeys)
		})

		// Device-specific tunnel control
		r.Route("/devices/{device_id}/tunnel", func(r chi.Router) {
			r.Patch("/enable", tunnelHandler.EnableTunnel)
			r.Patch("/disable", tunnelHandler.DisableTunnel)
		})

		// Biometric authentication
		r.Route("/biometric", func(r chi.Router) {
			r.Post("/request", biometricAuthHandler.CreateRequest)
			r.Get("/pending", biometricAuthHandler.ListPending)
			r.Post("/respond", biometricAuthHandler.RespondToRequest)
			r.Get("/poll/{request_id}", biometricAuthHandler.PollStatus)
			r.Get("/stats", biometricAuthHandler.GetStats)
		})

		// Device authorization (protected endpoints)
		// Note: Mobile app gets challenge_id from QR code scan, not from listing endpoint
		r.Post("/device-auth/approve", deviceAuthHandler.ApproveDevice)

		// SSH key management
		if sshHandler != nil {
			r.Get("/ssh/keys", sshHandler.GetSSHKeys)
		}
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(api.AuthMiddleware)
		r.Use(api.AdminMiddleware)
		r.Route("/network", func(r chi.Router) {
			r.Get("/scan", adminHandler.ScanNetworks)
			r.Get("/conflicts", adminHandler.ListConflicts)
			r.Post("/conflicts", adminHandler.AddConflict)
		})
	})

	// Get server config
	host := os.Getenv("API_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	// Find available port
	port = findAvailableAPIPort(port)
	addr := fmt.Sprintf("%s:%s", host, port)

	// Create server
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start background cleanup jobs
	go cleanupExpiredCodes(authService)
	go cleanupExpiredBiometricRequests(biometricAuthService)
	go cleanupExpiredDeviceChallenges(deviceAuthService)

	// Initialize and start SSH tunnel server (unless disabled for testing)
	var tunnelServer *tunnel.Server
	if os.Getenv("DISABLE_TUNNEL_SERVER") != "true" {
		log.Println("=== SSH Tunnel Server Setup ===")
		var err error
		tunnelServer, err = tunnel.NewServer(deviceRepo)
		if err != nil {
			log.Fatalf("Failed to initialize SSH tunnel server: %v", err)
		}

		// Check firewall and open port if needed
		if isFirewallActive() {
			log.Println("Firewall detected as active, ensuring port 2222 is open...")
			if err := openFirewallPort(2222); err != nil {
				log.Printf("Warning: failed to open firewall port 2222: %v", err)
			} else {
				log.Println("✓ Firewall configured for SSH tunnel (port 2222)")
			}
		} else {
			log.Println("Firewall not active, skipping firewall configuration")
		}

		// Start tunnel server
		if err := tunnelServer.Start(); err != nil {
			log.Printf("Warning: SSH tunnel server failed to start: %v", err)
		} else {
			log.Println("✓ SSH tunnel server started successfully")
		}
		defer tunnelServer.Stop()
	} else {
		log.Println("=== SSH Tunnel Server ===")
		log.Println("Tunnel server disabled (DISABLE_TUNNEL_SERVER=true)")
		log.Println("Using external SSH server on port 2222")
	}

	// Start server
	go func() {
		log.Printf("Server starting on %s", addr)
		log.Printf("WireGuard endpoint: %s", wgManager.GetEndpoint())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func cleanupExpiredCodes(authService *services.AuthService) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		if err := authService.CleanupExpiredCodes(ctx); err != nil {
			log.Printf("Failed to cleanup expired codes: %v", err)
		}
	}
}

func cleanupExpiredBiometricRequests(biometricAuthService *services.BiometricAuthService) {
	// More frequent cleanup for biometric requests (every 1 minute)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		// Delete requests older than 7 days
		if count, err := biometricAuthService.CleanupOldRequests(ctx, 7); err != nil {
			log.Printf("Failed to cleanup old biometric requests: %v", err)
		} else if count > 0 {
			log.Printf("Cleaned up %d old biometric requests", count)
		}
	}
}

func cleanupExpiredDeviceChallenges(deviceAuthService *services.DeviceAuthService) {
	// Cleanup device auth challenges every 2 minutes
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		if _, err := deviceAuthService.ListPendingChallenges(ctx); err != nil {
			log.Printf("Failed to cleanup expired device challenges: %v", err)
		}
		// ListPendingChallenges already calls ExpireOldChallenges internally
	}
}

func runEmbeddedMigrations(db *sql.DB) error {
	// Read all migration files from embedded FS
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migrations by filename to ensure correct order
	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			migrations = append(migrations, entry.Name())
		}
	}
	sort.Strings(migrations)

	// Execute each migration
	for _, migration := range migrations {
		log.Printf("Applying migration: %s", migration)

		// Read migration file
		content, err := migrationsFS.ReadFile(filepath.Join("migrations", migration))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migration, err)
		}

		// Execute migration (ignore errors if table already exists)
		if _, err := db.Exec(string(content)); err != nil {
			// Log warning but continue if table already exists
			if filepath.Base(migration)[:3] != "000" { // Skip version check migrations
				log.Printf("Warning: Migration %s: %v (may already exist)", migration, err)
			}
		}
	}

	return nil
}

// isPortAvailable checks if a port is available for binding
func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false // Port in use
	}
	ln.Close()
	return true // Port available
}

// findAvailableAPIPort finds an available port for the API server
func findAvailableAPIPort(preferredPort string) string {
	// Try preferred port first
	if isPortAvailable(preferredPort) {
		log.Printf("✓ Port %s available", preferredPort)
		return preferredPort
	}

	log.Printf("Port %s in use, trying alternatives...", preferredPort)

	// Convert preferred port to int
	startPort := 8080
	if p, err := strconv.Atoi(preferredPort); err == nil {
		startPort = p
	}

	// Try next 20 ports
	for i := 1; i <= 20; i++ {
		port := startPort + i
		portStr := strconv.Itoa(port)
		if isPortAvailable(portStr) {
			log.Printf("✓ Found available port: %s", portStr)
			return portStr
		}
	}

	// No ports available, return preferred (will fail with clear error)
	log.Printf("⚠️  No available ports found, will attempt %s", preferredPort)
	return preferredPort
}

// isFirewallActive checks if ufw firewall is active
// Returns true if firewall is active, false otherwise
func isFirewallActive() bool {
	// Check if ufw command exists
	if _, err := exec.LookPath("ufw"); err != nil {
		log.Println("ufw command not found, assuming no firewall")
		return false
	}

	// Run ufw status
	cmd := exec.Command("ufw", "status")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to check firewall status: %v", err)
		return false
	}

	// Check if output contains "Status: active"
	return strings.Contains(string(output), "Status: active")
}

// openFirewallPort opens a port in the ufw firewall
func openFirewallPort(port int) error {
	// Open the port
	cmd := exec.Command("ufw", "allow", fmt.Sprintf("%d/tcp", port))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to open port %d: %w (output: %s)", port, err, string(output))
	}

	log.Printf("Opened firewall port %d: %s", port, strings.TrimSpace(string(output)))
	return nil
}
