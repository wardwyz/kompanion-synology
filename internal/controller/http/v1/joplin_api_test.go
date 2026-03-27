package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if got := out.Items[0]["parent_id"]; got != "kompanion" {
		t.Fatalf("expected parent_id kompanion, got %v", got)
	}
	if !strings.Contains(listResp.Body.String(), `"has_more":false`) {
		t.Fatalf("expected has_more in list response, got %s", listResp.Body.String())
	}
}

func TestJoplinAPI_CreateNoteWithoutTitleOrBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())
	jg := r.Group("/joplin")
	jg.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(jg, noteSvc, logger.New("error"))

	payload := map[string]string{
		"source_url": "https://example.local?book=1&koreader_partial_md5=docxyz",
	}
	body, _ := json.Marshal(payload)

	createReq := httptest.NewRequest(http.MethodPost, "/joplin/notes?token=test-token", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	var out map[string]interface{}
	if err := json.Unmarshal(createResp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if got := out["title"]; got != "KOReader Note docxyz" {
		t.Fatalf("expected fallback title, got %v", got)
	}
	if got := out["body"]; got != "" {
		t.Fatalf("expected empty body, got %v", got)
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

func TestJoplinAPI_FolderNotesEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())
	jg := r.Group("/joplin")
	jg.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(jg, noteSvc, logger.New("error"))

	payload := map[string]string{
		"body": "highlight body",
	}
	body, _ := json.Marshal(payload)

	createReq := httptest.NewRequest(http.MethodPost, "/joplin/folders/kompanion/notes?token=test-token", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/joplin/folders/kompanion/notes?token=test-token", nil)
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

	folderReq := httptest.NewRequest(http.MethodGet, "/joplin/folders/kompanion?token=test-token", nil)
	folderResp := httptest.NewRecorder()
	r.ServeHTTP(folderResp, folderReq)
	if folderResp.Code != http.StatusOK {
		t.Fatalf("folder status = %d, body = %s", folderResp.Code, folderResp.Body.String())
	}
}

func TestJoplinAPI_LegacyRootPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())

	joplinRoutes := r.Group("/joplin")
	joplinRoutes.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(joplinRoutes, noteSvc, logger.New("error"))

	legacyRoutes := r.Group("/")
	legacyRoutes.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(legacyRoutes, noteSvc, logger.New("error"))

	req := httptest.NewRequest(http.MethodGet, "/ping?token=test-token", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	if got := resp.Body.String(); got != "JoplinClipperServer" {
		t.Fatalf("unexpected ping body: %s", got)
	}
}

func TestJoplinAPI_LegacyRootPathCreateAndReadMarkdownNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())

	joplinRoutes := r.Group("/joplin")
	joplinRoutes.Use(joplinTokenMiddleware("custom-token-8322"))
	newJoplinRoutes(joplinRoutes, noteSvc, logger.New("error"))

	legacyRoutes := r.Group("/")
	legacyRoutes.Use(joplinTokenMiddleware("custom-token-8322"))
	newJoplinRoutes(legacyRoutes, noteSvc, logger.New("error"))

	markdownBody := "# Reading note\n\n- highlight one\n- highlight two\n"
	payload := map[string]string{
		"title": "Markdown upload check",
		"body":  markdownBody,
	}
	body, _ := json.Marshal(payload)

	createReq := httptest.NewRequest(http.MethodPost, "/notes?token=custom-token-8322", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/notes?token=custom-token-8322", nil)
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
	gotBody, _ := out.Items[0]["body"].(string)
	if strings.TrimRight(gotBody, "\n") != strings.TrimRight(markdownBody, "\n") {
		t.Fatalf("expected markdown body %q, got %q", markdownBody, gotBody)
	}
}

func TestJoplinAPI_PutCreatesMissingNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())

	jg := r.Group("/joplin")
	jg.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(jg, noteSvc, logger.New("error"))

	payload := map[string]string{
		"body": "created-via-put",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/joplin/notes/fixed-id-1?token=test-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("put status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal put response: %v", err)
	}
	if got := out["id"]; got != "fixed-id-1" {
		t.Fatalf("expected id fixed-id-1, got %v", got)
	}
	if got := out["parent_id"]; got != "kompanion" {
		t.Fatalf("expected parent_id kompanion, got %v", got)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/joplin/notes/fixed-id-1?token=test-token", nil)
	getResp := httptest.NewRecorder()
	r.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResp.Code, getResp.Body.String())
	}
}

func TestJoplinAPI_GetNoteNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	noteSvc := notes.NewService(notes.NewMemoryRepo())

	jg := r.Group("/joplin")
	jg.Use(joplinTokenMiddleware("test-token"))
	newJoplinRoutes(jg, noteSvc, logger.New("error"))

	req := httptest.NewRequest(http.MethodGet, "/joplin/notes/missing?token=test-token", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", resp.Code, resp.Body.String())
	}
}
