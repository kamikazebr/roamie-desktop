package tunnel

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

// AuthorizationManager handles tunnel access control with caching and rate limiting
type AuthorizationManager struct {
	deviceRepo *storage.DeviceRepository

	// Cache for device ownership lookups (reduces DB load)
	cache      map[int]*cacheEntry
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
	maxCacheEntries int

	// Rate limiting per source device
	rateLimiter map[string]*rateLimitEntry
	rateMu      sync.RWMutex
	maxAttemptsPerMinute int
}

type cacheEntry struct {
	device    *models.Device
	expiresAt time.Time
}

type rateLimitEntry struct {
	attempts  int
	windowStart time.Time
	blockedUntil time.Time
}

// NewAuthorizationManager creates a new authorization manager
func NewAuthorizationManager(deviceRepo *storage.DeviceRepository) *AuthorizationManager {
	am := &AuthorizationManager{
		deviceRepo:          deviceRepo,
		cache:               make(map[int]*cacheEntry),
		cacheTTL:            30 * time.Second, // Cache for 30 seconds
		maxCacheEntries:     1000,
		rateLimiter:         make(map[string]*rateLimitEntry),
		maxAttemptsPerMinute: 10, // Max 10 failed attempts per minute
	}

	// Start cache cleanup goroutine
	go am.cleanupLoop()

	return am
}

// AuthorizeConnection checks if a source device can access a target tunnel port
func (am *AuthorizationManager) AuthorizeConnection(ctx context.Context, sourceDeviceID uuid.UUID, targetPort int) (*models.Device, error) {
	sourceDeviceIDStr := sourceDeviceID.String()

	// Check rate limiting first
	if am.isRateLimited(sourceDeviceIDStr) {
		log.Printf("⚠️  SECURITY: Rate limited device %s trying to access port %d", sourceDeviceID, targetPort)
		return nil, fmt.Errorf("rate limited: too many failed attempts")
	}

	// Try cache first
	targetDevice := am.getFromCache(targetPort)
	if targetDevice == nil {
		// Cache miss - query database
		var err error
		targetDevice, err = am.deviceRepo.GetByTunnelPort(ctx, targetPort)
		if err != nil {
			log.Printf("Error querying device for port %d: %v", targetPort, err)
			return nil, fmt.Errorf("database error during authorization")
		}

		if targetDevice == nil {
			am.recordFailedAttempt(sourceDeviceIDStr)
			log.Printf("⚠️  SECURITY: Device %s tried to access non-existent port %d", sourceDeviceID, targetPort)
			return nil, fmt.Errorf("tunnel port not found")
		}

		// Cache the result
		am.addToCache(targetPort, targetDevice)
	}

	// Get source device to compare user IDs
	sourceDevice, err := am.deviceRepo.GetByID(ctx, sourceDeviceID)
	if err != nil {
		log.Printf("Error querying source device %s: %v", sourceDeviceID, err)
		return nil, fmt.Errorf("database error during authorization")
	}

	if sourceDevice == nil {
		am.recordFailedAttempt(sourceDeviceIDStr)
		log.Printf("⚠️  SECURITY: Unknown device %s tried to access port %d", sourceDeviceID, targetPort)
		return nil, fmt.Errorf("source device not found")
	}

	// Check if source device is active
	if !sourceDevice.Active {
		am.recordFailedAttempt(sourceDeviceIDStr)
		log.Printf("⚠️  SECURITY: Inactive device %s tried to access port %d", sourceDeviceID, targetPort)
		return nil, fmt.Errorf("source device is inactive")
	}

	// Check if target device is active
	if !targetDevice.Active {
		am.recordFailedAttempt(sourceDeviceIDStr)
		log.Printf("⚠️  SECURITY: Device %s tried to access inactive device port %d", sourceDeviceID, targetPort)
		return nil, fmt.Errorf("target device is inactive")
	}

	// Check if target tunnel is enabled
	if !targetDevice.TunnelEnabled {
		am.recordFailedAttempt(sourceDeviceIDStr)
		log.Printf("⚠️  SECURITY: Device %s tried to access disabled tunnel port %d", sourceDeviceID, targetPort)
		return nil, fmt.Errorf("target tunnel is disabled")
	}

	// CRITICAL CHECK: Verify both devices belong to the same user
	if sourceDevice.UserID != targetDevice.UserID {
		am.recordFailedAttempt(sourceDeviceIDStr)
		log.Printf("🚨 SECURITY VIOLATION: Device %s (user %s) tried to access device %s (user %s) on port %d",
			sourceDeviceID, sourceDevice.UserID, targetDevice.ID, targetDevice.UserID, targetPort)
		return nil, fmt.Errorf("access denied: cross-account access not allowed")
	}

	// Authorization successful - reset rate limit counter
	am.resetRateLimit(sourceDeviceIDStr)

	log.Printf("✓ Authorized: Device %s → Port %d (same user: %s)", sourceDeviceID, targetPort, sourceDevice.UserID)
	return targetDevice, nil
}

