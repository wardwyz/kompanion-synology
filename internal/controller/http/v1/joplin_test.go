package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/storage"
	"github.com/vanadium23/kompanion/pkg/logger"
)

func TestJoplinPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	newJoplinRoutes(engine.Group("/"), storage.NewMemoryStorage(), logger.New("error"))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "JoplinClipperServer" {
		t.Fatalf("expected JoplinClipperServer, got %q", rec.Body.String())
	}
}

func TestJoplinCreateAndUpdateNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memStorage := storage.NewMemoryStorage()
	engine := gin.New()
	newJoplinRoutes(engine.Group("/"), memStorage, logger.New("error"))

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/notes",
		strings.NewReader(`{"title":"test","body":"hello from KOReader"}`),
	)
	createReq.Header.Set("Content-Type", "application/json")

	createRec := httptest.NewRecorder()
	engine.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", createRec.Code)
	}

	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("cannot unmarshal create response: %v", err)
	}

	noteID, ok := created["id"].(string)
	if !ok || noteID == "" {
		t.Fatalf("expected id in response, got %v", created["id"])
	}

	noteFile, err := memStorage.Read(t.Context(), "joplin/"+noteID+".md")
	if err != nil {
		t.Fatalf("expected stored note file, got error %v", err)
	}
	defer noteFile.Close()

	updateReq := httptest.NewRequest(
		http.MethodPut,
		"/notes/"+noteID,
		strings.NewReader(`{"title":"test","body":"updated note body"}`),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	engine.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on update, got %d", updateRec.Code)
	}

	updatedFile, err := memStorage.Read(t.Context(), "joplin/"+noteID+".md")
	if err != nil {
		t.Fatalf("expected updated note file, got error %v", err)
	}
	defer updatedFile.Close()

	updatedBody, err := os.ReadFile(updatedFile.Name())
	if err != nil {
		t.Fatalf("expected to read updated note file, got error %v", err)
	}
	if string(updatedBody) != "updated note body" {
		t.Fatalf("expected updated content, got %q", string(updatedBody))
	}

	indexFile, err := memStorage.Read(t.Context(), "joplin/index.json")
	if err != nil {
		t.Fatalf("expected index file, got error %v", err)
	}
	defer indexFile.Close()

	var indexPayload map[string]any
	if err = json.NewDecoder(indexFile).Decode(&indexPayload); err != nil {
		t.Fatalf("cannot decode index file: %v", err)
	}
}
