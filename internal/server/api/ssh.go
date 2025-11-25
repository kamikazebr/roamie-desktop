package api

import (
	"net/http"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
)

type SSHHandler struct {
	sshService *services.SSHService
	userRepo   *storage.UserRepository
}

func NewSSHHandler(
	sshService *services.SSHService,
	userRepo *storage.UserRepository,
) *SSHHandler {
	return &SSHHandler{
		sshService: sshService,
		userRepo:   userRepo,
	}
}

// GetSSHKeys returns all SSH public keys for the authenticated user
// GET /api/ssh/keys
func (h *SSHHandler) GetSSHKeys(w http.ResponseWriter, r *http.Request) {
	// Extract user from JWT
	claims := GetUserClaims(r)
	if claims == nil {
		respondErrorJSON(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get user from database to fetch email
	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		respondErrorJSON(w, http.StatusNotFound, "user not found")
		return
	}

	// Fetch SSH keys from Firestore using user's email
	keys, err := h.sshService.GetUserSSHKeys(r.Context(), user.Email)
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to fetch SSH keys: "+err.Error())
		return
	}

	// Return keys (empty array if no keys)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}
