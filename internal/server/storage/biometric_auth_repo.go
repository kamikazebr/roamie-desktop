package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type BiometricAuthRepository struct {
	db *DB
}

func NewBiometricAuthRepository(db *DB) *BiometricAuthRepository {
	return &BiometricAuthRepository{db: db}
}

// Create creates a new biometric auth request
func (r *BiometricAuthRepository) Create(ctx context.Context, req *models.BiometricAuthRequest) error {
	query := `
		INSERT INTO biometric_auth_requests (
			user_id, device_id, username, hostname, command,
			status, expires_at, ip_address
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		req.UserID, req.DeviceID, req.Username, req.Hostname, req.Command,
		req.Status, req.ExpiresAt, req.IPAddress,
	).Scan(&req.ID, &req.CreatedAt)
}

// GetByID retrieves a biometric auth request by ID
func (r *BiometricAuthRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.BiometricAuthRequest, error) {
	var req models.BiometricAuthRequest
	query := `SELECT * FROM biometric_auth_requests WHERE id = $1`
	err := r.db.GetContext(ctx, &req, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &req, nil
}

// ListPending lists all pending auth requests for a user
func (r *BiometricAuthRepository) ListPending(ctx context.Context, userID uuid.UUID) ([]models.BiometricAuthRequest, error) {
	var requests []models.BiometricAuthRequest
	query := `
		SELECT * FROM biometric_auth_requests
		WHERE user_id = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`
	err := r.db.SelectContext(ctx, &requests, query, userID)
	if err != nil {
		return nil, err
	}
	if requests == nil {
		requests = []models.BiometricAuthRequest{}
	}
	return requests, nil
}

// ListByUser lists all auth requests for a user with optional status filter
func (r *BiometricAuthRepository) ListByUser(ctx context.Context, userID uuid.UUID, status string, limit int) ([]models.BiometricAuthRequest, error) {
	var requests []models.BiometricAuthRequest
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT * FROM biometric_auth_requests
			WHERE user_id = $1 AND status = $2
			ORDER BY created_at DESC
			LIMIT $3
		`
		args = []interface{}{userID, status, limit}
	} else {
		query = `
			SELECT * FROM biometric_auth_requests
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit}
	}

	err := r.db.SelectContext(ctx, &requests, query, args...)
	if err != nil {
		return nil, err
	}
	if requests == nil {
		requests = []models.BiometricAuthRequest{}
	}
	return requests, nil
}

// UpdateStatus updates the status and response of an auth request
func (r *BiometricAuthRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, response *string) error {
	query := `
		UPDATE biometric_auth_requests
		SET status = $1, response = $2, responded_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, status, response, id)
	return err
}

// MarkExpired marks all expired pending requests as expired
func (r *BiometricAuthRepository) MarkExpired(ctx context.Context) (int, error) {
	query := `
		UPDATE biometric_auth_requests
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at <= NOW()
	`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}

// GetStats returns statistics about auth requests for a user
func (r *BiometricAuthRepository) GetStats(ctx context.Context, userID uuid.UUID, since time.Time) (*models.BiometricAuthStats, error) {
	query := `
		SELECT
			COUNT(*) as total_requests,
			COUNT(CASE WHEN status = 'approved' THEN 1 END) as approved_requests,
			COUNT(CASE WHEN status = 'denied' THEN 1 END) as denied_requests,
			COUNT(CASE WHEN status = 'expired' OR status = 'timeout' THEN 1 END) as expired_requests,
			COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending_requests
		FROM biometric_auth_requests
		WHERE user_id = $1 AND created_at >= $2
	`
	var stats models.BiometricAuthStats
	err := r.db.QueryRowContext(ctx, query, userID, since).Scan(
		&stats.TotalRequests,
		&stats.ApprovedRequests,
		&stats.DeniedRequests,
		&stats.ExpiredRequests,
		&stats.PendingRequests,
	)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// Delete removes an auth request (for testing/cleanup)
func (r *BiometricAuthRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM biometric_auth_requests WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteOld deletes auth requests older than the specified duration
func (r *BiometricAuthRepository) DeleteOld(ctx context.Context, olderThan time.Duration) (int, error) {
	query := `
		DELETE FROM biometric_auth_requests
		WHERE created_at < NOW() - $1::INTERVAL
	`
	result, err := r.db.ExecContext(ctx, query, olderThan.String())
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}
