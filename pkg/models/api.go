package models

// Auth API types
type RequestCodeRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type RequestCodeResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expires_in"`
}

type VerifyCodeRequest struct {
	Email string `json:"email" validate:"required,email"`
	Code  string `json:"code" validate:"required,len=6"`
}

type VerifyCodeResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// Device API types
type RegisterDeviceRequest struct {
	DeviceName string `json:"device_name" validate:"required,min=1,max=100"`
	PublicKey  string `json:"public_key" validate:"required,len=44"`

	// Structured device fields (optional for backward compatibility)
	// If provided, these override parsing from device_name
	OSType      *string `json:"os_type,omitempty" validate:"omitempty,oneof=android ios linux macos windows"`
	HardwareID  *string `json:"hardware_id,omitempty" validate:"omitempty,min=4,max=16"`
	DisplayName *string `json:"display_name,omitempty" validate:"omitempty,max=100"`
}

type RegisterDeviceResponse struct {
	DeviceID        string `json:"device_id"`
	VpnIP           string `json:"vpn_ip"`
	UserSubnet      string `json:"user_subnet"`
	ServerPublicKey string `json:"server_public_key"`
	ServerEndpoint  string `json:"server_endpoint"`
	AllowedIPs      string `json:"allowed_ips"`
}

type ListDevicesResponse struct {
	UserSubnet string   `json:"user_subnet"`
	Devices    []Device `json:"devices"`
}

type DeviceConfigResponse struct {
	Config string `json:"config"`
}

// Admin API types
type NetworkScanResponse struct {
	ScannedAt       string            `json:"scanned_at"`
	Conflicts       []NetworkConflict `json:"conflicts"`
	AvailableRanges []string          `json:"available_ranges"`
}

type AddConflictRequest struct {
	CIDR        string `json:"cidr" validate:"required"`
	Source      string `json:"source" validate:"required"`
	Description string `json:"description"`
}

// Error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
