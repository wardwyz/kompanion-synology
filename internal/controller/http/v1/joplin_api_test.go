package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/notes"
	"github.com/vanadium23/kompanion/pkg/logger"
)

func TestJoplinAPI_CreateAndListNotes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())
	jg := r.Group("/joplin")
	jg.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(jg, noteSvc, logger.New("error"))

	payload := map[string]string{
		"title":       "My KOReader Note",
		"body":        "hello\nKOReader_partial_md5: doc123",
		"source":      "koreader",
		"document_id": "",
	}
	body, _ := json.Marshal(payload)

	createReq := httptest.NewRequest(http.MethodPost, "/joplin/notes?token=test-token", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/joplin/notes?token=test-token", nil)
	listResp := httptest.NewRecorder()
	r.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}

	var out struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected 1 note, got %d", len(out.Items))
	}
	if got := out.Items[0]["document_id"]; got != "doc123" {
		t.Fatalf("expected extracted document_id doc123, got %v", got)
	}
}

func TestJoplinAPI_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())
	jg := r.Group("/joplin")
	jg.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(jg, noteSvc, logger.New("error"))

	req := httptest.NewRequest(http.MethodGet, "/joplin/ping?token=wrong", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}
