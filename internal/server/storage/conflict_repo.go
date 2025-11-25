package storage

import (
	"context"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type ConflictRepository struct {
	db *DB
}

func NewConflictRepository(db *DB) *ConflictRepository {
	return &ConflictRepository{db: db}
}

func (r *ConflictRepository) Create(ctx context.Context, conflict *models.NetworkConflict) error {
	query := `
		INSERT INTO network_conflicts (cidr, source, description, active)
		VALUES ($1, $2, $3, $4)
		RETURNING id, detected_at
	`
	return r.db.QueryRowContext(ctx, query,
		conflict.CIDR, conflict.Source, conflict.Description, conflict.Active,
	).Scan(&conflict.ID, &conflict.DetectedAt)
}

func (r *ConflictRepository) GetAll(ctx context.Context) ([]models.NetworkConflict, error) {
	var conflicts []models.NetworkConflict
	query := `SELECT * FROM network_conflicts WHERE active = true ORDER BY detected_at DESC`
	err := r.db.SelectContext(ctx, &conflicts, query)
	return conflicts, err
}

func (r *ConflictRepository) GetAllCIDRs(ctx context.Context) ([]string, error) {
	var cidrs []string
	query := `SELECT cidr FROM network_conflicts WHERE active = true`
	err := r.db.SelectContext(ctx, &cidrs, query)
	return cidrs, err
}

func (r *ConflictRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE network_conflicts SET active = false WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ConflictRepository) DeleteBySource(ctx context.Context, source string) error {
	query := `UPDATE network_conflicts SET active = false WHERE source = $1`
	_, err := r.db.ExecContext(ctx, query, source)
	return err
}

func (r *ConflictRepository) Exists(ctx context.Context, cidr string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM network_conflicts WHERE cidr = $1 AND active = true`
	err := r.db.GetContext(ctx, &count, query, cidr)
	return count > 0, err
}
