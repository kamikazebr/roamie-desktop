package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
)

type AuthRepository struct {
	db *DB
}

func NewAuthRepository(db *DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) CreateCode(ctx context.Context, code *models.AuthCode) error {
	query := `
		INSERT INTO auth_codes (email, code, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		code.Email, code.Code, code.ExpiresAt,
	).Scan(&code.ID, &code.CreatedAt)
}

func (r *AuthRepository) GetValidCode(ctx context.Context, email, code string) (*models.AuthCode, error) {
	var authCode models.AuthCode
	query := `
		SELECT * FROM auth_codes
		WHERE email = $1 AND code = $2 AND used = false AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := r.db.GetContext(ctx, &authCode, query, email, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &authCode, nil
}

func (r *AuthRepository) GetCode(ctx context.Context, email, code string) (*models.AuthCode, error) {
	var authCode models.AuthCode
	query := `
		SELECT * FROM auth_codes
		WHERE email = $1 AND code = $2
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := r.db.GetContext(ctx, &authCode, query, email, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &authCode, nil
}

func (r *AuthRepository) MarkCodeUsed(ctx context.Context, id string) error {
	query := `UPDATE auth_codes SET used = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *AuthRepository) DeleteExpiredCodes(ctx context.Context) error {
	query := `DELETE FROM auth_codes WHERE expires_at < NOW()`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *AuthRepository) InvalidateUserCodes(ctx context.Context, email string) error {
	query := `UPDATE auth_codes SET used = true WHERE email = $1 AND used = false`
	_, err := r.db.ExecContext(ctx, query, email)
	return err
}

func (r *AuthRepository) CleanupOldCodes(ctx context.Context, olderThan time.Duration) error {
	query := `DELETE FROM auth_codes WHERE created_at < $1`
	cutoff := time.Now().UTC().Add(-olderThan)
	_, err := r.db.ExecContext(ctx, query, cutoff)
	return err
}
