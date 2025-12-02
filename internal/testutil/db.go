package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// TestDB wraps the database connection for test utilities
type TestDB struct {
	DB *sqlx.DB
	t  *testing.T
}

// GetTestDB connects to the test database and returns a TestDB wrapper.
// If the database is not available, the test will be skipped.
func GetTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Try environment variable first, then default to e2e database
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://roamie:roamie_test_password@localhost:5436/roamie_vpn?sslmode=disable"
	}

	sqlxDB, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil
	}

	return &TestDB{DB: sqlxDB, t: t}
}

// Close closes the database connection
func (tdb *TestDB) Close() {
	if tdb.DB != nil {
		tdb.DB.Close()
	}
}

// CleanupTable deletes all rows from a table. Use with caution.
func (tdb *TestDB) CleanupTable(ctx context.Context, table string) {
	tdb.t.Helper()
	_, err := tdb.DB.ExecContext(ctx, "DELETE FROM "+table)
	if err != nil {
		tdb.t.Logf("Warning: failed to cleanup table %s: %v", table, err)
	}
}

// Exec executes a query and logs any errors
func (tdb *TestDB) Exec(ctx context.Context, query string, args ...interface{}) {
	tdb.t.Helper()
	_, err := tdb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		tdb.t.Fatalf("Failed to execute query: %v", err)
	}
}

// StorageDB returns a storage.DB wrapper for use with repositories
func (tdb *TestDB) StorageDB() *storage.DB {
	return &storage.DB{DB: tdb.DB}
}

// Repositories creates all standard repositories for testing
func (tdb *TestDB) Repositories() *TestRepositories {
	db := tdb.StorageDB()
	return &TestRepositories{
		Users:      storage.NewUserRepository(db),
		Devices:    storage.NewDeviceRepository(db),
		DeviceAuth: storage.NewDeviceAuthRepository(db),
		Conflicts:  storage.NewConflictRepository(db),
		Auth:       storage.NewAuthRepository(db),
	}
}

// TestRepositories contains all repositories for testing
type TestRepositories struct {
	Users      *storage.UserRepository
	Devices    *storage.DeviceRepository
	DeviceAuth *storage.DeviceAuthRepository
	Conflicts  *storage.ConflictRepository
	Auth       *storage.AuthRepository
}
