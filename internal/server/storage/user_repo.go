package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (email, subnet, max_devices, active)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		user.Email, user.Subnet, user.MaxDevices, user.Active,
	).Scan(&user.ID, &user.CreatedAt)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE email = $1 AND active = true`
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE id = $1 AND active = true`
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListAll(ctx context.Context) ([]models.User, error) {
	var users []models.User
	query := `SELECT * FROM users WHERE active = true ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &users, query)
	return users, err
}

func (r *UserRepository) GetAllSubnets(ctx context.Context) ([]string, error) {
	var subnets []string
	query := `SELECT subnet FROM users WHERE active = true`
	err := r.db.SelectContext(ctx, &subnets, query)
	return subnets, err
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET max_devices = $1, active = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, user.MaxDevices, user.Active, user.ID)
	return err
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET active = false WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *UserRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE firebase_uid = $1 AND active = true`
	err := r.db.GetContext(ctx, &user, query, firebaseUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) CreateWithFirebaseUID(ctx context.Context, user *models.User, firebaseUID string) error {
	query := `
		INSERT INTO users (email, subnet, max_devices, active, firebase_uid)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		user.Email, user.Subnet, user.MaxDevices, user.Active, firebaseUID,
	).Scan(&user.ID, &user.CreatedAt)
}
