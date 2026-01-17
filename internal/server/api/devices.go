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
	deviceService       *services.DeviceService
	userRepo            *storage.UserRepository
	deviceRepo          *storage.DeviceRepository
	wgManager           *wireguard.Manager
	deviceCache         *services.DeviceCache
	diagnosticsService  *services.DiagnosticsService
}

func NewDeviceHandler(
	deviceService *services.DeviceService,
	userRepo *storage.UserRepository,
	deviceRepo *storage.DeviceRepository,
	wgManager *wireguard.Manager,
	deviceCache *services.DeviceCache,
	diagnosticsService *services.DiagnosticsService,
) *DeviceHandler {
	return &DeviceHandler{
		deviceService:      deviceService,
		userRepo:           userRepo,
		deviceRepo:         deviceRepo,
		wgManager:          wgManager,
		deviceCache:        deviceCache,
		diagnosticsService: diagnosticsService,
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

// TriggerDoctor creates a diagnostics request for a device
func (h *DeviceHandler) TriggerDoctor(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get device_id from URL parameter and parse as UUID
	deviceIDStr := chi.URLParam(r, "device_id")
	if deviceIDStr == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id is required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id format")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "diagnostics service not available (Firebase not configured)")
		return
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Create diagnostics request in Firestore
	// Use DeviceName (the compound identifier) for Firestore lookups
	req := &services.DiagnosticsRequest{
		RequestID:   requestID,
		DeviceID:    device.DeviceName, // Use device name as identifier for Firestore
		UserID:      claims.UserID.String(),
		RequestedBy: "api", // Could be "mobile_app" or "dashboard" in future
		Status:      "pending",
	}

	if err := h.diagnosticsService.CreateDiagnosticsRequest(r.Context(), req); err != nil {
		log.Printf("Failed to create diagnostics request: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to create diagnostics request")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"request_id":  requestID,
		"device_id":   device.ID.String(),
		"device_name": device.DeviceName,
		"status":      "pending",
		"message":     "Diagnostics request created. Device daemon will process it within 30 seconds.",
	})
}

// GetDiagnosticsReport fetches a specific diagnostics report
func (h *DeviceHandler) GetDiagnosticsReport(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get device_id and request_id from URL parameters
	deviceIDStr := chi.URLParam(r, "device_id")
	requestID := chi.URLParam(r, "request_id")
	if deviceIDStr == "" || requestID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id and request_id are required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id format")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "diagnostics service not available (Firebase not configured)")
		return
	}

	// Fetch report from Firestore
	report, err := h.diagnosticsService.GetDiagnosticsReport(r.Context(), device.DeviceName, requestID)
	if err != nil {
		log.Printf("Failed to get diagnostics report: %v", err)
		respondErrorJSON(w, http.StatusNotFound, "diagnostics report not found")
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// UploadDiagnosticsReport accepts a diagnostics report from the client and saves it to Firestore
func (h *DeviceHandler) UploadDiagnosticsReport(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "diagnostics service not available (Firebase not configured)")
		return
	}

	// Decode report from request body
	var report services.DiagnosticsReport
	if err := decodeJSON(r, &report); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if report.DeviceID == "" || report.RequestID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id and request_id are required")
		return
	}

	// Find device by DeviceName (which is what we use as device_id in Firestore)
	devices, err := h.deviceRepo.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("Failed to get user devices: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to fetch devices")
		return
	}

	// Verify device belongs to user
	deviceFound := false
	for _, device := range devices {
		if device.DeviceName == report.DeviceID {
			deviceFound = true
			break
		}
	}

	if !deviceFound {
		respondErrorJSON(w, http.StatusForbidden, "device does not belong to user")
		return
	}

	// Save report to Firestore
	if err := h.diagnosticsService.SaveDiagnosticsReport(r.Context(), &report); err != nil {
		log.Printf("Failed to save diagnostics report: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to save diagnostics report")
		return
	}

	// Delete pending request
	if err := h.diagnosticsService.DeletePendingRequest(r.Context(), report.DeviceID, report.RequestID); err != nil {
		log.Printf("Failed to delete pending request: %v", err)
		// Don't fail the request, just log the error
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"request_id": report.RequestID,
		"device_id":  report.DeviceID,
		"message":    "Diagnostics report saved successfully",
	})
}

