package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileWithOwnership(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Write file
	data := []byte("test content")
	err := WriteFileWithOwnership(testFile, data, 0644)
	if err != nil {
		t.Fatalf("WriteFileWithOwnership failed: %v", err)
	}

	// Verify file exists and content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Content mismatch: got %q, want %q", string(content), "test content")
	}
}

func TestMkdirAllWithOwnership(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c")

	err := MkdirAllWithOwnership(nested, 0755)
	if err != nil {
		t.Fatalf("MkdirAllWithOwnership failed: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}
}

func TestCreateFileWithOwnership(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "created.txt")

	f, err := CreateFileWithOwnership(testFile)
	if err != nil {
		t.Fatalf("CreateFileWithOwnership failed: %v", err)
	}
	defer f.Close()

	// Write to file
	_, err = f.WriteString("hello")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	_, err = os.Stat(testFile)
	if err != nil {
		t.Fatalf("File not created: %v", err)
	}
}

func TestOpenFileWithOwnership(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "append.txt")

	// Create with append flag
	f, err := OpenFileWithOwnership(testFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("OpenFileWithOwnership failed: %v", err)
	}
	f.WriteString("line1\n")
	f.Close()

	// Append more
	f2, err := OpenFileWithOwnership(testFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("OpenFileWithOwnership (append) failed: %v", err)
	}
	f2.WriteString("line2\n")
	f2.Close()

	// Verify content
	content, _ := os.ReadFile(testFile)
	if string(content) != "line1\nline2\n" {
		t.Errorf("Content mismatch: got %q", string(content))
	}
}
