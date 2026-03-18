package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/library"
	"github.com/vanadium23/kompanion/internal/stats"
	syncpkg "github.com/vanadium23/kompanion/internal/sync"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type shelfStub struct {
	storeBookFunc func(ctx context.Context, tempFile *os.File, uploadedFilename string) (entity.Book, error)
}

func (s shelfStub) StoreBook(ctx context.Context, tempFile *os.File, uploadedFilename string) (entity.Book, error) {
	return s.storeBookFunc(ctx, tempFile, uploadedFilename)
}

func (s shelfStub) ListBooks(ctx context.Context, sortBy, sortOrder string, page, perPage int) (library.PaginatedBookList, error) {
	return library.PaginatedBookList{}, nil
}

func (s shelfStub) ViewBook(ctx context.Context, bookID string) (entity.Book, error) {
	return entity.Book{}, nil
}

func (s shelfStub) DownloadBook(ctx context.Context, bookID string) (entity.Book, *os.File, error) {
	return entity.Book{}, nil, nil
}

func (s shelfStub) UpdateBookMetadata(ctx context.Context, bookID string, metadata entity.Book) (entity.Book, error) {
	return entity.Book{}, nil
}

func (s shelfStub) DeleteBook(ctx context.Context, bookID string) error {
	return nil
}

func (s shelfStub) ViewCover(ctx context.Context, bookID string) (*os.File, error) {
	return nil, nil
}

type statsStub struct{}

func (s statsStub) GetBookStats(ctx context.Context, fileHash string) (*stats.BookStats, error) {
	return &stats.BookStats{}, nil
}

func (s statsStub) GetGeneralStats(ctx context.Context, from, to time.Time) (*stats.GeneralStats, error) {
	return &stats.GeneralStats{}, nil
}

func (s statsStub) GetDailyStats(ctx context.Context, from, to time.Time) ([]stats.DailyStats, error) {
	return nil, nil
}

func (s statsStub) Write(ctx context.Context, r io.ReadCloser, deviceName string) error {
	return nil
}

type progressStub struct{}

func (p progressStub) Sync(ctx context.Context, progress entity.Progress) (entity.Progress, error) {
	return entity.Progress{}, nil
}

func (p progressStub) Fetch(ctx context.Context, bookID string) (entity.Progress, error) {
	return entity.Progress{}, nil
}

var _ library.Shelf = shelfStub{}
var _ stats.ReadingStats = statsStub{}
var _ syncpkg.Progress = progressStub{}

func TestUploadBook_MultipleFiles(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	storedFilenames := make([]string, 0, 2)
	router := gin.New()
	newBooksRoutes(router.Group("/books"), shelfStub{
		storeBookFunc: func(ctx context.Context, tempFile *os.File, uploadedFilename string) (entity.Book, error) {
			content, err := os.ReadFile(tempFile.Name())
			require.NoError(t, err)
			require.NotEmpty(t, content)

			storedFilenames = append(storedFilenames, uploadedFilename)
			return entity.Book{ID: uploadedFilename}, nil
		},
	}, statsStub{}, progressStub{}, logger.New("error"))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	addUploadFile(t, writer, "book", "first.epub", "first-book")
	addUploadFile(t, writer, "book", "second.pdf", "second-book")
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/books/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusFound, resp.Code)
	require.Equal(t, "/books", resp.Header().Get("Location"))
	require.Equal(t, []string{"first.epub", "second.pdf"}, storedFilenames)
}

func TestUploadBook_RequiresFile(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	newBooksRoutes(router.Group("/books"), shelfStub{
		storeBookFunc: func(ctx context.Context, tempFile *os.File, uploadedFilename string) (entity.Book, error) {
			return entity.Book{}, nil
		},
	}, statsStub{}, progressStub{}, logger.New("error"))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/books/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Equal(t, "book file is required", payload["message"])
}

func addUploadFile(t *testing.T, writer *multipart.Writer, fieldName, filename, content string) {
	t.Helper()

	part, err := writer.CreateFormFile(fieldName, filepath.Base(filename))
	require.NoError(t, err)

	_, err = part.Write([]byte(content))
	require.NoError(t, err)
}
