package api

import (
	"log"
	"net/http"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/internal/server/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DeviceHandler struct {
	deviceService *services.DeviceService
	userRepo      *storage.UserRepository
	deviceRepo    *storage.DeviceRepository
	wgManager     *wireguard.Manager
	deviceCache   *services.DeviceCache
}

func NewDeviceHandler(
	deviceService *services.DeviceService,
	userRepo *storage.UserRepository,
	deviceRepo *storage.DeviceRepository,
	wgManager *wireguard.Manager,
	deviceCache *services.DeviceCache,
) *DeviceHandler {
	return &DeviceHandler{
		deviceService: deviceService,
		userRepo:      userRepo,
		deviceRepo:    deviceRepo,
		wgManager:     wgManager,
		deviceCache:   deviceCache,
	}
}

func (h *DeviceHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.RegisterDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Register device in database (no username from manual registration)
	result, err := h.deviceService.RegisterDevice(
		r.Context(),
		claims.UserID,
		req.DeviceName,
		req.PublicKey,
		nil, // username
		req.OSType,
		req.HardwareID,
		req.DisplayName,
		nil, // deviceID - let server generate for manual registration
	)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get user subnet
	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	// Add device to WireGuard (handles replacement and rollback)
	if err := services.AddDeviceToWireGuard(r.Context(), h.wgManager, h.deviceRepo, result, true); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to configure WireGuard: "+err.Error())
		return
	}

	response := models.RegisterDeviceResponse{
		DeviceID:        result.Device.ID.String(),
		VpnIP:           result.Device.VpnIP,
		UserSubnet:      user.Subnet,
		ServerPublicKey: h.wgManager.GetPublicKey(),
		ServerEndpoint:  h.wgManager.GetEndpoint(),
		AllowedIPs:      user.Subnet,
	}

	respondJSON(w, http.StatusCreated, response)
}

func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	devices, err := h.deviceService.GetUserDevices(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get devices")
		return
	}

	// Calculate is_online for each device from cache
	for i := range devices {
		devices[i].IsOnline = h.deviceCache.IsOnline(devices[i].ID.String())
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	response := models.ListDevicesResponse{
		UserSubnet: user.Subnet,
		Devices:    devices,
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *DeviceHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deviceIDStr := chi.URLParam(r, "device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device ID")
		return
	}

	// Get device to retrieve public key
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, err.Error())
		return
	}

	// Remove from WireGuard
	if err := h.wgManager.RemovePeer(device.PublicKey); err != nil {
		log.Printf("Warning: Failed to remove WireGuard peer for device %s: %v", device.ID, err)
		// Log error but continue with database deletion
	} else {
		log.Printf("Removed WireGuard peer for device %s (public key: %s)", device.ID, device.PublicKey)
	}

	// Delete from database (including refresh tokens and challenges in cascata cleanup)
	if err := h.deviceService.DeleteDevice(r.Context(), deviceID, claims.UserID); err != nil {
		log.Printf("Error: Failed to delete device %s for user %s: %v", deviceID, claims.UserID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to delete device")
		return
	}

	log.Printf("Device %s successfully deleted for user %s (WireGuard peer removed, refresh tokens and challenges cleaned up)", device.ID, claims.UserID)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Device revoked successfully",
	})
}

func (h *DeviceHandler) GetDeviceConfig(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	deviceIDStr := chi.URLParam(r, "device_id")
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device ID")
		return
	}

	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, err.Error())
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	// Generate config (client will fill in private key)
	config := wireguard.GenerateClientConfig(
		"<INSERT_YOUR_PRIVATE_KEY_HERE>",
		device.VpnIP,
		h.wgManager.GetPublicKey(),
		h.wgManager.GetEndpoint(),
		user.Subnet,
	)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(config))
}

func (h *DeviceHandler) ValidateDevice(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get the device associated with the current JWT
	// The device ID should be in the JWT claims (or we need to infer it somehow)
	// For now, we'll accept device_id as a query parameter or in the JWT
	deviceIDStr := r.URL.Query().Get("device_id")
	if deviceIDStr == "" {
		// Try to get from URL param
		deviceIDStr = chi.URLParam(r, "device_id")
	}

	if deviceIDStr == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device ID")
		return
	}

	// Check if device still exists and belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Device exists and is valid
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"device_id": device.ID.String(),
		"status":    "valid",
		"device":    device,
	})
}

// Heartbeat updates the last_seen timestamp for a device
// POST /api/devices/heartbeat
// Body: {"device_id": "uuid"}
func (h *DeviceHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
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

	// Update last_seen in database
	if err := h.deviceRepo.UpdateLastSeen(r.Context(), device.ID); err != nil {
		log.Printf("Failed to update last_seen for device %s: %v", device.ID, err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to update heartbeat")
		return
	}

	// Update cache (device considered online for next 90 seconds)
	h.deviceCache.MarkOnline(device.ID.String())

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
