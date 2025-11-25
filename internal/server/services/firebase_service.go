package services

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type FirebaseService struct {
	authClient *auth.Client
}

// NewFirebaseService initializes Firebase Admin SDK
func NewFirebaseService(ctx context.Context) (*FirebaseService, error) {
	// Check if Firebase is configured
	credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credentialsPath == "" {
		return nil, fmt.Errorf("FIREBASE_CREDENTIALS_PATH not set")
	}

	// Initialize Firebase app with service account credentials
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	// Get Auth client
	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firebase Auth client: %w", err)
	}

	return &FirebaseService{
		authClient: authClient,
	}, nil
}

// VerifyIDToken verifies a Firebase ID token and returns the token claims
func (s *FirebaseService) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := s.authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify Firebase ID token: %w", err)
	}
	return token, nil
}

// GetFirebaseUser gets Firebase user info by UID
func (s *FirebaseService) GetFirebaseUser(ctx context.Context, uid string) (*auth.UserRecord, error) {
	user, err := s.authClient.GetUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firebase user: %w", err)
	}
	return user, nil
}
