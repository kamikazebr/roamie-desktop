package api

import (
	"net/http"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) RequestCode(w http.ResponseWriter, r *http.Request) {
	var req models.RequestCodeRequest

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		respondErrorJSON(w, http.StatusBadRequest, "email is required")
		return
	}

	expiresIn, err := h.authService.RequestCode(r.Context(), req.Email)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := models.RequestCodeResponse{
		Message:   "Code sent to email",
		ExpiresIn: expiresIn,
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var req models.VerifyCodeRequest

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Code == "" {
		respondErrorJSON(w, http.StatusBadRequest, "email and code are required")
		return
	}

	token, expiresAt, err := h.authService.VerifyCode(r.Context(), req.Email, req.Code)
	if err != nil {
		respondErrorJSON(w, http.StatusUnauthorized, err.Error())
		return
	}

	response := models.VerifyCodeResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format("2006-01-02T15:04:05Z"),
	}

	respondJSON(w, http.StatusOK, response)
}
