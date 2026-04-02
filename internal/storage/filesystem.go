package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FilesystemStorage struct {
	// contains filtered or unexported fields
	root string
}

func NewFilesystemStorage(root string) (*FilesystemStorage, error) {
	// Try to create the root directory
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	dirPath := filepath.Dir(root)
	err := os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	// Check system writes on the provided root
	err = checkSystemWrites(root)
	if err != nil {
		return nil, err
	}

	return &FilesystemStorage{root: root}, nil
}

func (s *FilesystemStorage) Read(ctx context.Context, p string) (*os.File, error) {
	resolvedPath, err := resolveStoragePath(s.root, p)
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(resolvedPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return os.Open(resolvedPath)
}

func (s *FilesystemStorage) Write(ctx context.Context, src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dst, err := resolveStoragePath(s.root, dest)
	if err != nil {
		return err
	}

	dirPath := filepath.Dir(dst)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func (s *FilesystemStorage) Delete(ctx context.Context, p string) error {
	resolvedPath, err := resolveStoragePath(s.root, p)
	if err != nil {
		return err
	}

	err = os.Remove(resolvedPath)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}

	return err
}

func resolveStoragePath(root, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", fmt.Errorf("invalid path: empty paths are not allowed")
	}

	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("invalid path: absolute paths are not allowed")
	}

	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(relativePath)
	if cleanPath == "." {
		return "", fmt.Errorf("invalid path: root path is not allowed")
	}

	fullPath := filepath.Join(cleanRoot, cleanPath)
	rel, err := filepath.Rel(cleanRoot, fullPath)
	if err != nil {
		return "", err
	}

	if rel == ".." || strings.HasPrefix(rel, fmt.Sprintf("..%c", filepath.Separator)) {
		return "", fmt.Errorf("invalid path: path traversal is not allowed")
	}

	return fullPath, nil
}

func checkSystemWrites(root string) error {
	// Create a temporary file in the root directory
	tempFile, err := os.CreateTemp(root, "write_test")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// Try to write to the temporary file
	_, err = tempFile.WriteString("test")
	if err != nil {
		return err
	}

	return nil
}
