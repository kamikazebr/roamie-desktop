package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Email       string    `json:"email" db:"email"`
	Subnet      string    `json:"subnet" db:"subnet"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	MaxDevices  int       `json:"max_devices" db:"max_devices"`
	Active      bool      `json:"active" db:"active"`
	FirebaseUID *string   `json:"firebase_uid,omitempty" db:"firebase_uid"`
}

type AuthCode struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Code      string    `json:"code" db:"code"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	Used      bool      `json:"used" db:"used"`
}
