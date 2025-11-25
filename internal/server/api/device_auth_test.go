package api

import (
	"testing"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

func TestResolveRefreshTokenDeviceID_PrefersPersistedDeviceID(t *testing.T) {
	challengeDeviceID := uuid.New()
	wgDeviceID := uuid.New()

	challenge := &models.DeviceAuthChallenge{
		DeviceID:   challengeDeviceID,
		WgDeviceID: &wgDeviceID,
	}

	got := resolveRefreshTokenDeviceID(challenge)
	if got != wgDeviceID {
		t.Fatalf("expected %s, got %s", wgDeviceID, got)
	}
}

func TestResolveRefreshTokenDeviceID_FallsBackToChallengeID(t *testing.T) {
	challengeDeviceID := uuid.New()

	challenge := &models.DeviceAuthChallenge{
		DeviceID: challengeDeviceID,
	}

	got := resolveRefreshTokenDeviceID(challenge)
	if got != challengeDeviceID {
		t.Fatalf("expected %s, got %s", challengeDeviceID, got)
	}
}

func TestResolveRefreshTokenDeviceID_NilChallenge(t *testing.T) {
	if got := resolveRefreshTokenDeviceID(nil); got != uuid.Nil {
		t.Fatalf("expected uuid.Nil, got %s", got)
	}
}
