package opds

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/vanadium23/kompanion/internal/auth"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/library"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type testShelf struct {
	book entity.Book
	file *os.File
	err  error
}

func (s testShelf) StoreBook(context.Context, *os.File, string) (entity.Book, error) {
	return entity.Book{}, nil
}

func (s testShelf) ListBooks(context.Context, string, string, int, int) (library.PaginatedBookList, error) {
	return library.PaginatedBookList{}, nil
}

func (s testShelf) ViewBook(context.Context, string) (entity.Book, error) {
	return entity.Book{}, nil
}

func (s testShelf) DownloadBook(context.Context, string) (entity.Book, *os.File, error) {
	return s.book, s.file, s.err
}

func (s testShelf) UpdateBookMetadata(context.Context, string, entity.Book) (entity.Book, error) {
	return entity.Book{}, nil
}

func (s testShelf) DeleteBook(context.Context, string) error {
	return nil
}

func (s testShelf) ViewCover(context.Context, string) (*os.File, error) {
	return nil, os.ErrNotExist
}

type testAuth struct{}

func (a testAuth) CheckPassword(context.Context, string, string) bool { return true }
func (a testAuth) Login(context.Context, string, string, string, net.IP) (string, error) {
	return "", nil
}
func (a testAuth) IsAuthenticated(context.Context, string) bool { return true }
func (a testAuth) Logout(context.Context, string) error         { return nil }
func (a testAuth) RegisterUser(context.Context, string, string) error {
	return nil
}
func (a testAuth) AddUserDevice(context.Context, string, string) error { return nil }
func (a testAuth) DeactivateUserDevice(context.Context, string) error  { return nil }
func (a testAuth) CheckDevicePassword(context.Context, string, string, bool) bool {
	return true
}
func (a testAuth) ListDevices(context.Context) ([]auth.Device, error) { return nil, nil }

func TestOPDSDownloadBookSupportsRangeRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	file, err := os.CreateTemp(t.TempDir(), "book-*.epub")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = file.Close()
	})
	_, err = file.WriteString("0123456789")
	require.NoError(t, err)

	router := gin.New()
	NewRouter(router, logger.New("info"), testAuth{}, nil, testShelf{
		book: entity.Book{ID: "book-1", Title: "Test", Format: "epub"},
		file: file,
	})

	req := httptest.NewRequest(http.MethodGet, "/opds/book/book-1/download", nil)
	req.SetBasicAuth("device", "secret")
	req.Header.Set("Range", "bytes=0-3")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusPartialContent, resp.Code)
	require.Equal(t, "bytes", resp.Header().Get("Accept-Ranges"))
	require.Equal(t, "bytes 0-3/10", resp.Header().Get("Content-Range"))
	require.Equal(t, "0123", resp.Body.String())
}
