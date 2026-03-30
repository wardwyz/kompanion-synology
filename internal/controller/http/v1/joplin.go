package v1

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/notes"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type joplinRoutes struct {
	notes notes.Service
	l     logger.Interface
}

const defaultJoplinFolderID = "kompanion"

type joplinNotePayload struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Source     string `json:"source"`
	SourceURL  string `json:"source_url"`
	ParentID   string `json:"parent_id"`
	DocumentID string `json:"document_id"`
}

func newJoplinRoutes(handler *gin.RouterGroup, n notes.Service, l logger.Interface) {
	r := &joplinRoutes{notes: n, l: l}

	h := handler.Group("/")
	{
		h.GET("/ping", r.ping)
		h.GET("/folders", r.listFolders)
		h.POST("/folders", r.createFolder)
		h.GET("/folders/:id", r.getFolder)
		h.GET("/folders/:id/notes", r.listNotes)
		h.POST("/folders/:id/notes", r.createNote)
		h.POST("/notes", r.createNote)
		h.GET("/notes/:id", r.getNote)
		h.PUT("/notes/:id", r.updateNote)
		h.GET("/notes", r.listNotes)
	}
}

func (r *joplinRoutes) ping(c *gin.Context) {
	c.String(http.StatusOK, "JoplinClipperServer")
}

func (r *joplinRoutes) listFolders(c *gin.Context) {
	// KOReader usually needs at least one notebook in the response.
	c.JSON(http.StatusOK, gin.H{
		"items":    []gin.H{{"id": defaultJoplinFolderID, "title": "KOReader Notes"}},
		"has_more": false,
	})
}

func (r *joplinRoutes) getFolder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "title": "KOReader Notes"})
}

func (r *joplinRoutes) createFolder(c *gin.Context) {
	payload, err := parseJoplinFolderPayload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = "KOReader Notes"
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    defaultJoplinFolderID,
		"title": title,
	})
}

func (r *joplinRoutes) createNote(c *gin.Context) {
	payload, err := parseJoplinNotePayload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	note := entity.ReadingNote{
		ID:         payload.ID,
		Title:      normalizeJoplinTitle(payload),
		Body:       strings.TrimSpace(payload.Body),
		Source:     payload.Source,
		SourceURL:  payload.SourceURL,
		DocumentID: payload.DocumentID,
	}
	if note.DocumentID == "" {
		note.DocumentID = extractDocumentID(payload)
	}

	saved, err := r.notes.Save(c.Request.Context(), note)
	if err != nil {
		r.l.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, toJoplinResponse(saved))
}

func (r *joplinRoutes) updateNote(c *gin.Context) {
	payload, err := parseJoplinNotePayload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload.ID = c.Param("id")

	note := entity.ReadingNote{
		ID:         payload.ID,
		Title:      normalizeJoplinTitle(payload),
		Body:       strings.TrimSpace(payload.Body),
		Source:     payload.Source,
		SourceURL:  payload.SourceURL,
		DocumentID: payload.DocumentID,
	}
	if note.DocumentID == "" {
		note.DocumentID = extractDocumentID(payload)
	}

	updated, err := r.notes.Update(c.Request.Context(), note)
	if err != nil {
		r.l.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, toJoplinResponse(updated))
}

func parseJoplinNotePayload(c *gin.Context) (joplinNotePayload, error) {
	var payload joplinNotePayload

	raw, err := c.GetRawData()
	if err != nil {
		return joplinNotePayload{}, err
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(raw)))

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return payloadFromForm(c), nil
	}

	if err := json.Unmarshal(raw, &payload); err == nil {
		return payload, nil
	}

	return payloadFromForm(c), nil
}

func payloadFromForm(c *gin.Context) joplinNotePayload {
	_ = c.Request.ParseForm()
	formOrQuery := func(key string) string {
		if v := c.PostForm(key); v != "" {
			return v
		}
		return c.Query(key)
	}
	return joplinNotePayload{
		ID:         strings.TrimSpace(formOrQuery("id")),
		Title:      formOrQuery("title"),
		Body:       formOrQuery("body"),
		Source:     formOrQuery("source"),
		SourceURL:  formOrQuery("source_url"),
		ParentID:   formOrQuery("parent_id"),
		DocumentID: formOrQuery("document_id"),
	}
}

type joplinFolderPayload struct {
	Title string `json:"title"`
}

func parseJoplinFolderPayload(c *gin.Context) (joplinFolderPayload, error) {
	var payload joplinFolderPayload

	raw, err := c.GetRawData()
	if err != nil {
		return joplinFolderPayload{}, err
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(raw)))

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return joplinFolderPayload{Title: c.PostForm("title")}, nil
	}

	if err := json.Unmarshal(raw, &payload); err == nil {
		return payload, nil
	}

	return joplinFolderPayload{Title: c.PostForm("title")}, nil
}

func (r *joplinRoutes) listNotes(c *gin.Context) {
	items, err := r.notes.List(c.Request.Context(), 200)
	if err != nil {
		r.l.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	responseItems := make([]gin.H, 0, len(items))
	for _, note := range items {
		responseItems = append(responseItems, toJoplinResponse(note))
	}
	c.JSON(http.StatusOK, gin.H{"items": responseItems, "has_more": false})
}

func (r *joplinRoutes) getNote(c *gin.Context) {
	note, err := r.notes.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, notes.ErrNoteNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		r.l.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, toJoplinResponse(note))
}

func toJoplinResponse(note entity.ReadingNote) gin.H {
	return gin.H{
		"id":          note.ID,
		"title":       note.Title,
		"body":        note.Body,
		"document_id": note.DocumentID,
		"source":      note.Source,
		"source_url":  note.SourceURL,
		"created_at":  note.CreatedAt,
		"updated_at":  note.UpdatedAt,
		"parent_id":   defaultJoplinFolderID,
	}
}

func extractDocumentID(payload joplinNotePayload) string {
	lowerBody := strings.ToLower(payload.Body)
	const marker = "koreader_partial_md5:"
	if idx := strings.Index(lowerBody, marker); idx >= 0 {
		rest := strings.TrimSpace(payload.Body[idx+len(marker):])
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			return strings.TrimSpace(fields[0])
		}
	}
	if strings.Contains(payload.SourceURL, "koreader_partial_md5=") {
		parts := strings.Split(payload.SourceURL, "koreader_partial_md5=")
		if len(parts) > 1 {
			return strings.TrimSpace(strings.Split(parts[1], "&")[0])
		}
	}
	return ""
}

func normalizeJoplinTitle(payload joplinNotePayload) string {
	title := strings.TrimSpace(payload.Title)
	if title != "" {
		return title
	}

	if payload.DocumentID != "" {
		return "KOReader Note " + strings.TrimSpace(payload.DocumentID)
	}

	if extracted := extractDocumentID(payload); extracted != "" {
		return "KOReader Note " + extracted
	}

	return "KOReader Note"
}

func joplinTokenMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && strings.HasSuffix(c.Request.URL.Path, "/ping") {
			c.Next()
			return
		}
		if c.Query("token") != token {
			c.JSON(http.StatusForbidden, gin.H{"error": `Invalid "token" parameter`})
			c.Abort()
			return
		}
		c.Next()
	}
}
