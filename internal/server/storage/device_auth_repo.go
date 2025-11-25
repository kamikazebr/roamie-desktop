package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type DeviceAuthRepository struct {
	db *DB
}

func NewDeviceAuthRepository(db *DB) *DeviceAuthRepository {
	return &DeviceAuthRepository{db: db}
}

// CreateChallenge creates a new device authorization challenge
func (r *DeviceAuthRepository) CreateChallenge(ctx context.Context, challenge *models.DeviceAuthChallenge) error {
	query := `
		INSERT INTO device_auth_challenges (id, device_id, hostname, ip_address, username, public_key, os_type, hardware_id, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at
	`
	return r.db.QueryRowContext(ctx, query,
		challenge.ID, challenge.DeviceID, challenge.Hostname,
		challenge.IPAddress, challenge.Username, challenge.PublicKey,
		challenge.OSType, challenge.HardwareID,
		challenge.Status, challenge.ExpiresAt,
	).Scan(&challenge.CreatedAt)
}

// GetChallenge gets a challenge by ID
func (r *DeviceAuthRepository) GetChallenge(ctx context.Context, id uuid.UUID) (*models.DeviceAuthChallenge, error) {
	var challenge models.DeviceAuthChallenge
	query := `SELECT * FROM device_auth_challenges WHERE id = $1`
	err := r.db.GetContext(ctx, &challenge, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &challenge, nil
}

// ListPendingChallenges lists all pending challenges
func (r *DeviceAuthRepository) ListPendingChallenges(ctx context.Context) ([]*models.DeviceAuthChallenge, error) {
	var challenges []*models.DeviceAuthChallenge
	query := `
		SELECT * FROM device_auth_challenges
		WHERE status = 'pending' AND expires_at > NOW()
		ORDER BY created_at DESC
	`
	err := r.db.SelectContext(ctx, &challenges, query)
	return challenges, err
}

// UpdateChallengeStatus updates the status of a challenge
func (r *DeviceAuthRepository) UpdateChallengeStatus(ctx context.Context, id uuid.UUID, status string, userID *uuid.UUID) error {
	query := `
		UPDATE device_auth_challenges
		SET status = $1, user_id = $2, approved_at = NOW()
		WHERE id = $3 AND status = 'pending' AND expires_at > NOW()
	`
	result, err := r.db.ExecContext(ctx, query, status, userID, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("challenge not found or already processed")
	}

	return nil
}

// ExpireOldChallenges marks old challenges as expired
func (r *DeviceAuthRepository) ExpireOldChallenges(ctx context.Context) error {
	query := `
		UPDATE device_auth_challenges
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at < NOW()
	`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// UpdateChallengeDeviceID updates the WireGuard device ID for a challenge
func (r *DeviceAuthRepository) UpdateChallengeDeviceID(ctx context.Context, challengeID uuid.UUID, deviceID *uuid.UUID) error {
	query := `
		UPDATE device_auth_challenges
		SET wg_device_id = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, deviceID, challengeID)
	return err
}

// CreateRefreshToken creates a new refresh token
func (r *DeviceAuthRepository) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, device_id, token, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`
	return r.db.QueryRowContext(ctx, query,
		token.ID, token.UserID, token.DeviceID, token.Token, token.ExpiresAt,
	).Scan(&token.CreatedAt)
}

// GetRefreshToken gets a refresh token by token string
func (r *DeviceAuthRepository) GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	query := `
		SELECT * FROM refresh_tokens
		WHERE token = $1 AND expires_at > NOW()
	`
	err := r.db.GetContext(ctx, &rt, query, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rt, nil
}

// UpdateRefreshTokenLastUsed updates the last_used_at timestamp
func (r *DeviceAuthRepository) UpdateRefreshTokenLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE refresh_tokens SET last_used_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteRefreshToken deletes a refresh token
func (r *DeviceAuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

// ListUserRefreshTokens lists all refresh tokens for a user
func (r *DeviceAuthRepository) ListUserRefreshTokens(ctx context.Context, userID uuid.UUID) ([]*models.RefreshToken, error) {
	var tokens []*models.RefreshToken
	query := `
		SELECT * FROM refresh_tokens
		WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY created_at DESC
	`
	err := r.db.SelectContext(ctx, &tokens, query, userID)
	return tokens, err
}

// DeleteRefreshTokensByDeviceID deletes all refresh tokens for a device
func (r *DeviceAuthRepository) DeleteRefreshTokensByDeviceID(ctx context.Context, deviceID uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE device_id = $1`
	_, err := r.db.ExecContext(ctx, query, deviceID)
	return err
}

// DeleteChallengesByDeviceID deletes all device auth challenges for a device
func (r *DeviceAuthRepository) DeleteChallengesByDeviceID(ctx context.Context, deviceID uuid.UUID) error {
	query := `DELETE FROM device_auth_challenges WHERE device_id = $1`
	_, err := r.db.ExecContext(ctx, query, deviceID)
	return err
}
