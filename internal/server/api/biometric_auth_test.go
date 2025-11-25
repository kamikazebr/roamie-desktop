package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kamikazebr/roamie-desktop/internal/server/services"
	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/go-chi/chi/v5"
)

// TestBiometricRoutesRegistered verifica se as rotas biometric estão registradas corretamente
func TestBiometricRoutesRegistered(t *testing.T) {
	// Setup minimal router with biometric routes
	r := chi.NewRouter()

	// Create a mock handler (we're just testing routing, not logic)
	mockHandler := &BiometricAuthHandler{
		authService: nil, // nil is ok for route testing
	}

	r.Route("/api", func(r chi.Router) {
		r.Use(AuthMiddleware) // Routes are protected

		r.Route("/biometric", func(r chi.Router) {
			r.Post("/request", mockHandler.CreateRequest)
			r.Get("/pending", mockHandler.ListPending)
			r.Post("/respond", mockHandler.RespondToRequest)
			r.Get("/poll/{request_id}", mockHandler.PollStatus)
			r.Get("/stats", mockHandler.GetStats)
		})
	})

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "POST /api/biometric/request without auth",
			method:         "POST",
			path:           "/api/biometric/request",
			expectedStatus: http.StatusUnauthorized, // 401 because no auth
		},
		{
			name:           "GET /api/biometric/pending without auth",
			method:         "GET",
			path:           "/api/biometric/pending",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "POST /api/biometric/respond without auth",
			method:         "POST",
			path:           "/api/biometric/respond",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "GET /api/biometric/poll/123 without auth",
			method:         "GET",
			path:           "/api/biometric/poll/550e8400-e29b-41d4-a716-446655440000",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "GET /api/biometric/stats without auth",
			method:         "GET",
			path:           "/api/biometric/stats",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d for %s %s",
					tt.expectedStatus, w.Code, tt.method, tt.path)
				t.Logf("Response body: %s", w.Body.String())
			} else {
				t.Logf("✓ Route %s %s correctly returns %d", tt.method, tt.path, w.Code)
			}
		})
	}
}

// TestOldRoutesNotFound verifica que as rotas antigas retornam 404
func TestOldRoutesNotFound(t *testing.T) {
	r := chi.NewRouter()

	mockHandler := &BiometricAuthHandler{
		authService: nil,
	}

	r.Route("/api", func(r chi.Router) {
		// Only apply auth middleware to the biometric subroute, not the whole /api
		r.Route("/biometric", func(r chi.Router) {
			r.Use(AuthMiddleware)
			r.Get("/pending", mockHandler.ListPending)
		})
	})

	// Try old route path (should 404 - route doesn't exist)
	req := httptest.NewRequest("GET", "/api/auth/biometric/pending", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for old route /api/auth/biometric/pending, got %d", w.Code)
	} else {
		t.Logf("✓ Old route /api/auth/biometric/pending correctly returns 404")
	}

	// Try new route path (should 401 because no valid auth)
	req = httptest.NewRequest("GET", "/api/biometric/pending", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for new route /api/biometric/pending, got %d", w.Code)
	} else {
		t.Logf("✓ New route /api/biometric/pending correctly returns 401 (requires auth)")
	}
}

// TestBiometricHandlerInitialization verifica se o handler pode ser inicializado
func TestBiometricHandlerInitialization(t *testing.T) {
	// This test just verifies we can create the handler
	// In real tests, we'd use a mock or test database

	handler := NewBiometricAuthHandler(nil)

	if handler == nil {
		t.Error("Expected non-nil handler")
	}

	if handler.authService != nil {
		t.Error("Expected nil authService (we passed nil)")
	}

	t.Log("✓ BiometricAuthHandler can be initialized")
}

// Mock implementations for testing
type mockBiometricAuthService struct {
	services.BiometricAuthService
}

type mockDB struct {
	storage.DB
}
