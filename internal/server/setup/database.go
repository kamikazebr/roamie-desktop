package setup

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	DefaultPostgresUser   = "roamie"
	DefaultPostgresDB     = "roamie_vpn"
	PostgresContainerName = "roamie-postgres"
)

var selectedPostgresPort = "5432" // Will be set by findAvailablePostgresPort()

// getPostgresPassword returns the PostgreSQL password from environment or generates a random one for development
func getPostgresPassword() string {
	if pw := os.Getenv("POSTGRES_PASSWORD"); pw != "" {
		return pw
	}
	// Fallback for local development auto-setup only
	// In production, POSTGRES_PASSWORD or DATABASE_URL should always be set
	return "roamie_local_dev_" + fmt.Sprintf("%d", time.Now().Unix()%10000)
}

// CheckAndSetupDatabase ensures a PostgreSQL database is available
// It tries in order: existing DATABASE_URL -> Docker -> error
func CheckAndSetupDatabase() error {
	log.Println("Checking database configuration...")

	// Check if DATABASE_URL is set and working
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		if isDatabaseAccessible(databaseURL) {
			log.Println("✓ Database already configured and accessible")
			return nil
		}
		log.Println("DATABASE_URL set but database not accessible, will try to setup...")
	}

	// Check if Docker is installed
	if !isDockerInstalled() {
		return fmt.Errorf(`Docker is required for automatic database setup.

Please install Docker:
  curl -fsSL https://get.docker.com | sh

Or manually configure DATABASE_URL in .env file.`)
	}

	log.Println("✓ Docker is installed")

	// Check if PostgreSQL container is already running
	if isPostgresContainerRunning() {
		// Get the port of the running container
		cmd := exec.Command("docker", "port", PostgresContainerName, "5432")
		if portOutput, err := cmd.Output(); err == nil {
			// Extract port from output like "0.0.0.0:5432"
			parts := strings.Split(strings.TrimSpace(string(portOutput)), ":")
			if len(parts) == 2 {
				selectedPostgresPort = parts[1]
			}
		}
		log.Printf("✓ PostgreSQL container already running on port %s", selectedPostgresPort)
		return ensureDatabaseURLInEnv()
	}

	// Start PostgreSQL with Docker
	log.Println("Starting PostgreSQL container...")
	if err := startPostgresContainer(); err != nil {
		return fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	log.Println("✓ PostgreSQL container started")

	// Wait for PostgreSQL to be ready
	log.Println("Waiting for PostgreSQL to be ready...")
	if err := waitForPostgres(); err != nil {
		return fmt.Errorf("PostgreSQL failed to start: %w", err)
	}

	log.Println("✓ PostgreSQL is ready")

	// Update .env file with DATABASE_URL
	if err := ensureDatabaseURLInEnv(); err != nil {
		return fmt.Errorf("failed to update .env file: %w", err)
	}

	log.Println("✓ Database setup complete")
	return nil
}

func isDockerInstalled() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}

func isPostgresContainerRunning() bool {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", PostgresContainerName), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == PostgresContainerName
}

func startPostgresContainer() error {
	// Check if container exists but is stopped
	cmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s", PostgresContainerName), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == PostgresContainerName {
		// Container exists, get its port mapping to reuse
		cmd = exec.Command("docker", "port", PostgresContainerName, "5432")
		if portOutput, err := cmd.Output(); err == nil {
			// Extract port from output like "0.0.0.0:5432"
			parts := strings.Split(strings.TrimSpace(string(portOutput)), ":")
			if len(parts) == 2 {
				selectedPostgresPort = parts[1]
				log.Printf("Using existing container port: %s", selectedPostgresPort)
			}
		}

		// Container exists, just start it
		log.Println("Starting existing PostgreSQL container...")
		cmd = exec.Command("docker", "start", PostgresContainerName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start existing container: %w", err)
		}
		return nil
	}

	// Find an available port
	selectedPostgresPort = findAvailablePostgresPort()

	// Create and start new container
	args := []string{
		"run",
		"-d",
		"--name", PostgresContainerName,
		"-e", fmt.Sprintf("POSTGRES_USER=%s", DefaultPostgresUser),
		"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", getPostgresPassword()),
		"-e", fmt.Sprintf("POSTGRES_DB=%s", DefaultPostgresDB),
		"-p", fmt.Sprintf("%s:5432", selectedPostgresPort),
		"--restart", "unless-stopped",
		"postgres:15-alpine",
	}

	log.Printf("Starting PostgreSQL container on port %s...", selectedPostgresPort)
	cmd = exec.Command("docker", args...)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func waitForPostgres() error {
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		cmd := exec.Command("docker", "exec", PostgresContainerName, "pg_isready", "-U", DefaultPostgresUser)
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("PostgreSQL did not become ready after %d seconds", maxAttempts)
}

func isDatabaseAccessible(databaseURL string) bool {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return false
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.PingContext(ctx) == nil
}

func ensureDatabaseURLInEnv() error {
	// Build DATABASE_URL using the selected port
	databaseURL := fmt.Sprintf(
		"postgresql://%s:%s@localhost:%s/%s?sslmode=disable",
		DefaultPostgresUser,
		getPostgresPassword(),
		selectedPostgresPort,
		DefaultPostgresDB,
	)

	// Set in current environment
	os.Setenv("DATABASE_URL", databaseURL)

	// Check if .env file exists
	envPath := ".env"
	content := ""
	if data, err := os.ReadFile(envPath); err == nil {
		content = string(data)
	}

	// Check if DATABASE_URL is already in .env
	if strings.Contains(content, "DATABASE_URL=") {
		// Already exists, don't modify
		return nil
	}

	// Append DATABASE_URL to .env
	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		f.WriteString("\n")
	}

	f.WriteString("# Auto-generated by Roamie VPN\n")
	f.WriteString(fmt.Sprintf("DATABASE_URL=%s\n", databaseURL))

	log.Printf("✓ DATABASE_URL added to .env file")
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

// findAvailablePostgresPort finds an available port for PostgreSQL
func findAvailablePostgresPort() string {
	// Try standard port first
	if isPortAvailable("5432") {
		log.Println("✓ Port 5432 available")
		return "5432"
	}

	log.Println("Port 5432 in use, trying alternatives...")

	// Try common alternatives
	for _, port := range []string{"5433", "5434", "5435", "5436"} {
		if isPortAvailable(port) {
			log.Printf("✓ Found available port: %s", port)
			return port
		}
	}

	// Find any available port between 5437-5450
	for port := 5437; port <= 5450; port++ {
		portStr := fmt.Sprintf("%d", port)
		if isPortAvailable(portStr) {
			log.Printf("✓ Found available port: %s", portStr)
			return portStr
		}
	}

	// No ports available, return default (will fail with clear error)
	log.Println("⚠️  No available ports found between 5432-5450, will attempt 5432")
	return "5432"
}
