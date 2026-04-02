package library

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestUploadScrapeThenDownloadHasRewrittenTitleMetadata(t *testing.T) {
	ctx := context.Background()

	uploaded := buildFlowTestEPUB(t, `<package><metadata><dc:title>Old Title</dc:title></metadata></package>`)
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

	info, err := downloaded.Stat()
	if err != nil {
		t.Fatalf("stat downloaded failed: %v", err)
	}

	zr, err := zip.NewReader(downloaded, info.Size())
	if err != nil {
		t.Fatalf("open downloaded zip failed: %v", err)
	}

	opfBody := ""
	for _, f := range zr.File {
		if f.Name != filepath.ToSlash("OEBPS/content.opf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open opf failed: %v", err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		opfBody = string(data)
	}

	if !strings.Contains(opfBody, "<dc:title>豆瓣新标题</dc:title>") {
		t.Fatalf("expected downloaded metadata title updated, got: %s", opfBody)
	}
}

func buildFlowTestEPUB(t *testing.T, opfContent string) *os.File {
	t.Helper()

	tmp, err := os.CreateTemp("", "flow-metadata-test-*.epub")
	if err != nil {
		t.Fatalf("create temp failed: %v", err)
	}

	zw := zip.NewWriter(tmp)
	writeZipFile := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s failed: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s failed: %v", name, err)
		}
	}

	writeZipFile("META-INF/container.xml", `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`)
	writeZipFile(filepath.ToSlash("OEBPS/content.opf"), opfContent)

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
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
