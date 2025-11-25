package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/internal/server/wireguard"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DeviceAuthHandler struct {
	deviceAuthService *services.DeviceAuthService
	authService       *services.AuthService
	firebaseService   *services.FirebaseService
	deviceService     *services.DeviceService
	wgManager         *wireguard.Manager
	userRepo          *storage.UserRepository
	deviceRepo        *storage.DeviceRepository
}

func NewDeviceAuthHandler(
	deviceAuthService *services.DeviceAuthService,
	authService *services.AuthService,
	firebaseService *services.FirebaseService,
	deviceService *services.DeviceService,
	wgManager *wireguard.Manager,
	userRepo *storage.UserRepository,
	deviceRepo *storage.DeviceRepository,
) *DeviceAuthHandler {
	return &DeviceAuthHandler{
		deviceAuthService: deviceAuthService,
		authService:       authService,
		firebaseService:   firebaseService,
		deviceService:     deviceService,
		wgManager:         wgManager,
		userRepo:          userRepo,
		deviceRepo:        deviceRepo,
	}
}

// CreateDeviceRequest handles POST /api/auth/device-request (public, no auth required)
// Called from roamie to initiate device authorization
func (h *DeviceAuthHandler) CreateDeviceRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID   string  `json:"device_id"`
		Hostname   string  `json:"hostname"`
		Username   *string `json:"username,omitempty"`    // Optional: system username for SSH
		PublicKey  *string `json:"public_key,omitempty"`  // Optional: for auto-registration
		OSType     *string `json:"os_type,omitempty"`     // Optional: OS type (linux, macos, windows, etc.)
		HardwareID *string `json:"hardware_id,omitempty"` // Optional: 8-char hardware identifier
	}

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DeviceID == "" || req.Hostname == "" {
		respondErrorJSON(w, http.StatusBadRequest, "device_id and hostname are required")
		return
	}

	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid device_id format")
		return
	}

	// Extract client IP address
	ipAddress := getClientIP(r)

	// Create challenge
	challenge, err := h.deviceAuthService.CreateChallenge(r.Context(), deviceID, req.Hostname, ipAddress, req.Username, req.PublicKey, req.OSType, req.HardwareID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to create challenge")
		return
	}

	// Generate QR data - only challenge ID needed (mobile app already knows server)
	qrData := fmt.Sprintf("roamie://auth?challenge=%s", challenge.ID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"challenge_id": challenge.ID.String(),
		"qr_data":      qrData,
		"expires_in":   300, // 5 minutes in seconds
	})
}

// PollChallenge handles GET /api/auth/device-poll/{challenge_id} (public, no auth required)
// Called from roamie to poll for authorization status
func (h *DeviceAuthHandler) PollChallenge(w http.ResponseWriter, r *http.Request) {
	challengeIDStr := chi.URLParam(r, "challenge_id")
	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid challenge_id format")
		return
	}

	challenge, err := h.deviceAuthService.GetChallenge(r.Context(), challengeID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, "challenge not found")
		return
	}

	switch challenge.Status {
	case "approved":
		// Get user info
		user, err := h.authService.GetUserByID(r.Context(), *challenge.UserID)
		if err != nil {
			respondErrorJSON(w, http.StatusInternalServerError, "failed to get user")
			return
		}

		// Generate JWT (30 days)
		jwt, expiresAt, err := h.authService.GenerateToken(user.ID, user.Email)
		if err != nil {
			respondErrorJSON(w, http.StatusInternalServerError, "failed to generate JWT")
			return
		}

		deviceIDForToken := resolveRefreshTokenDeviceID(challenge)

		// Generate refresh token bound to the persisted device record
		refreshToken, err := h.deviceAuthService.GenerateRefreshToken(r.Context(), user.ID, deviceIDForToken)
		if err != nil {
			respondErrorJSON(w, http.StatusInternalServerError, "failed to generate refresh token")
			return
		}

		response := map[string]interface{}{
			"status":        "approved",
			"jwt":           jwt,
			"refresh_token": refreshToken,
			"expires_at":    expiresAt.Format("2006-01-02T15:04:05Z"),
		}

		// If device was auto-registered, include device info
		if challenge.WgDeviceID != nil && h.deviceService != nil {
			device, err := h.deviceService.GetDeviceByID(r.Context(), *challenge.WgDeviceID)
			if err == nil && device != nil {
				deviceInfo := map[string]interface{}{
					"id":          device.ID,
					"device_name": device.DeviceName,
					"vpn_ip":      device.VpnIP,
					"active":      device.Active,
				}

				// Include username if present in challenge
				if challenge.Username != nil && *challenge.Username != "" {
					deviceInfo["username"] = *challenge.Username
				}

				response["device"] = deviceInfo
				response["auto_registered"] = true

				// Include WireGuard connection info
				response["server_public_key"] = h.wgManager.GetPublicKey()
				response["server_endpoint"] = h.wgManager.GetEndpoint()
				response["allowed_ips"] = user.Subnet
			}
		}

		respondJSON(w, http.StatusOK, response)

	case "denied":
		respondJSON(w, http.StatusForbidden, map[string]interface{}{
			"status":  "denied",
			"message": "Device authorization denied by user",
		})

	case "expired":
		respondJSON(w, http.StatusGone, map[string]interface{}{
			"status":  "expired",
			"message": "Authorization request expired",
		})

	default: // pending
		respondJSON(w, http.StatusAccepted, map[string]interface{}{
			"status":  "pending",
			"message": "Waiting for user approval",
		})
	}
}

