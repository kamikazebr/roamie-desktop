package api

import (
	"log"
	"net/http"
	"os"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TunnelHandler handles SSH reverse tunnel related requests
type TunnelHandler struct {
	deviceRepo     *storage.DeviceRepository
	deviceService  *services.DeviceService
	tunnelPortPool *services.TunnelPortPool
	tunnelService  *services.TunnelService
}

func NewTunnelHandler(
	deviceRepo *storage.DeviceRepository,
	deviceService *services.DeviceService,
	tunnelPortPool *services.TunnelPortPool,
	tunnelService *services.TunnelService,
) *TunnelHandler {
	return &TunnelHandler{
		deviceRepo:     deviceRepo,
		deviceService:  deviceService,
		tunnelPortPool: tunnelPortPool,
		tunnelService:  tunnelService,
	}
}

// Register allocates a tunnel port for a device
// POST /api/tunnel/register
// Body: {"device_id": "uuid"}
// Response: {"tunnel_port": 10001, "server_host": "1.2.3.4"}
func (h *TunnelHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Check if already has tunnel port allocated
	if device.TunnelPort != nil {
		log.Printf("Device %s already has tunnel port %d", device.ID, *device.TunnelPort)

		serverHost := os.Getenv("TUNNEL_SERVER_HOST")
		if serverHost == "" {
			serverHost = os.Getenv("WG_SERVER_PUBLIC_ENDPOINT")
			if serverHost != "" {
				// Extract just the host part (remove port if present)
				if idx := len(serverHost) - 1; idx > 0 {
					for i := len(serverHost) - 1; i >= 0; i-- {
						if serverHost[i] == ':' {
							serverHost = serverHost[:i]
							break
						}
					}
				}
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"tunnel_port": *device.TunnelPort,
			"server_host": serverHost,
			"message":     "using existing tunnel port",
		})
		return
	}

	// Allocate new port
	port, err := h.tunnelPortPool.AllocatePort(r.Context())
	if err != nil {
		log.Printf("Failed to allocate tunnel port for device %s: %v", device.ID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "no available tunnel ports")
		return
	}

	// Save to database
	if err := h.deviceRepo.UpdateTunnelPort(r.Context(), device.ID, port); err != nil {
		log.Printf("Failed to save tunnel port %d for device %s: %v", port, device.ID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to register tunnel")
		return
	}

	log.Printf("Allocated tunnel port %d for device %s (user: %s)", port, device.ID, claims.UserID)

	serverHost := os.Getenv("TUNNEL_SERVER_HOST")
	if serverHost == "" {
		serverHost = os.Getenv("WG_SERVER_PUBLIC_ENDPOINT")
		if serverHost != "" {
			// Extract just the host part (remove port if present)
			if idx := len(serverHost) - 1; idx > 0 {
				for i := len(serverHost) - 1; i >= 0; i-- {
					if serverHost[i] == ':' {
						serverHost = serverHost[:i]
						break
					}
				}
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tunnel_port": port,
		"server_host": serverHost,
		"message":     "tunnel port allocated",
	})
}

// GetStatus returns tunnel status for all user's devices
// GET /api/tunnel/status
func (h *TunnelHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	devices, err := h.deviceRepo.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get devices")
		return
	}

	var tunnelDevices []map[string]interface{}
	for _, device := range devices {
		if device.TunnelPort != nil {
			tunnelDevices = append(tunnelDevices, map[string]interface{}{
				"device_id":   device.ID.String(),
				"device_name": device.DeviceName,
				"tunnel_port": *device.TunnelPort,
				"vpn_ip":      device.VpnIP,
				"last_seen":   device.LastSeen,
			})
		}
	}

	serverHost := os.Getenv("TUNNEL_SERVER_HOST")
	if serverHost == "" {
		serverHost = "server"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tunnels":     tunnelDevices,
		"server_host": serverHost,
	})
}

// RegisterKey registers an SSH public key for a device
// POST /api/tunnel/register-key
// Body: {"device_id": "uuid", "public_key": "ssh-rsa ..."}
func (h *TunnelHandler) RegisterKey(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		DeviceID  string `json:"device_id"`
		PublicKey string `json:"public_key"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Update SSH key
	if err := h.deviceRepo.UpdateTunnelSSHKey(r.Context(), device.ID, req.PublicKey); err != nil {
		log.Printf("Failed to update SSH key for device %s: %v", device.ID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to register SSH key")
		return
	}

	log.Printf("Registered SSH key for device %s (user: %s)", device.ID, claims.UserID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "SSH key registered successfully",
	})
}

// EnableTunnel enables the tunnel for a specific device
// PATCH /api/devices/{device_id}/tunnel/enable
func (h *TunnelHandler) EnableTunnel(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deviceIDStr := chi.URLParam(r, "device_id")
	if deviceIDStr == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id is required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Enable tunnel
	if err := h.deviceRepo.UpdateTunnelEnabled(r.Context(), device.ID, true); err != nil {
		log.Printf("Failed to enable tunnel for device %s: %v", device.ID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to enable tunnel")
		return
	}

	log.Printf("Enabled tunnel for device %s (user: %s)", device.ID, claims.UserID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "tunnel enabled",
	})
}

// DisableTunnel disables the tunnel for a specific device
// PATCH /api/devices/{device_id}/tunnel/disable
func (h *TunnelHandler) DisableTunnel(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deviceIDStr := chi.URLParam(r, "device_id")
	if deviceIDStr == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id is required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Disable tunnel
	if err := h.deviceRepo.UpdateTunnelEnabled(r.Context(), device.ID, false); err != nil {
		log.Printf("Failed to disable tunnel for device %s: %v", device.ID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to disable tunnel")
		return
	}

	log.Printf("Disabled tunnel for device %s (user: %s)", device.ID, claims.UserID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "tunnel disabled",
	})
}

// GetAuthorizedKeys returns all tunnel SSH keys for devices in the user's account
// GET /api/tunnel/authorized-keys
// Response: [{"device_id": "uuid", "device_name": "...", "public_key": "ssh-rsa ...", "comment": "..."}]
func (h *TunnelHandler) GetAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get all authorized tunnel keys for this user's devices
	keys, err := h.tunnelService.GetAuthorizedTunnelKeys(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("Failed to get authorized tunnel keys for user %s: %v", claims.UserID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get authorized keys")
		return
	}

	log.Printf("Retrieved %d authorized tunnel keys for user %s", len(keys), claims.UserID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}
