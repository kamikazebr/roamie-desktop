package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type DiagnosticsService struct {
	firestoreClient *firestore.Client
}

// DiagnosticsRequest represents a pending diagnostics request
type DiagnosticsRequest struct {
	RequestID   string    `firestore:"request_id" json:"request_id"`
	DeviceID    string    `firestore:"device_id" json:"device_id"`
	UserID      string    `firestore:"user_id" json:"user_id"`
	RequestedAt time.Time `firestore:"requested_at" json:"requested_at"`
	RequestedBy string    `firestore:"requested_by" json:"requested_by"`
	Status      string    `firestore:"status" json:"status"` // pending, running, completed, failed
}

// CheckResult represents a single diagnostic check result
type CheckResult struct {
	Name     string   `firestore:"name" json:"name"`
	Category string   `firestore:"category" json:"category"`
	Status   string   `firestore:"status" json:"status"`
	Message  string   `firestore:"message" json:"message"`
	Fixes    []string `firestore:"fixes" json:"fixes"`
}

// DiagnosticsReport represents a completed diagnostics report
type DiagnosticsReport struct {
	RequestID     string        `firestore:"request_id" json:"request_id"`
	DeviceID      string        `firestore:"device_id" json:"device_id"`
	RanAt         time.Time     `firestore:"ran_at" json:"ran_at"`
	Checks        []CheckResult `firestore:"checks" json:"checks"`
	Summary       Summary       `firestore:"summary" json:"summary"`
	ClientVersion string        `firestore:"client_version" json:"client_version"`
	OS            string        `firestore:"os" json:"os"`
	Platform      string        `firestore:"platform" json:"platform"`
}

// Summary represents the summary of check results
type Summary struct {
	Passed   int `firestore:"passed" json:"passed"`
	Warnings int `firestore:"warnings" json:"warnings"`
	Errors   int `firestore:"errors" json:"errors"`
	Info     int `firestore:"info" json:"info"`
}

// NewDiagnosticsService initializes Diagnostics service with Firestore client
func NewDiagnosticsService(ctx context.Context) (*DiagnosticsService, error) {
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

	// Get Firestore client
	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firestore client: %w", err)
	}

	return &DiagnosticsService{
		firestoreClient: firestoreClient,
	}, nil
}

// CreateDiagnosticsRequest creates a new diagnostics request in Firestore
// Path: diagnostics_requests/{device_id}/pending/{request_id}
func (s *DiagnosticsService) CreateDiagnosticsRequest(ctx context.Context, req *DiagnosticsRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if req.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}

	// Set defaults
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now().UTC()
	}
	if req.Status == "" {
		req.Status = "pending"
	}

	// Write to Firestore
	_, err := s.firestoreClient.
		Collection("diagnostics_requests").
		Doc(req.DeviceID).
		Collection("pending").
		Doc(req.RequestID).
		Set(ctx, req)

	if err != nil {
		return fmt.Errorf("failed to create diagnostics request: %w", err)
	}

	return nil
}