// ApproveDevice handles POST /api/device-auth/approve (requires JWT)
// Called from Flutter app to approve or deny a device authorization request.
//
// Intended flow:
// 1. Desktop runs `roamie login` → creates challenge
// 2. Desktop shows QR code containing challenge_id
// 3. User scans QR code with mobile app
// 4. Mobile app calls this endpoint with challenge_id from QR code
// 5. Challenge gets approved and associated with the authenticated user
func (h *DeviceAuthHandler) ApproveDevice(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ChallengeID string `json:"challenge_id"`
		Approved    bool   `json:"approved"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ChallengeID == "" {
		respondErrorJSON(w, http.StatusBadRequest, "challenge_id is required")
		return
	}

	challengeID, err := uuid.Parse(req.ChallengeID)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid challenge_id format")
		return
	}

	// Approve or deny the challenge
	if err := h.deviceAuthService.ApproveChallenge(r.Context(), challengeID, claims.UserID, req.Approved, h.wgManager, h.deviceRepo); err != nil {
		errMsg := err.Error()

		// Return proper HTTP status codes based on error type
		if errMsg == "challenge not found" {
			respondErrorJSON(w, http.StatusNotFound, errMsg)
			return
		}
		if errMsg == "challenge has expired" {
			respondErrorJSON(w, http.StatusGone, errMsg)
			return
		}
		if errMsg == "challenge already processed with different decision" {
			respondErrorJSON(w, http.StatusConflict, errMsg)
			return
		}

		// Generic internal server error for unexpected errors
		respondErrorJSON(w, http.StatusInternalServerError, "failed to process device authorization")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Device authorization recorded",
		"status":  map[bool]string{true: "approved", false: "denied"}[req.Approved],
	})
}

// RefreshJWT handles POST /api/auth/refresh (public, uses refresh token)
// Called from roamie daemon to refresh JWT
func (h *DeviceAuthHandler) RefreshJWT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		respondErrorJSON(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	// Validate refresh token
	token, err := h.deviceAuthService.ValidateRefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		respondErrorJSON(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Get user info
	user, err := h.authService.GetUserByID(r.Context(), token.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	// Generate new JWT (30 days)
	jwt, expiresAt, err := h.authService.GenerateToken(user.ID, user.Email)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to generate JWT")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jwt":        jwt,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Login handles POST /api/auth/login (public, Firebase token exchange)
// Called from Flutter app after Firebase authentication
func (h *DeviceAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FirebaseToken string `json:"firebase_token"`
		DeviceInfo    *struct {
			DeviceName string `json:"device_name"`
			PublicKey  string `json:"public_key"`
		} `json:"device_info,omitempty"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FirebaseToken == "" {
		respondErrorJSON(w, http.StatusBadRequest, "firebase_token is required")
		return
	}

	// Check if Firebase is configured
	if h.firebaseService == nil {
		respondErrorJSON(w, http.StatusServiceUnavailable, "Firebase authentication is not configured")
		return
	}

	// Verify Firebase ID token
	token, err := h.firebaseService.VerifyIDToken(r.Context(), req.FirebaseToken)
	if err != nil {
		respondErrorJSON(w, http.StatusUnauthorized, "invalid Firebase token")
		return
	}

	// Get email from Firebase token
	email := ""
	if emailClaim, ok := token.Claims["email"].(string); ok {
		email = emailClaim
	}
	if email == "" {
		respondErrorJSON(w, http.StatusBadRequest, "email not found in Firebase token")
		return
	}

	// Get or create user by Firebase UID
	user, err := h.authService.GetOrCreateUserByFirebaseUID(r.Context(), token.UID, email)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("failed to get or create user: %v", err))
		return
	}

	// Generate JWT (30 days)
	jwt, expiresAt, err := h.authService.GenerateToken(user.ID, user.Email)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to generate JWT")
		return
	}

	response := map[string]interface{}{
		"jwt":        jwt,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z"),
		"user": map[string]interface{}{
			"id":     user.ID,
			"email":  user.Email,
			"subnet": user.Subnet,
		},
	}

	// If device_info provided, auto-register device
	if req.DeviceInfo != nil && req.DeviceInfo.DeviceName != "" && req.DeviceInfo.PublicKey != "" {
		result, err := h.deviceService.RegisterDevice(
			r.Context(),
			user.ID,
			req.DeviceInfo.DeviceName,
			req.DeviceInfo.PublicKey,
			nil, // No username available from this flow
			nil, // osType - will be parsed from device_name
			nil, // hardwareID - will be parsed from device_name
			nil, // displayName
			nil, // deviceID - let server generate for biometric auth
		)
		if err != nil {
			// Don't fail the login, just return warning
			response["device_registration_error"] = err.Error()
		} else {
			// Add device to WireGuard (handles replacement and rollback)
			if err := services.AddDeviceToWireGuard(r.Context(), h.wgManager, h.deviceRepo, result, true); err != nil {
				response["device_registration_error"] = err.Error()
			} else {
				// Success! Add device info to response
				response["device"] = map[string]interface{}{
					"id":          result.Device.ID,
					"device_name": result.Device.DeviceName,
					"vpn_ip":      result.Device.VpnIP,
					"active":      result.Device.Active,
				}
				response["auto_registered"] = true
			}
		}
	}

	respondJSON(w, http.StatusOK, response)
}

func resolveRefreshTokenDeviceID(challenge *models.DeviceAuthChallenge) uuid.UUID {
	if challenge == nil {
		return uuid.Nil
	}
	if challenge.WgDeviceID != nil && *challenge.WgDeviceID != uuid.Nil {
		return *challenge.WgDeviceID
	}
	return challenge.DeviceID
}

// Helper function to extract client IP address
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr, stripping port
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If no port in RemoteAddr, use as-is
		return r.RemoteAddr
	}

	return host
}
