package utils

import "os"

// WriteFileWithOwnership writes a file and fixes ownership when running with sudo.
// This ensures files created by root (when running with sudo) are owned by the actual user.
// On Windows, this is a no-op for ownership (os.Chown does nothing).
func WriteFileWithOwnership(path string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	return FixFileOwnership(path)
}

// MkdirAllWithOwnership creates a directory (and parents) and fixes ownership when running with sudo.
// This ensures directories created by root (when running with sudo) are owned by the actual user.
// On Windows, this is a no-op for ownership (os.Chown does nothing).
func MkdirAllWithOwnership(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	return FixFileOwnership(path)
}

// CreateFileWithOwnership creates/truncates a file and fixes ownership when running with sudo.
// Returns the file handle for further operations.
// Caller is responsible for closing the file.
func CreateFileWithOwnership(path string) (*os.File, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	// Fix ownership immediately after creation
	_ = FixFileOwnership(path) // Ignore errors, non-critical
	return f, nil
}

// OpenFileWithOwnership opens a file with flags and fixes ownership when running with sudo.
// Useful for append operations or custom flags.
// Caller is responsible for closing the file.
func OpenFileWithOwnership(path string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	// Fix ownership if file was created (O_CREATE flag)
	if flag&os.O_CREATE != 0 {
		_ = FixFileOwnership(path) // Ignore errors, non-critical
	}
	return f, nil
}
