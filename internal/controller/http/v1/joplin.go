package v1

import (
	"net/http"
	"strconv"
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

type joplinNotePayload struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	ParentID   string `json:"parent_id"`
	Source     string `json:"source"`
	SourceURL  string `json:"source_url"`
	DocumentID string `json:"document_id"`
}

const defaultJoplinNotebookID = "kompanion"
const defaultJoplinNotebookTitle = "KOReader Notes"

func newJoplinRoutes(handler *gin.RouterGroup, n notes.Service, l logger.Interface, token string) {
	r := &joplinRoutes{notes: n, l: l}

	public := handler.Group("/")
	{
		public.GET("/ping", r.ping)
	}

	protected := handler.Group("/")
	protected.Use(joplinTokenMiddleware(token))
	{
		protected.GET("/folders", r.listFolders)
		protected.POST("/folders", r.createFolder)
		protected.POST("/notes", r.createNote)
		protected.PUT("/notes/:id", r.updateNote)
		protected.GET("/notes", r.listNotes)
	}
}

func (r *joplinRoutes) ping(c *gin.Context) {
	c.String(http.StatusOK, "JoplinClipperServer")
}

func (r *joplinRoutes) listFolders(c *gin.Context) {
	items := []gin.H{{"id": defaultJoplinNotebookID, "title": defaultJoplinNotebookTitle}}
	query := strings.TrimSpace(c.Query("query"))
	if query != "" && !strings.EqualFold(query, "title") && !strings.Contains(strings.ToLower(defaultJoplinNotebookTitle), strings.ToLower(query)) {
		items = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    applyFieldsToItems(items, c.Query("fields")),
		"has_more": false,
	})
}

func (r *joplinRoutes) createFolder(c *gin.Context) {
	var payload struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(payload.Title) == "" {
		payload.Title = defaultJoplinNotebookTitle
	}
	c.JSON(http.StatusOK, gin.H{"id": defaultJoplinNotebookID, "title": payload.Title})
}

func (r *joplinRoutes) createNote(c *gin.Context) {
	var payload joplinNotePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(payload.Title) == "" || strings.TrimSpace(payload.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and body are required"})
		return
	}

	note := entity.ReadingNote{
		ID:         payload.ID,
		Title:      payload.Title,
		Body:       payload.Body,
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

	c.JSON(http.StatusOK, saved)
}

func (r *joplinRoutes) updateNote(c *gin.Context) {
	var payload joplinNotePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	noteID := c.Param("id")
	existing, err := r.notes.Get(c.Request.Context(), noteID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
		return
	}

	note := existing
	note.ID = noteID
	if strings.TrimSpace(payload.Title) != "" {
		note.Title = payload.Title
	}
	if strings.TrimSpace(payload.Body) != "" {
		note.Body = payload.Body
	}
	if strings.TrimSpace(payload.Source) != "" {
		note.Source = payload.Source
	}
	if strings.TrimSpace(payload.SourceURL) != "" {
		note.SourceURL = payload.SourceURL
	}
	if strings.TrimSpace(payload.DocumentID) != "" {
		note.DocumentID = payload.DocumentID
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

	c.JSON(http.StatusOK, updated)
}

func (r *joplinRoutes) listNotes(c *gin.Context) {
	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := r.notes.List(c.Request.Context(), limit)
	if err != nil {
		r.l.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	parentIDFilter := strings.TrimSpace(c.Query("parent_id"))
	filtered := make([]gin.H, 0, len(items))
	for _, item := range items {
		row := gin.H{
			"id":          item.ID,
			"title":       item.Title,
			"body":        item.Body,
			"parent_id":   defaultJoplinNotebookID,
			"source":      item.Source,
			"source_url":  item.SourceURL,
			"document_id": item.DocumentID,
			"created_at":  item.CreatedAt.UnixMilli(),
			"updated_at":  item.UpdatedAt.UnixMilli(),
		}
		if parentIDFilter != "" && row["parent_id"] != parentIDFilter {
			continue
		}
		filtered = append(filtered, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    applyFieldsToItems(filtered, c.Query("fields")),
		"has_more": false,
	})
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

func applyFieldsToItems(items []gin.H, fieldsRaw string) []gin.H {
	fieldsRaw = strings.TrimSpace(fieldsRaw)
	if fieldsRaw == "" {
		return items
	}
	fields := strings.Split(fieldsRaw, ",")
	selected := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		selected[field] = struct{}{}
	}
	if len(selected) == 0 {
		return items
	}

	filtered := make([]gin.H, 0, len(items))
	for _, item := range items {
		row := gin.H{}
		for field := range selected {
			if value, ok := item[field]; ok {
				row[field] = value
			}
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func joplinTokenMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query("token") != token {
			c.JSON(http.StatusForbidden, gin.H{"error": `Invalid "token" parameter`})
			c.Abort()
			return
		}
		c.Next()
	}
}
