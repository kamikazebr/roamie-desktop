package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

func resetAdminEmailsForTest() {
	adminEmailsOnce = sync.Once{}
	adminEmailSet = nil
}

func TestAdminMiddleware_AllowsConfiguredEmail(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	resetAdminEmailsForTest()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/network/scan", nil)
	claims := &utils.Claims{Email: "admin@example.com"}
	ctx := context.WithValue(req.Context(), userClaimsKey, claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	nextCalled := false
	handler := AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatalf("expected next handler to run for configured admin")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 status, got %d", rec.Code)
	}
}

func TestAdminMiddleware_RejectsNonAdmin(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	resetAdminEmailsForTest()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/network/scan", nil)
	claims := &utils.Claims{Email: "user@example.com"}
	ctx := context.WithValue(req.Context(), userClaimsKey, claims)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler := AdminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected call to next handler")
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 status for non-admin, got %d", rec.Code)
	}
}
