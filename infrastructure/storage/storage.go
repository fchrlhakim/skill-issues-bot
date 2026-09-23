package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Storage interface {
	Save(ctx context.Context, directory, extension string, reader io.Reader) (string, error)
}

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) Storage {
	return &LocalStorage{root: root}
}

func (s *LocalStorage) Save(ctx context.Context, directory, extension string, reader io.Reader) (string, error) {
	if extension == "" {
		extension = ".bin"
	}
	if !safeExtension(extension) {
		return "", fmt.Errorf("invalid storage extension")
	}
	dir, err := s.safeDirectory(directory)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, uuid.NewString()+extension)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- directory and extension are validated before constructing the path.
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, reader); err != nil {
		return "", err
	}
	return path, ctx.Err()
}

func (s *LocalStorage) safeDirectory(directory string) (string, error) {
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	cleanDirectory := filepath.Clean(directory)
	if cleanDirectory == "." || filepath.IsAbs(cleanDirectory) || strings.HasPrefix(cleanDirectory, ".."+string(os.PathSeparator)) || cleanDirectory == ".." {
		return "", fmt.Errorf("invalid storage directory")
	}
	dir := filepath.Join(root, cleanDirectory)
	cleanDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if cleanDir != root && !strings.HasPrefix(cleanDir, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid storage directory")
	}
	return cleanDir, nil
}

func safeExtension(extension string) bool {
	if !strings.HasPrefix(extension, ".") || strings.ContainsAny(extension, `/\`) {
		return false
	}
	for _, char := range extension[1:] {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return len(extension) > 1
}
