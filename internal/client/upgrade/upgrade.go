package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kamikazebr/roamie-desktop/pkg/version"
)

const (
	githubRepo     = "kamikazebr/roamie-desktop"
	githubAPIURL   = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	githubReleases = "https://github.com/" + githubRepo + "/releases"
)

// Release represents a GitHub release
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckResult contains the result of a version check
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseNotes    string
	ReleaseURL      string
	DownloadURL     string
	ChecksumURL     string
	AssetName       string
}

// CheckForUpdates checks if a new version is available
func CheckForUpdates() (*CheckResult, error) {
	release, err := fetchLatestRelease()
	if err != nil {
		return nil, err
	}

	currentVersion := version.Version
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")

	// dev builds always show update available
	updateAvailable := currentVersion == "dev" || currentClean != latestVersion

	result := &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: updateAvailable,
		ReleaseNotes:    release.Body,
		ReleaseURL:      release.HTMLURL,
	}

	// Find matching asset for current platform
	assetName := getAssetName()
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			result.DownloadURL = asset.BrowserDownloadURL
			result.AssetName = asset.Name
		}
		if asset.Name == "checksums.txt" {
			result.ChecksumURL = asset.BrowserDownloadURL
		}
	}

	return result, nil
}

// Upgrade downloads and installs the latest version
func Upgrade(result *CheckResult) error {
	if result.DownloadURL == "" {
		return fmt.Errorf("no compatible binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Check write permissions
	if err := checkWritePermission(execPath); err != nil {
		return err
	}

	// Download checksum file first
	fmt.Println("Downloading checksums...")
	expectedChecksum, err := downloadChecksum(result.ChecksumURL, result.AssetName)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}

	// Download new binary to temp file
	fmt.Printf("Downloading %s...\n", result.AssetName)
	tempFile, err := downloadBinary(result.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.Remove(tempFile)

	// Verify checksum
	fmt.Println("Verifying checksum...")
	if err := verifyChecksum(tempFile, expectedChecksum); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	fmt.Println("Checksum verified!")

	// Backup current binary
	backupPath := execPath + ".backup"
	fmt.Printf("Backing up current binary to %s...\n", backupPath)
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Replace binary
	fmt.Println("Installing new version...")
	if err := replaceBinary(tempFile, execPath); err != nil {
		// Try to restore backup
		copyFile(backupPath, execPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// Clean up backup on success
	os.Remove(backupPath)

	return nil
}

// fetchLatestRelease fetches the latest release from GitHub API
func fetchLatestRelease() (*Release, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("GitHub rate limit exceeded. Please try again in a few minutes")
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found. Check %s", githubReleases)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

// getAssetName returns the expected asset name for current platform
func getAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	name := fmt.Sprintf("roamie-%s-%s", os, arch)
	if os == "windows" {
		name += ".exe"
	}
	return name
}

// downloadChecksum downloads checksums.txt and extracts the checksum for the given asset
func downloadChecksum(url, assetName string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download checksums: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse checksums.txt format: "checksum  filename"
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s", assetName)
}

// downloadBinary downloads the binary to a temp file
func downloadBinary(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download: status %d", resp.StatusCode)
	}

	tempFile, err := os.CreateTemp("", "roamie-upgrade-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// verifyChecksum verifies the SHA256 checksum of a file
func verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// checkWritePermission checks if we can write to the executable path
func checkWritePermission(path string) error {
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".roamie-upgrade-test")

	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("no write permission to %s. Try running with sudo", dir)
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Preserve permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

// replaceBinary replaces the current binary with the new one
func replaceBinary(newBinary, targetPath string) error {
	// Get original permissions
	info, err := os.Stat(targetPath)
	if err != nil {
		return err
	}
	mode := info.Mode()

	// Remove old binary
	if err := os.Remove(targetPath); err != nil {
		return err
	}

	// Copy new binary
	if err := copyFile(newBinary, targetPath); err != nil {
		return err
	}

	// Set executable permissions
	return os.Chmod(targetPath, mode)
}
