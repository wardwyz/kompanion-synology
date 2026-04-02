package storage_test

import (
	"context"
	"os"
	"testing"

	"github.com/vanadium23/kompanion/internal/storage"
)

func TestFilesystemStorage(t *testing.T) {
	ctx := context.Background()
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	st, err := storage.NewFilesystemStorage(tmpdir)
	if err != nil {
		t.Fatalf("Error creating filesystem storage: %v", err)
	}

	body := []byte("Hello, World!")
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	_, err = tempFile.Write(body)
	if err != nil {
		t.Fatalf("Error writing to temp file: %v", err)
	}

	defer os.Remove(tempFile.Name())

	err = st.Write(ctx, tempFile.Name(), "test")
	if err != nil {
		t.Errorf("Error writing file: %v", err)
	}

	readFile, err := st.Read(ctx, "test")
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}
	defer readFile.Close()
	readBody, err := os.ReadFile(readFile.Name())
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}
	if string(readBody) != string(body) {
		t.Errorf("Expected body %s, got %s", string(body), string(readBody))
	}

	err = st.Delete(ctx, "test")
	if err != nil {
		t.Errorf("Error deleting file: %v", err)
	}

	_, err = st.Read(ctx, "test")
	if err == nil {
		t.Errorf("Expected read error after delete, got nil")
	}
}

func TestFilesystemStorage_RejectsPathTraversal(t *testing.T) {
	ctx := context.Background()
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	st, err := storage.NewFilesystemStorage(tmpdir)
	if err != nil {
		t.Fatalf("Error creating filesystem storage: %v", err)
	}

	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	for _, invalidPath := range []string{"../escape", "../../etc/passwd", "/tmp/absolute"} {
		err = st.Write(ctx, tempFile.Name(), invalidPath)
		if err == nil {
			t.Fatalf("expected write to fail for invalid path %q", invalidPath)
		}

		_, err = st.Read(ctx, invalidPath)
		if err == nil {
			t.Fatalf("expected read to fail for invalid path %q", invalidPath)
		}

		err = st.Delete(ctx, invalidPath)
		if err == nil {
			t.Fatalf("expected delete to fail for invalid path %q", invalidPath)
		}
	}
}

func TestFilesystemStorage_RejectsEmptyOrRootRelativePath(t *testing.T) {
	ctx := context.Background()
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	st, err := storage.NewFilesystemStorage(tmpdir)
	if err != nil {
		t.Fatalf("Error creating filesystem storage: %v", err)
	}

	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	for _, invalidPath := range []string{"", " ", ".", "./"} {
		err = st.Write(ctx, tempFile.Name(), invalidPath)
		if err == nil {
			t.Fatalf("expected write to fail for invalid path %q", invalidPath)
		}

		_, err = st.Read(ctx, invalidPath)
		if err == nil {
			t.Fatalf("expected read to fail for invalid path %q", invalidPath)
		}

		err = st.Delete(ctx, invalidPath)
		if err == nil {
			t.Fatalf("expected delete to fail for invalid path %q", invalidPath)
		}
	}
}

func TestFilesystemStorage_AllowsNestedRelativePath(t *testing.T) {
	ctx := context.Background()
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	st, err := storage.NewFilesystemStorage(tmpdir)
	if err != nil {
		t.Fatalf("Error creating filesystem storage: %v", err)
	}

	body := []byte("nested content")
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write(body)
	if err != nil {
		t.Fatalf("Error writing temp file: %v", err)
	}

	dest := "nested/path/book.txt"
	err = st.Write(ctx, tempFile.Name(), dest)
	if err != nil {
		t.Fatalf("Error writing nested path: %v", err)
	}

	readFile, err := st.Read(ctx, dest)
	if err != nil {
		t.Fatalf("Error reading nested path: %v", err)
	}
	defer readFile.Close()

	readBody, err := os.ReadFile(readFile.Name())
	if err != nil {
		t.Fatalf("Error reading nested file body: %v", err)
	}
	if string(readBody) != string(body) {
		t.Fatalf("Expected body %q, got %q", string(body), string(readBody))
	}

	err = st.Delete(ctx, dest)
	if err != nil {
		t.Fatalf("Error deleting nested path: %v", err)
	}
}
