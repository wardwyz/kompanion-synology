package library

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/storage"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type flowTestRepo struct {
	book entity.Book
}

func (r *flowTestRepo) Store(context.Context, entity.Book) error { return nil }
func (r *flowTestRepo) List(context.Context, string, string, int, int) ([]entity.Book, error) {
	return []entity.Book{r.book}, nil
}
func (r *flowTestRepo) Count(context.Context) (int, error) { return 1, nil }
func (r *flowTestRepo) GetById(context.Context, string) (entity.Book, error) {
	return r.book, nil
}
func (r *flowTestRepo) GetByFileHash(context.Context, string) (entity.Book, error) {
	return entity.Book{}, errors.New("not found")
}
func (r *flowTestRepo) Update(_ context.Context, b entity.Book) error {
	if b.FilePath == "" {
		b.FilePath = r.book.FilePath
	}
	if b.Format == "" {
		b.Format = r.book.Format
	}
	if b.DocumentID == "" {
		b.DocumentID = r.book.DocumentID
	}
	if b.CoverPath == "" {
		b.CoverPath = r.book.CoverPath
	}
	r.book = b
	return nil
}
func (r *flowTestRepo) Delete(context.Context, string) error { return nil }

func TestUploadScrapeThenDownloadDoesNotRewriteFileMetadata(t *testing.T) {
	ctx := context.Background()

	uploaded := createTempFlowFile(t, "original file content")
	defer os.Remove(uploaded.Name())
	defer uploaded.Close()

	st := storage.NewMemoryStorage()
	storagePath := "2026/04/02/test__book.epub"
	if err := st.Write(ctx, uploaded.Name(), storagePath); err != nil {
		t.Fatalf("write upload to storage failed: %v", err)
	}

	repo := &flowTestRepo{
		book: entity.Book{
			ID:          "book-1",
			Title:       "Old Title",
			Author:      "Author A",
			Description: "Desc",
			FilePath:    storagePath,
			Format:      "epub",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	shelf := NewBookShelf(st, repo, logger.New("error"))

	if _, err := shelf.UpdateBookMetadata(ctx, "book-1", entity.Book{Title: "豆瓣新标题"}); err != nil {
		t.Fatalf("update metadata failed: %v", err)
	}

	_, downloaded, err := shelf.DownloadBook(ctx, "book-1")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer downloaded.Close()

	downloadedData, err := os.ReadFile(downloaded.Name())
	if err != nil {
		t.Fatalf("read downloaded failed: %v", err)
	}

	if string(downloadedData) != "original file content" {
		t.Fatalf("expected downloaded file unchanged, got: %s", string(downloadedData))
	}
}

func createTempFlowFile(t *testing.T, content string) *os.File {
	t.Helper()

	tmp, err := os.CreateTemp("", "flow-file-test-*")
	if err != nil {
		t.Fatalf("create temp failed: %v", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write temp file failed: %v", err)
	}

	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("seek temp file failed: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp file failed: %v", err)
	}
	reopened, err := os.Open(tmp.Name())
	if err != nil {
		t.Fatalf("reopen temp file failed: %v", err)
	}
	return reopened
}
