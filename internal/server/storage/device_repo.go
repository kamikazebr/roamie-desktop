package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type DeviceRepository struct {
	db *DB
}

func NewDeviceRepository(db *DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Create(ctx context.Context, device *models.Device) error {
	// Parse device_name to extract hardware_id and os_type
	device.ParseDeviceName()

	query := `
		INSERT INTO devices (id, user_id, device_name, hardware_id, os_type, public_key, vpn_ip, username, active, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id, created_at, last_seen
	`
	return r.db.QueryRowContext(ctx, query,
		device.ID, device.UserID, device.DeviceName, device.HardwareID, device.OSType,
		device.PublicKey, device.VpnIP, device.Username, device.Active,
	).Scan(&device.ID, &device.CreatedAt, &device.LastSeen)
}

func (r *DeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	var device models.Device
	query := `SELECT * FROM devices WHERE id = $1 AND active = true`
	err := r.db.GetContext(ctx, &device, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) GetByPublicKey(ctx context.Context, publicKey string) (*models.Device, error) {
	var device models.Device
	query := `SELECT * FROM devices WHERE public_key = $1 AND active = true`
	err := r.db.GetContext(ctx, &device, query, publicKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Device, error) {
	var devices []models.Device
	query := `SELECT * FROM devices WHERE user_id = $1 AND active = true ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &devices, query, userID)
	return devices, err
}

func (r *DeviceRepository) GetByUserAndName(ctx context.Context, userID uuid.UUID, deviceName string) (*models.Device, error) {
	var device models.Device
	query := `SELECT * FROM devices WHERE user_id = $1 AND device_name = $2 AND active = true`
	err := r.db.GetContext(ctx, &device, query, userID, deviceName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM devices WHERE user_id = $1 AND active = true`
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}

func (r *DeviceRepository) GetAllActiveIPs(ctx context.Context) ([]string, error) {
	var ips []string
	query := `SELECT vpn_ip FROM devices WHERE active = true`
	err := r.db.SelectContext(ctx, &ips, query)
	return ips, err
}

func (r *DeviceRepository) Update(ctx context.Context, device *models.Device) error {
	query := `
		UPDATE devices
		SET device_name = $1, last_handshake = $2, active = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query,
		device.DeviceName, device.LastHandshake, device.Active, device.ID)
	return err
}

func (r *DeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM devices WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *DeviceRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE devices SET active = false WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// UpdateLastSeen updates the last_seen timestamp for heartbeat tracking
func (r *DeviceRepository) UpdateLastSeen(ctx context.Context, deviceID uuid.UUID) error {
	query := `UPDATE devices SET last_seen = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, deviceID)
	return err
}

// GetAllTunnelPorts returns all currently allocated tunnel ports
// Used by TunnelPortPool to find available ports
func (r *DeviceRepository) GetAllTunnelPorts(ctx context.Context) ([]int, error) {
	var ports []int
	query := `SELECT tunnel_port FROM devices WHERE tunnel_port IS NOT NULL AND active = true`
	err := r.db.SelectContext(ctx, &ports, query)
	return ports, err
}

// UpdateTunnelPort assigns a tunnel port to a device
func (r *DeviceRepository) UpdateTunnelPort(ctx context.Context, deviceID uuid.UUID, port int) error {
	query := `UPDATE devices SET tunnel_port = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, port, deviceID)
	return err
}

// GetByUserAndHardwareID finds a device by user_id and hardware_id
// Used to prevent duplicate device registrations
func (r *DeviceRepository) GetByUserAndHardwareID(ctx context.Context, userID uuid.UUID, hardwareID string) (*models.Device, error) {
	var device models.Device
	query := `SELECT * FROM devices WHERE user_id = $1 AND hardware_id = $2 AND active = true`
	err := r.db.GetContext(ctx, &device, query, userID, hardwareID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

// GetByTunnelSSHKey finds a device by its SSH public key
// Used by tunnel server for authentication
func (r *DeviceRepository) GetByTunnelSSHKey(ctx context.Context, sshKey string) (*models.Device, error) {
	var device models.Device
	query := `SELECT * FROM devices WHERE tunnel_ssh_key = $1 AND active = true`
	err := r.db.GetContext(ctx, &device, query, sshKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

// UpdateTunnelSSHKey updates the SSH public key for a device
func (r *DeviceRepository) UpdateTunnelSSHKey(ctx context.Context, deviceID uuid.UUID, sshKey string) error {
	query := `UPDATE devices SET tunnel_ssh_key = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, sshKey, deviceID)
	return err
}

// UpdateTunnelEnabled enables or disables tunnel for a device
func (r *DeviceRepository) UpdateTunnelEnabled(ctx context.Context, deviceID uuid.UUID, enabled bool) error {
	query := `UPDATE devices SET tunnel_enabled = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, enabled, deviceID)
	return err
}

// GetByTunnelPort finds a device by its allocated tunnel port
// Used by tunnel server for authorization checks
func (r *DeviceRepository) GetByTunnelPort(ctx context.Context, port int) (*models.Device, error) {
	var device models.Device
	query := `SELECT * FROM devices WHERE tunnel_port = $1 AND active = true`
	err := r.db.GetContext(ctx, &device, query, port)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}
