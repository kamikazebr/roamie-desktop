package api

import (
	"net"
	"net/http"
	"strconv"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type BiometricAuthHandler struct {
	authService *services.BiometricAuthService
}

func NewBiometricAuthHandler(authService *services.BiometricAuthService) *BiometricAuthHandler {
	return &BiometricAuthHandler{
		authService: authService,
	}
}

// CreateRequest handles POST /api/auth/biometric/request
// Called from Linux system to create a new auth request
func (h *BiometricAuthHandler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateBiometricAuthRequest
	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Username == "" || req.Hostname == "" || req.Command == "" {
		respondErrorJSON(w, http.StatusBadRequest, "username, hostname, and command are required")
		return
	}

	// Get client IP if not provided
	ipAddress := req.IPAddress
	if ipAddress == "" {
		// Strip port from RemoteAddr (e.g., "10.100.0.4:58852" -> "10.100.0.4")
		// PostgreSQL INET type requires IP address only, not IP:PORT
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// If no port in RemoteAddr, use as-is
			ipAddress = r.RemoteAddr
		} else {
			ipAddress = host
		}
	}

	// Create auth request
	authReq, err := h.authService.CreateRequest(
		r.Context(),
		claims.UserID,
		req.Username,
		req.Hostname,
		req.Command,
		req.DeviceID,
		ipAddress,
	)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := models.CreateBiometricAuthResponse{
		RequestID: authReq.ID.String(),
		ExpiresAt: authReq.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		ExpiresIn: 30, // seconds
	}

	respondJSON(w, http.StatusCreated, response)
}

// ListPending handles GET /api/auth/biometric/pending
// Called from Flutter app to list pending auth requests
func (h *BiometricAuthHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requests, err := h.authService.ListPendingRequests(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := models.ListPendingAuthRequestsResponse{
		Requests: requests,
		Count:    len(requests),
	}

	respondJSON(w, http.StatusOK, response)
}

// RespondToRequest handles POST /api/auth/biometric/respond
// Called from Flutter app to approve/deny an auth request
func (h *BiometricAuthHandler) RespondToRequest(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.RespondToAuthRequest
	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.RequestID == "" || req.Response == "" {
		respondErrorJSON(w, http.StatusBadRequest, "request_id and response are required")
		return
	}

	// Parse request ID
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request_id format")
		return
	}

	// Validate response
	if req.Response != "approved" && req.Response != "denied" {
		respondErrorJSON(w, http.StatusBadRequest, "response must be 'approved' or 'denied'")
		return
	}

	// Process response
	authReq, err := h.authService.RespondToRequest(
		r.Context(),
		claims.UserID,
		requestID,
		req.Response,
	)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	response := models.RespondToAuthResponse{
		Message:   "Response recorded successfully",
		Status:    authReq.Status,
		RequestID: authReq.ID.String(),
	}

	respondJSON(w, http.StatusOK, response)
}

// PollStatus handles GET /api/auth/biometric/poll/{request_id}
// Called from Linux system to poll for auth status
func (h *BiometricAuthHandler) PollStatus(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requestIDStr := chi.URLParam(r, "request_id")

	// Parse request ID
	requestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request_id format")
		return
	}

	// Get request status
	authReq, err := h.authService.PollRequestStatus(r.Context(), requestID)
	if err != nil {
		respondErrorJSON(w, http.StatusNotFound, err.Error())
		return
	}

	// Verify request belongs to user
	if authReq.UserID != claims.UserID {
		respondErrorJSON(w, http.StatusForbidden, "unauthorized access to request")
		return
	}

	// Build response
	response := models.PollAuthStatusResponse{
		Status: authReq.Status,
	}

	if authReq.Response != nil {
		response.Response = *authReq.Response
	}

	if authReq.RespondedAt != nil {
		response.RespondedAt = authReq.RespondedAt.Format("2006-01-02T15:04:05Z")
	}

	// Add message based on status
	switch authReq.Status {
	case "pending":
		response.Message = "Waiting for user response"
	case "approved":
		response.Message = "Authentication approved"
	case "denied":
		response.Message = "Authentication denied"
	case "expired":
		response.Message = "Request expired"
	case "timeout":
		response.Message = "Request timed out"
	}

	respondJSON(w, http.StatusOK, response)
}

// GetStats handles GET /api/auth/biometric/stats
// Returns statistics about biometric auth requests
func (h *BiometricAuthHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get days parameter (default 30)
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil && parsed > 0 {
			days = parsed
		}
	}

	stats, err := h.authService.GetStats(r.Context(), claims.UserID, days)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