// GetPendingRequests fetches all pending diagnostics requests for a device
// Path: diagnostics_requests/{device_id}/pending
func (s *DiagnosticsService) GetPendingRequests(ctx context.Context, deviceID string) ([]DiagnosticsRequest, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	iter := s.firestoreClient.
		Collection("diagnostics_requests").
		Doc(deviceID).
		Collection("pending").
		Documents(ctx)

	var requests []DiagnosticsRequest
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate pending requests: %w", err)
		}

		var req DiagnosticsRequest
		if err := doc.DataTo(&req); err != nil {
			return nil, fmt.Errorf("failed to parse diagnostics request: %w", err)
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// DeletePendingRequest deletes a pending request after completion
// Path: diagnostics_requests/{device_id}/pending/{request_id}
func (s *DiagnosticsService) DeletePendingRequest(ctx context.Context, deviceID, requestID string) error {
	if deviceID == "" || requestID == "" {
		return fmt.Errorf("device_id and request_id are required")
	}

	_, err := s.firestoreClient.
		Collection("diagnostics_requests").
		Doc(deviceID).
		Collection("pending").
		Doc(requestID).
		Delete(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete pending request: %w", err)
	}

	return nil
}

// SaveDiagnosticsReport saves a completed diagnostics report
// Path: diagnostics_reports/{device_id}/{request_id}
func (s *DiagnosticsService) SaveDiagnosticsReport(ctx context.Context, report *DiagnosticsReport) error {
	if report.DeviceID == "" || report.RequestID == "" {
		return fmt.Errorf("device_id and request_id are required")
	}

	// Set timestamp if not set
	if report.RanAt.IsZero() {
		report.RanAt = time.Now().UTC()
	}

	_, err := s.firestoreClient.
		Collection("diagnostics_reports").
		Doc(report.DeviceID).
		Collection("reports").
		Doc(report.RequestID).
		Set(ctx, report)

	if err != nil {
		return fmt.Errorf("failed to save diagnostics report: %w", err)
	}

	return nil
}

// GetDiagnosticsReport fetches a specific diagnostics report
// Path: diagnostics_reports/{device_id}/{request_id}
func (s *DiagnosticsService) GetDiagnosticsReport(ctx context.Context, deviceID, requestID string) (*DiagnosticsReport, error) {
	if deviceID == "" || requestID == "" {
		return nil, fmt.Errorf("device_id and request_id are required")
	}

	doc, err := s.firestoreClient.
		Collection("diagnostics_reports").
		Doc(deviceID).
		Collection("reports").
		Doc(requestID).
		Get(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get diagnostics report: %w", err)
	}

	var report DiagnosticsReport
	if err := doc.DataTo(&report); err != nil {
		return nil, fmt.Errorf("failed to parse diagnostics report: %w", err)
	}

	return &report, nil
}

// GetAllDiagnosticsReports fetches all diagnostics reports for a device (last N)
// Path: diagnostics_reports/{device_id}/reports
func (s *DiagnosticsService) GetAllDiagnosticsReports(ctx context.Context, deviceID string, limit int) ([]DiagnosticsReport, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	if limit <= 0 {
		limit = 10 // Default to last 10
	}

	query := s.firestoreClient.
		Collection("diagnostics_reports").
		Doc(deviceID).
		Collection("reports").
		OrderBy("ran_at", firestore.Desc).
		Limit(limit)

	iter := query.Documents(ctx)

	var reports []DiagnosticsReport
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate diagnostics reports: %w", err)
		}

		var report DiagnosticsReport
		if err := doc.DataTo(&report); err != nil {
			return nil, fmt.Errorf("failed to parse diagnostics report: %w", err)
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// CleanupOldRequests deletes pending requests older than the specified duration
func (s *DiagnosticsService) CleanupOldRequests(ctx context.Context, deviceID string, olderThan time.Duration) error {
	if deviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	cutoffTime := time.Now().UTC().Add(-olderThan)

	iter := s.firestoreClient.
		Collection("diagnostics_requests").
		Doc(deviceID).
		Collection("pending").
		Where("requested_at", "<", cutoffTime).
		Documents(ctx)

	batch := s.firestoreClient.Batch()
	deleteCount := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to iterate old requests: %w", err)
		}

		batch.Delete(doc.Ref)
		deleteCount++
	}

	if deleteCount > 0 {
		_, err := batch.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit batch delete: %w", err)
		}
	}

	return nil
}

// UpgradeRequest represents a pending upgrade request
type UpgradeRequest struct {
	RequestID   string    `firestore:"request_id" json:"request_id"`
	DeviceID    string    `firestore:"device_id" json:"device_id"`
	UserID      string    `firestore:"user_id" json:"user_id"`
	RequestedAt time.Time `firestore:"requested_at" json:"requested_at"`
	RequestedBy string    `firestore:"requested_by" json:"requested_by"`
	Status      string    `firestore:"status" json:"status"` // pending, running, completed, failed
	TargetVersion string  `firestore:"target_version,omitempty" json:"target_version,omitempty"` // Optional: specific version to upgrade to
}

