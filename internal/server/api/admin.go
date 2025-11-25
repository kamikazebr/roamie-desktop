package api

import (
	"net/http"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
)

type AdminHandler struct {
	networkScanner *services.NetworkScanner
}

func NewAdminHandler(networkScanner *services.NetworkScanner) *AdminHandler {
	return &AdminHandler{
		networkScanner: networkScanner,
	}
}

func (h *AdminHandler) ScanNetworks(w http.ResponseWriter, r *http.Request) {
	conflicts, err := h.networkScanner.ScanNetworks(r.Context())
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to scan networks")
		return
	}

	// Define available ranges (could be made dynamic based on conflicts)
	availableRanges := []string{
		"10.100.0.0/16",
		"10.200.0.0/16",
		"10.150.0.0/16",
	}

	response := models.NetworkScanResponse{
		ScannedAt:       time.Now().Format("2006-01-02T15:04:05Z"),
		Conflicts:       conflicts,
		AvailableRanges: availableRanges,
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *AdminHandler) AddConflict(w http.ResponseWriter, r *http.Request) {
	var req models.AddConflictRequest

	if err := decodeJSON(r, &req); err != nil {
		respondErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CIDR == "" || req.Source == "" {
		respondErrorJSON(w, http.StatusBadRequest, "cidr and source are required")
		return
	}

	if err := h.networkScanner.AddManualConflict(r.Context(), req.CIDR, req.Source, req.Description); err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to add conflict")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Network conflict registered",
	})
}

func (h *AdminHandler) ListConflicts(w http.ResponseWriter, r *http.Request) {
	conflicts, err := h.networkScanner.GetAllConflicts(r.Context())
	if err != nil {
		respondErrorJSON(w, http.StatusInternalServerError, "failed to get conflicts")
		return
	}

	respondJSON(w, http.StatusOK, conflicts)
}