// GetPendingDiagnostics fetches all pending diagnostics requests for the authenticated user's devices
func (h *DeviceHandler) GetPendingDiagnostics(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "diagnostics service not available (Firebase not configured)")
		return
	}

	// Fetch all devices for the user
	devices, err := h.deviceRepo.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("Failed to get user devices: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to fetch devices")
		return
	}

	// Fetch pending requests for each device
	type PendingRequest struct {
		RequestID  string `json:"request_id"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
	}

	var allPending []PendingRequest
	for _, device := range devices {
		pending, err := h.diagnosticsService.GetPendingRequests(r.Context(), device.DeviceName)
		if err != nil {
			log.Printf("Failed to get pending requests for device %s: %v", device.DeviceName, err)
			continue // Skip this device but continue with others
		}

		for _, req := range pending {
			allPending = append(allPending, PendingRequest{
				RequestID:  req.RequestID,
				DeviceID:   device.ID.String(),
				DeviceName: device.DeviceName,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"pending_requests": allPending,
		"count":            len(allPending),
	})
}

// GetAllDiagnosticsReports fetches all diagnostics reports for a device
func (h *DeviceHandler) GetAllDiagnosticsReports(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get device_id from URL parameter
	deviceIDStr := chi.URLParam(r, "device_id")
	if deviceIDStr == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id is required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id format")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "diagnostics service not available (Firebase not configured)")
		return
	}

	// Fetch reports from Firestore (last 10)
	reports, err := h.diagnosticsService.GetAllDiagnosticsReports(r.Context(), device.DeviceName, 10)
	if err != nil {
		log.Printf("Failed to get diagnostics reports: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to fetch diagnostics reports")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":   device.ID.String(),
		"device_name": device.DeviceName,
		"reports":     reports,
		"count":       len(reports),
	})
}

// TriggerUpgrade creates an upgrade request for a device
func (h *DeviceHandler) TriggerUpgrade(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get device_id from URL parameter and parse as UUID
	deviceIDStr := chi.URLParam(r, "device_id")
	if deviceIDStr == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id is required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id format")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "upgrade service not available (Firebase not configured)")
		return
	}

	// Parse optional request body for target version
	var reqBody struct {
		TargetVersion string `json:"target_version,omitempty"`
	}
	_ = decodeJSON(r, &reqBody) // Ignore error - target_version is optional

	// Generate request ID
	requestID := uuid.New().String()

	// Create upgrade request in Firestore
	// Use DeviceName (the compound identifier) for Firestore lookups
	req := &services.UpgradeRequest{
		RequestID:     requestID,
		DeviceID:      device.DeviceName, // Use device name as identifier for Firestore
		UserID:        claims.UserID.String(),
		RequestedBy:   "api", // Could be "mobile_app" or "dashboard" in future
		Status:        "pending",
		TargetVersion: reqBody.TargetVersion,
	}

	if err := h.diagnosticsService.CreateUpgradeRequest(r.Context(), req); err != nil {
		log.Printf("Failed to create upgrade request: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to create upgrade request")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"request_id":     requestID,
		"device_id":      device.ID.String(),
		"device_name":    device.DeviceName,
		"status":         "pending",
		"target_version": reqBody.TargetVersion,
		"message":        "Upgrade request created. Device daemon will process it within 30 seconds.",
	})
}

// GetUpgradeResult fetches a specific upgrade result
func (h *DeviceHandler) GetUpgradeResult(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get device_id and request_id from URL parameters
	deviceIDStr := chi.URLParam(r, "device_id")
	requestID := chi.URLParam(r, "request_id")
	if deviceIDStr == "" || requestID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id and request_id are required")
		return
	}

	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id format")
		return
	}

	// Verify device belongs to user
	device, err := h.deviceService.GetDevice(r.Context(), deviceID, claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "device not found")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "upgrade service not available (Firebase not configured)")
		return
	}

	// Fetch result from Firestore
	result, err := h.diagnosticsService.GetUpgradeResult(r.Context(), device.DeviceName, requestID)
	if err != nil {
		log.Printf("Failed to get upgrade result: %v", err)
		respondErrorJSON(w, http.StatusNotFound, "upgrade result not found")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// UploadUpgradeResult accepts an upgrade result from the client and saves it to Firestore
func (h *DeviceHandler) UploadUpgradeResult(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "upgrade service not available (Firebase not configured)")
		return
	}

	// Decode result from request body
	var result services.UpgradeResult
	if err := decodeJSON(r, &result); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if result.DeviceID == "" || result.RequestID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id and request_id are required")
		return
	}

	// Find device by DeviceName (which is what we use as device_id in Firestore)
	devices, err := h.deviceRepo.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("Failed to get user devices: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to fetch devices")
		return
	}

	// Verify device belongs to user
	deviceFound := false
	for _, device := range devices {
		if device.DeviceName == result.DeviceID {
			deviceFound = true
			break
		}
	}

	if !deviceFound {
		respondErrorJSON(w, http.StatusForbidden, "device does not belong to user")
		return
	}

	// Save result to Firestore
	if err := h.diagnosticsService.SaveUpgradeResult(r.Context(), &result); err != nil {
		log.Printf("Failed to save upgrade result: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to save upgrade result")
		return
	}

	// Delete pending request
	if err := h.diagnosticsService.DeletePendingUpgrade(r.Context(), result.DeviceID, result.RequestID); err != nil {
		log.Printf("Failed to delete pending upgrade: %v", err)
		// Don't fail the request, just log the error
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"request_id": result.RequestID,
		"device_id":  result.DeviceID,
		"message":    "Upgrade result saved successfully",
	})
}

// GetPendingUpgrades fetches all pending upgrade requests for the authenticated user's devices
func (h *DeviceHandler) GetPendingUpgrades(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check if DiagnosticsService is available
	if h.diagnosticsService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "upgrade service not available (Firebase not configured)")
		return
	}

	// Fetch all devices for the user
	devices, err := h.deviceRepo.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("Failed to get user devices: %v", err)
		respondErrorJSON(w, http.StatusInternalServerError, "failed to fetch devices")
		return
	}

	// Fetch pending requests for each device
	type PendingUpgrade struct {
		RequestID     string `json:"request_id"`
		DeviceID      string `json:"device_id"`
		DeviceName    string `json:"device_name"`
		TargetVersion string `json:"target_version,omitempty"`
	}

	var allPending []PendingUpgrade
	for _, device := range devices {
		pending, err := h.diagnosticsService.GetPendingUpgrades(r.Context(), device.DeviceName)
		if err != nil {
			log.Printf("Failed to get pending upgrades for device %s: %v", device.DeviceName, err)
			continue // Skip this device but continue with others
		}

		for _, req := range pending {
			allPending = append(allPending, PendingUpgrade{
				RequestID:     req.RequestID,
				DeviceID:      device.ID.String(),
				DeviceName:    device.DeviceName,
				TargetVersion: req.TargetVersion,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"pending_upgrades": allPending,
		"count":            len(allPending),
	})
}
