package filesystem

import (
	"fmt"
	"io/ioutil"
	"os"
)

// Filesystem interface for abstraction
type Filesystem interface {
	EnsureDirectory(path string) error
	WriteFile(path string, data string) error
	ReadFile(path string) (string, error)
	DeleteFile(path string) error
	ListFiles(directory string) ([]string, error)
}

// FilesystemImpl is a concrete implementation of the Filesystem interface.
type FilesystemImpl struct{}

// EnsureDirectory ensures that the specified directory exists.
func (fs *FilesystemImpl) EnsureDirectory(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}

// WriteFile writes data to a file at the specified path.
func (fs *FilesystemImpl) WriteFile(path string, data string) error {
	fileWriteHandler, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer fileWriteHandler.Close()

	_, err = fileWriteHandler.WriteString(data)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", path, err)
	}
	return fileWriteHandler.Sync() // Ensure data is flushed to disk
}

// ReadFile reads data from a file at the specified path.
func (fs *FilesystemImpl) ReadFile(path string) (string, error) {
	dataBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return string(dataBytes), nil
}

// DeleteFile deletes the file at the specified path.
func (fs *FilesystemImpl) DeleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}
	return nil
}

// ListFiles lists all files in the specified directory.
func (fs *FilesystemImpl) ListFiles(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var fileNames []string
	for _, file := range files {
		if !file.IsDir() { // Only include files, not directories
			fileNames = append(fileNames, file.Name())
		}
	}
	return fileNames, nil
}
