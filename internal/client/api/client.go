package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrDeviceDeleted is returned when the device has been deleted from the server
var ErrDeviceDeleted = errors.New("device_deleted")

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HealthCheck checks if the server is reachable
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("failed to reach server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

// CreateDeviceRequest creates a new device authorization challenge
func (c *Client) CreateDeviceRequest(deviceID, hostname, username, publicKey, osType, hardwareID string) (*DeviceRequestResponse, error) {
	reqBody := map[string]interface{}{
		"device_id": deviceID,
		"hostname":  hostname,
	}

	// Add username if provided (for SSH host creation)
	if username != "" {
		reqBody["username"] = username
	}

	// Add public key if provided (for auto-registration)
	if publicKey != "" {
		reqBody["public_key"] = publicKey
	}

	// Add OS type if provided
	if osType != "" {
		reqBody["os_type"] = osType
	}

	// Add hardware ID if provided
	if hardwareID != "" {
		reqBody["hardware_id"] = hardwareID
	}

	body, _ := json.Marshal(reqBody)

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/auth/device-request",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result DeviceRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PollChallenge polls the status of a device challenge
func (c *Client) PollChallenge(challengeID string) (*PollResponse, error) {
	resp, err := c.httpClient.Get(
		c.baseURL + "/api/auth/device-poll/" + challengeID,
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result PollResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RefreshJWT refreshes the JWT using a refresh token
func (c *Client) RefreshJWT(refreshToken string) (*RefreshResponse, error) {
	reqBody := map[string]string{
		"refresh_token": refreshToken,
	}

	body, _ := json.Marshal(reqBody)

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/auth/refresh",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result RefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetSSHKeys fetches SSH public keys from the server
// Requires JWT token for authentication
func (c *Client) GetSSHKeys(jwt string) ([]SSHKey, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/ssh/keys", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add JWT authorization header
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result SSHKeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Keys, nil
}

// GetTunnelAuthorizedKeys fetches authorized tunnel SSH keys from the server
// Returns keys for all devices in the same user account (for tunnel access authorization)
// Requires JWT token for authentication
func (c *Client) GetTunnelAuthorizedKeys(jwt string) ([]TunnelAuthorizedKey, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/tunnel/authorized-keys", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tunnel authorized keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var result TunnelAuthorizedKeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Keys, nil
}

// DeleteDevice deletes a device from the server
// Requires JWT token for authentication
func (c *Client) DeleteDevice(deviceID, jwt string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/devices/"+deviceID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add JWT authorization header
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ValidateDevice checks if the current device still exists on the server
// Requires JWT token for authentication
func (c *Client) ValidateDevice(deviceID, jwt string) (*ValidateDeviceResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/devices/validate?device_id="+deviceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add JWT authorization header
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 404 - device was deleted
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrDeviceDeleted
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ValidateDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Response types

type DeviceRequestResponse struct {
	ChallengeID string `json:"challenge_id"`
	QRData      string `json:"qr_data"`
	ExpiresIn   int    `json:"expires_in"`
}

type PollResponse struct {
	Status          string      `json:"status"`
	JWT             string      `json:"jwt,omitempty"`
	RefreshToken    string      `json:"refresh_token,omitempty"`
	ExpiresAt       string      `json:"expires_at,omitempty"`
	Message         string      `json:"message,omitempty"`
	AutoRegistered  bool        `json:"auto_registered,omitempty"`
	Device          *DeviceInfo `json:"device,omitempty"`
	ServerPublicKey string      `json:"server_public_key,omitempty"`
	ServerEndpoint  string      `json:"server_endpoint,omitempty"`
	AllowedIPs      string      `json:"allowed_ips,omitempty"`
}

type DeviceInfo struct {
	ID         string `json:"id"`
	DeviceName string `json:"device_name"`
	VpnIP      string `json:"vpn_ip"`
	Username   string `json:"username,omitempty"`
	Active     bool   `json:"active"`
}

type RefreshResponse struct {
	JWT       string `json:"jwt"`
	ExpiresAt string `json:"expires_at"`
}

type SSHKey struct {
	PublicKey   string `json:"publicKey"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
}

type SSHKeysResponse struct {
	Keys []SSHKey `json:"keys"`
}

type TunnelAuthorizedKey struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	PublicKey  string `json:"public_key"`
	Comment    string `json:"comment"`
}

type TunnelAuthorizedKeysResponse struct {
	Keys []TunnelAuthorizedKey `json:"keys"`
}

type ValidateDeviceResponse struct {
	DeviceID string      `json:"device_id"`
	Status   string      `json:"status"`
	Device   *DeviceInfo `json:"device,omitempty"`
}

// SendHeartbeat sends a heartbeat to the server to mark the device as online
// Should be called every 30 seconds
func (c *Client) SendHeartbeat(deviceID, jwt string) error {
	reqBody := map[string]string{
		"device_id": deviceID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/devices/heartbeat", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RegisterTunnelKey registers an SSH public key for tunnel authentication
func (c *Client) RegisterTunnelKey(deviceID, publicKey, jwt string) error {
	reqBody := map[string]string{
		"device_id":  deviceID,
		"public_key": publicKey,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/tunnel/register-key", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// TunnelInfo contains information about a tunnel
type TunnelInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Port       int    `json:"tunnel_port"`
	VpnIP      string `json:"vpn_ip"`
	LastSeen   string `json:"last_seen"`
	Enabled    bool   `json:"enabled"`
	Connected  bool   `json:"connected"`
}

// TunnelStatusResponse contains the tunnel status
type TunnelStatusResponse struct {
	Tunnels []TunnelInfo `json:"tunnels"`
}

// GetTunnelStatus gets the tunnel status from the server
func (c *Client) GetTunnelStatus(jwt string) (*TunnelStatusResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/tunnel/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result TunnelStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// TunnelRegisterResponse contains the response from tunnel registration
type TunnelRegisterResponse struct {
	TunnelPort int    `json:"tunnel_port"`
	ServerHost string `json:"server_host"`
	Message    string `json:"message"`
}

// RegisterTunnel allocates a tunnel port for a device
func (c *Client) RegisterTunnel(deviceID, jwt string) (*TunnelRegisterResponse, error) {
	reqBody := map[string]string{
		"device_id": deviceID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/tunnel/register", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result TunnelRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// EnableTunnel enables the tunnel for a device
func (c *Client) EnableTunnel(deviceID, jwt string) error {
	req, err := http.NewRequest("PATCH", c.baseURL+"/api/devices/"+deviceID+"/tunnel/enable", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DisableTunnel disables the tunnel for a device
func (c *Client) DisableTunnel(deviceID, jwt string) error {
	req, err := http.NewRequest("PATCH", c.baseURL+"/api/devices/"+deviceID+"/tunnel/disable", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// PendingDiagnosticsRequest represents a pending diagnostics request
type PendingDiagnosticsRequest struct {
	RequestID  string `json:"request_id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// PendingDiagnosticsResponse represents the response from GET /api/devices/diagnostics/pending
type PendingDiagnosticsResponse struct {
	PendingRequests []PendingDiagnosticsRequest `json:"pending_requests"`
	Count           int                         `json:"count"`
}

// GetPendingDiagnostics fetches all pending diagnostics requests for the authenticated user's devices
func (c *Client) GetPendingDiagnostics(jwt string) (*PendingDiagnosticsResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/devices/diagnostics/pending", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PendingDiagnosticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UploadDiagnosticsReport uploads a diagnostics report to the server
func (c *Client) UploadDiagnosticsReport(jwt string, report interface{}) error {
	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/devices/diagnostics/report", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// PendingUpgradeRequest represents a pending upgrade request
type PendingUpgradeRequest struct {
	RequestID     string `json:"request_id"`
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name"`
	TargetVersion string `json:"target_version,omitempty"`
}

// PendingUpgradesResponse represents the response from GET /api/devices/upgrades/pending
type PendingUpgradesResponse struct {
	PendingUpgrades []PendingUpgradeRequest `json:"pending_upgrades"`
	Count           int                     `json:"count"`
}

// GetPendingUpgrades fetches all pending upgrade requests for the authenticated user's devices
func (c *Client) GetPendingUpgrades(jwt string) (*PendingUpgradesResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/devices/upgrades/pending", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PendingUpgradesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UploadUpgradeResult uploads an upgrade result to the server
func (c *Client) UploadUpgradeResult(jwt string, result interface{}) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/devices/upgrades/result", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
