package version

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := GitCommit
	origTime := BuildTime
	origDirty := GitDirty

	// Restore after test
	defer func() {
		Version = origVersion
		GitCommit = origCommit
		BuildTime = origTime
		GitDirty = origDirty
	}()

	tests := []struct {
		name      string
		version   string
		commit    string
		buildTime string
		dirty     string
		appName   string
		want      string
	}{
		{
			name:      "clean build",
			version:   "v1.0.0",
			commit:    "abc1234",
			buildTime: "2025-01-01T12:00:00Z",
			dirty:     "false",
			appName:   "roamie",
			want:      "roamie v1.0.0 (abc1234 2025-01-01T12:00:00Z)",
		},
		{
			name:      "dirty build",
			version:   "v1.0.0",
			commit:    "abc1234",
			buildTime: "2025-01-01T12:00:00Z",
			dirty:     "true",
			appName:   "roamie",
			want:      "roamie v1.0.0 (abc1234-dirty 2025-01-01T12:00:00Z)",
		},
		{
			name:      "dev version",
			version:   "dev",
			commit:    "unknown",
			buildTime: "unknown",
			dirty:     "",
			appName:   "roamie",
			want:      "roamie dev (unknown unknown)",
		},
		{
			name:      "server binary",
			version:   "v2.0.0",
			commit:    "def5678",
			buildTime: "2025-06-15T08:30:00Z",
			dirty:     "false",
			appName:   "roamie-server",
			want:      "roamie-server v2.0.0 (def5678 2025-06-15T08:30:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version = tt.version
			GitCommit = tt.commit
			BuildTime = tt.buildTime
			GitDirty = tt.dirty

			got := GetVersion(tt.appName)
			if got != tt.want {
				t.Errorf("GetVersion(%q) = %q, want %q", tt.appName, got, tt.want)
			}
		})
	}
}

func TestGetVersionInfo(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := GitCommit
	origTime := BuildTime
	origDirty := GitDirty

	// Restore after test
	defer func() {
		Version = origVersion
		GitCommit = origCommit
		BuildTime = origTime
		GitDirty = origDirty
	}()

	Version = "v1.2.3"
	GitCommit = "abc1234"
	BuildTime = "2025-01-15T10:00:00Z"
	GitDirty = "false"

	info := GetVersionInfo()

	// Check that all expected fields are present
	expectedFields := []string{
		"Version:    v1.2.3",
		"Git commit: abc1234 (clean)",
		"Built:      2025-01-15T10:00:00Z",
		"Go version:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(info, field) {
			t.Errorf("GetVersionInfo() missing field %q\nGot:\n%s", field, info)
		}
	}

	// Test dirty state
	GitDirty = "true"
	info = GetVersionInfo()
	if !strings.Contains(info, "(dirty)") {
		t.Errorf("GetVersionInfo() should show (dirty) when GitDirty=true\nGot:\n%s", info)
	}
}
