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
	newJoplinRoutes(jg, noteSvc, logger.New("error"), "test-token")

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
	newJoplinRoutes(jg, noteSvc, logger.New("error"), "test-token")

	req := httptest.NewRequest(http.MethodGet, "/joplin/notes?token=wrong", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestJoplinAPI_KOReaderFlowOnRootPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())
	newJoplinRoutes(r.Group("/"), noteSvc, logger.New("error"), "test-token")

	pingReq := httptest.NewRequest(http.MethodGet, "/ping", nil)
	pingResp := httptest.NewRecorder()
	r.ServeHTTP(pingResp, pingReq)
	if pingResp.Code != http.StatusOK || pingResp.Body.String() != "JoplinClipperServer" {
		t.Fatalf("ping status = %d, body = %s", pingResp.Code, pingResp.Body.String())
	}

	foldersReq := httptest.NewRequest(http.MethodGet, "/folders?token=test-token", nil)
	foldersResp := httptest.NewRecorder()
	r.ServeHTTP(foldersResp, foldersReq)
	if foldersResp.Code != http.StatusOK {
		t.Fatalf("folders status = %d, body = %s", foldersResp.Code, foldersResp.Body.String())
	}

	createBody := []byte(`{"title":"book","body":"v1","parent_id":"kompanion"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/notes?token=test-token", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created id")
	}

	updateBody := []byte(`{"body":"v2"}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/notes/"+created.ID+"?token=test-token", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	r.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/notes?token=test-token&fields=id,title,parent_id&page=1", nil)
	listResp := httptest.NewRecorder()
	r.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listOut struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listOut); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listOut.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(listOut.Items))
	}
	if listOut.Items[0]["parent_id"] != "kompanion" {
		t.Fatalf("expected parent_id kompanion, got %v", listOut.Items[0]["parent_id"])
	}
	if _, ok := listOut.Items[0]["body"]; ok {
		t.Fatalf("did not expect body in fields-filtered response: %v", listOut.Items[0])
	}
}
