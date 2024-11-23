package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirectory(t *testing.T) {
	dir := "testdir"
	fs := &FilesystemImpl{} // Create an instance of FilesystemImpl

	// Clean up after the test
	defer os.RemoveAll(dir)

	// Test creating a new directory
	if err := fs.EnsureDirectory(dir); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("Expected directory %s to exist, but it does not", dir)
	}
}

func TestWriteFile(t *testing.T) {
	dir := "testdir"
	filePath := filepath.Join(dir, "testfile.txt")
	data := "Hello, World!"
	fs := &FilesystemImpl{} // Create an instance of FilesystemImpl

	// Clean up after the test
	defer os.RemoveAll(dir)

	// Ensure the directory exists
	if err := fs.EnsureDirectory(dir); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test writing to a file
	if err := fs.WriteFile(filePath, data); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check if the file exists and read its content
	content, err := fs.ReadFile(filePath) // Updated to use fs
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if content != data {
		t.Fatalf("Expected file content %q, got %q", data, content)
	}
}

func TestReadFile(t *testing.T) {
	dir := "testdir"
	filePath := filepath.Join(dir, "testfile.txt")
	data := "Hello, World!"
	fs := &FilesystemImpl{} // Create an instance of FilesystemImpl

	// Clean up after the test
	defer os.RemoveAll(dir)

	// Ensure the directory exists and write a file
	if err := fs.EnsureDirectory(dir); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}
	if err := fs.WriteFile(filePath, data); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test reading from the file
	content, err := fs.ReadFile(filePath) // Updated to use fs
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if content != data {
		t.Fatalf("Expected file content %q, got %q", data, content)
	}
}

func TestDeleteFile(t *testing.T) {
	dir := "testdir"
	filePath := filepath.Join(dir, "testfile.txt")
	fs := &FilesystemImpl{} // Create an instance of FilesystemImpl

	// Clean up after the test
	defer os.RemoveAll(dir)

	// Ensure the directory exists and create a file
	if err := fs.EnsureDirectory(dir); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}
	if err := fs.WriteFile(filePath, "data"); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test deleting the file
	if err := fs.DeleteFile(filePath); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check if the file has been deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("Expected file %s to be deleted, but it still exists", filePath)
	}
}

func TestListFiles(t *testing.T) {
	dir := "testdir"
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	fs := &FilesystemImpl{} // Create an instance of FilesystemImpl

	// Clean up after the test
	defer os.RemoveAll(dir)

	// Ensure the directory exists
	if err := fs.EnsureDirectory(dir); err != nil { // Updated to use fs
		t.Fatalf("Expected no error, got %v", err)
	}

	// Create test files
	for _, file := range files {
		if err := fs.WriteFile(filepath.Join(dir, file), "data"); err != nil { // Updated to use fs
			t.Fatalf("Expected no error, got %v", err)
		}
	}

	// Test listing files
	listedFiles, err := fs.ListFiles(dir) // Updated to use fs
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check if the listed files match the expected files
	for _, file := range files {
		found := false
		for _, listedFile := range listedFiles {
			if listedFile == file {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Expected to find file %s in the listed files", file)
		}
	}
}
