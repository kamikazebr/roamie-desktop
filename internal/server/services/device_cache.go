package services

import (
	"log"
	"sync"
	"time"
)

// DeviceCache provides in-memory caching of device online status
// Used to avoid hitting the database on every GET /api/devices request
type DeviceCache struct {
	data sync.Map // map[string]time.Time - deviceID -> expiry timestamp
}

// NewDeviceCache creates a new device cache and starts the cleanup goroutine
func NewDeviceCache() *DeviceCache {
	cache := &DeviceCache{}
	go cache.startCleanup()
	log.Println("Device cache initialized")
	return cache
}

// MarkOnline marks a device as online in the cache
// The device will be considered online for 90 seconds (heartbeat interval is 30s)
func (c *DeviceCache) MarkOnline(deviceID string) {
	expiryTime := time.Now().Add(90 * time.Second)
	c.data.Store(deviceID, expiryTime)
}

// IsOnline checks if a device is currently online
// Returns true if the device's expiry time has not passed
func (c *DeviceCache) IsOnline(deviceID string) bool {
	val, ok := c.data.Load(deviceID)
	if !ok {
		return false
	}

	expiryTime, ok := val.(time.Time)
	if !ok {
		return false
	}

	return time.Now().Before(expiryTime)
}

// GetOnlineDevices returns a list of all currently online device IDs
func (c *DeviceCache) GetOnlineDevices() []string {
	var online []string
	now := time.Now()

	c.data.Range(func(key, val interface{}) bool {
		deviceID, ok1 := key.(string)
		expiryTime, ok2 := val.(time.Time)

		if ok1 && ok2 && now.Before(expiryTime) {
			online = append(online, deviceID)
		}
		return true
	})

	return online
}

// startCleanup runs a background goroutine that removes expired entries
// Runs every 30 seconds
func (c *DeviceCache) startCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		keysToDelete := []string{}

		// Collect expired keys
		c.data.Range(func(key, val interface{}) bool {
			deviceID, ok1 := key.(string)
			expiryTime, ok2 := val.(time.Time)

			if ok1 && ok2 && now.After(expiryTime) {
				keysToDelete = append(keysToDelete, deviceID)
			}
			return true
		})

		// Delete expired keys
		for _, deviceID := range keysToDelete {
			c.data.Delete(deviceID)
		}

		if len(keysToDelete) > 0 {
			log.Printf("Device cache: cleaned up %d expired entries", len(keysToDelete))
		}
	}
}
