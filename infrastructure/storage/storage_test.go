package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageSaveRejectsDirectoryTraversal(t *testing.T) {
	root := t.TempDir()
	store := NewLocalStorage(root)

	_, err := store.Save(context.Background(), "../escape", ".txt", strings.NewReader("payload"))
	if err == nil {
		t.Fatal("expected directory traversal to be rejected")
	}

	if _, statErr := os.Stat(filepath.Join(root, "..", "escape")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no directory outside root, got stat error %v", statErr)
	}
}

func TestLocalStorageSaveRejectsUnsafeExtension(t *testing.T) {
	root := t.TempDir()
	store := NewLocalStorage(root)

	_, err := store.Save(context.Background(), "avatars", "/../../escape", strings.NewReader("payload"))
	if err == nil {
		t.Fatal("expected unsafe extension to be rejected")
	}
}

func TestLocalStorageSaveCreatesPrivateFileUnderRoot(t *testing.T) {
	root := t.TempDir()
	store := NewLocalStorage(root)

	path, err := store.Save(context.Background(), "avatars", ".txt", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("root abs failed: %v", err)
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("path abs failed: %v", err)
	}
	if !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		t.Fatalf("expected path %q under root %q", cleanPath, cleanRoot)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected file mode 0600, got %v", info.Mode().Perm())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(content, []byte("payload")) {
		t.Fatalf("unexpected file content %q", string(content))
	}
}