// getFromCache retrieves a device from cache if not expired
func (am *AuthorizationManager) getFromCache(port int) *models.Device {
	am.cacheMu.RLock()
	defer am.cacheMu.RUnlock()

	entry, exists := am.cache[port]
	if !exists {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		// Expired - will be cleaned up by cleanup loop
		return nil
	}

	return entry.device
}

// addToCache adds a device to cache with TTL
func (am *AuthorizationManager) addToCache(port int, device *models.Device) {
	am.cacheMu.Lock()
	defer am.cacheMu.Unlock()

	// Prevent cache from growing too large
	if len(am.cache) >= am.maxCacheEntries {
		// Simple eviction: clear entire cache
		am.cache = make(map[int]*cacheEntry)
		log.Printf("Cache full, cleared %d entries", am.maxCacheEntries)
	}

	am.cache[port] = &cacheEntry{
		device:    device,
		expiresAt: time.Now().Add(am.cacheTTL),
	}
}

// InvalidateCache removes a specific port from cache (e.g., when device is disabled)
func (am *AuthorizationManager) InvalidateCache(port int) {
	am.cacheMu.Lock()
	defer am.cacheMu.Unlock()
	delete(am.cache, port)
}

// isRateLimited checks if a device is currently rate limited
func (am *AuthorizationManager) isRateLimited(deviceID string) bool {
	am.rateMu.RLock()
	defer am.rateMu.RUnlock()

	entry, exists := am.rateLimiter[deviceID]
	if !exists {
		return false
	}

	// Check if device is in blocking period
	if time.Now().Before(entry.blockedUntil) {
		return true
	}

	return false
}

// recordFailedAttempt records a failed authorization attempt
func (am *AuthorizationManager) recordFailedAttempt(deviceID string) {
	am.rateMu.Lock()
	defer am.rateMu.Unlock()

	now := time.Now()
	entry, exists := am.rateLimiter[deviceID]

	if !exists {
		// First failed attempt
		am.rateLimiter[deviceID] = &rateLimitEntry{
			attempts:    1,
			windowStart: now,
			blockedUntil: time.Time{},
		}
		return
	}

	// Check if we're in a new time window
	if now.Sub(entry.windowStart) > time.Minute {
		// Reset counter for new window
		entry.attempts = 1
		entry.windowStart = now
		entry.blockedUntil = time.Time{}
		return
	}

	// Increment counter in current window
	entry.attempts++

	// Block if exceeded threshold
	if entry.attempts >= am.maxAttemptsPerMinute {
		// Block for progressively longer periods
		blockDuration := time.Duration(entry.attempts-am.maxAttemptsPerMinute+1) * time.Minute
		if blockDuration > 10*time.Minute {
			blockDuration = 10 * time.Minute // Max 10 minutes
		}
		entry.blockedUntil = now.Add(blockDuration)
		log.Printf("🚨 SECURITY: Device %s blocked for %v after %d failed attempts",
			deviceID, blockDuration, entry.attempts)
	}
}

// resetRateLimit resets the rate limit counter for a device after successful auth
func (am *AuthorizationManager) resetRateLimit(deviceID string) {
	am.rateMu.Lock()
	defer am.rateMu.Unlock()
	delete(am.rateLimiter, deviceID)
}

// cleanupLoop periodically cleans up expired cache entries and rate limit entries
func (am *AuthorizationManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		am.cleanupCache()
		am.cleanupRateLimiter()
	}
}

func (am *AuthorizationManager) cleanupCache() {
	am.cacheMu.Lock()
	defer am.cacheMu.Unlock()

	now := time.Now()
	for port, entry := range am.cache {
		if now.After(entry.expiresAt) {
			delete(am.cache, port)
		}
	}
}

func (am *AuthorizationManager) cleanupRateLimiter() {
	am.rateMu.Lock()
	defer am.rateMu.Unlock()

	now := time.Now()
	for deviceID, entry := range am.rateLimiter {
		// Remove entries that are no longer blocked and past their window
		if now.After(entry.blockedUntil) && now.Sub(entry.windowStart) > 5*time.Minute {
			delete(am.rateLimiter, deviceID)
		}
	}
}
