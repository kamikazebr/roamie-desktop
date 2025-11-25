package api

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

type contextKey string

const (
	userClaimsKey contextKey = "userClaims"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		token := parts[1]
		jwtSecret := os.Getenv("JWT_SECRET")

		claims, err := utils.ValidateJWT(token, jwtSecret)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), userClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserClaims(r *http.Request) *utils.Claims {
	claims, ok := r.Context().Value(userClaimsKey).(*utils.Claims)
	if !ok {
		return nil
	}
	return claims
}

var (
	adminEmailsOnce sync.Once
	adminEmailSet   map[string]struct{}
)

func loadAdminEmails() {
	adminEmailSet = make(map[string]struct{})
	addEmail := func(email string) {
		email = strings.TrimSpace(strings.ToLower(email))
		if email != "" {
			adminEmailSet[email] = struct{}{}
		}
	}

	// Load admin emails from environment variable (required for admin access)
	// Set ADMIN_EMAILS=email1@example.com,email2@example.com
	if raw := os.Getenv("ADMIN_EMAILS"); raw != "" {
		for _, email := range strings.Split(raw, ",") {
			addEmail(email)
		}
	}
}

func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminEmailsOnce.Do(loadAdminEmails)

		claims := GetUserClaims(r)
		if claims == nil {
			respondError(w, http.StatusUnauthorized, "missing authorization claims")
			return
		}

		if _, ok := adminEmailSet[strings.ToLower(claims.Email)]; !ok {
			respondError(w, http.StatusForbidden, "admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := models.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	}
	writeJSON(w, response)
}