// UpgradeResult represents a completed upgrade result
type UpgradeResult struct {
	RequestID      string    `firestore:"request_id" json:"request_id"`
	DeviceID       string    `firestore:"device_id" json:"device_id"`
	RanAt          time.Time `firestore:"ran_at" json:"ran_at"`
	Success        bool      `firestore:"success" json:"success"`
	PreviousVersion string   `firestore:"previous_version" json:"previous_version"`
	NewVersion     string    `firestore:"new_version" json:"new_version"`
	ErrorMessage   string    `firestore:"error_message,omitempty" json:"error_message,omitempty"`
}

// CreateUpgradeRequest creates a new upgrade request in Firestore
// Path: upgrade_requests/{device_id}/pending/{request_id}
func (s *DiagnosticsService) CreateUpgradeRequest(ctx context.Context, req *UpgradeRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if req.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}

	// Set defaults
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now().UTC()
	}
	if req.Status == "" {
		req.Status = "pending"
	}

	// Write to Firestore
	_, err := s.firestoreClient.
		Collection("upgrade_requests").
		Doc(req.DeviceID).
		Collection("pending").
		Doc(req.RequestID).
		Set(ctx, req)

	if err != nil {
		return fmt.Errorf("failed to create upgrade request: %w", err)
	}

	return nil
}

// GetPendingUpgrades fetches all pending upgrade requests for a device
// Path: upgrade_requests/{device_id}/pending
func (s *DiagnosticsService) GetPendingUpgrades(ctx context.Context, deviceID string) ([]UpgradeRequest, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	iter := s.firestoreClient.
		Collection("upgrade_requests").
		Doc(deviceID).
		Collection("pending").
		Documents(ctx)

	var requests []UpgradeRequest
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate pending upgrades: %w", err)
		}

		var req UpgradeRequest
		if err := doc.DataTo(&req); err != nil {
			return nil, fmt.Errorf("failed to parse upgrade request: %w", err)
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// DeletePendingUpgrade deletes a pending upgrade request after completion
// Path: upgrade_requests/{device_id}/pending/{request_id}
func (s *DiagnosticsService) DeletePendingUpgrade(ctx context.Context, deviceID, requestID string) error {
	if deviceID == "" || requestID == "" {
		return fmt.Errorf("device_id and request_id are required")
	}

	_, err := s.firestoreClient.
		Collection("upgrade_requests").
		Doc(deviceID).
		Collection("pending").
		Doc(requestID).
		Delete(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete pending upgrade: %w", err)
	}

	return nil
}

// SaveUpgradeResult saves a completed upgrade result
// Path: upgrade_results/{device_id}/{request_id}
func (s *DiagnosticsService) SaveUpgradeResult(ctx context.Context, result *UpgradeResult) error {
	if result.DeviceID == "" || result.RequestID == "" {
		return fmt.Errorf("device_id and request_id are required")
	}

	// Set timestamp if not set
	if result.RanAt.IsZero() {
		result.RanAt = time.Now().UTC()
	}

	_, err := s.firestoreClient.
		Collection("upgrade_results").
		Doc(result.DeviceID).
		Collection("results").
		Doc(result.RequestID).
		Set(ctx, result)

	if err != nil {
		return fmt.Errorf("failed to save upgrade result: %w", err)
	}

	return nil
}

// GetUpgradeResult fetches a specific upgrade result
// Path: upgrade_results/{device_id}/{request_id}
func (s *DiagnosticsService) GetUpgradeResult(ctx context.Context, deviceID, requestID string) (*UpgradeResult, error) {
	if deviceID == "" || requestID == "" {
		return nil, fmt.Errorf("device_id and request_id are required")
	}

	doc, err := s.firestoreClient.
		Collection("upgrade_results").
		Doc(deviceID).
		Collection("results").
		Doc(requestID).
		Get(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get upgrade result: %w", err)
	}

	var result UpgradeResult
	if err := doc.DataTo(&result); err != nil {
		return nil, fmt.Errorf("failed to parse upgrade result: %w", err)
	}

	return &result, nil
}

// Close closes the Firestore client
func (s *DiagnosticsService) Close() error {
	if s.firestoreClient != nil {
		return s.firestoreClient.Close()
	}
	return nil
}
