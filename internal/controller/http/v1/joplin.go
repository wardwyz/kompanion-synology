package v1

import (
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
		h.GET("/ping/", r.ping)
		h.GET("/folders", r.listFolders)
		h.GET("/folders/", r.listFolders)
		h.GET("/folders/:id/notes", r.listNotes)
		h.GET("/folders/:id/notes/", r.listNotes)
		h.POST("/folders/:id/notes", r.createNote)
		h.POST("/folders/:id/notes/", r.createNote)
		h.POST("/notes", r.createNote)
		h.POST("/notes/", r.createNote)
		h.PUT("/notes/:id", r.updateNote)
		h.PUT("/notes/:id/", r.updateNote)
		h.GET("/notes", r.listNotes)
		h.GET("/notes/", r.listNotes)
	}
}

func (r *joplinRoutes) ping(c *gin.Context) {
	c.String(http.StatusOK, "JoplinClipperServer")
}

func (r *joplinRoutes) listFolders(c *gin.Context) {
	// KOReader usually needs at least one notebook in the response.
	c.JSON(http.StatusOK, gin.H{"items": []gin.H{{"id": "kompanion", "title": "KOReader Notes"}}})
}

func (r *joplinRoutes) createNote(c *gin.Context) {
	var payload joplinNotePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
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

	c.JSON(http.StatusOK, saved)
}

func (r *joplinRoutes) updateNote(c *gin.Context) {
	var payload joplinNotePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
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

	c.JSON(http.StatusOK, updated)
}

func (r *joplinRoutes) listNotes(c *gin.Context) {
	items, err := r.notes.List(c.Request.Context(), 200)
	if err != nil {
		r.l.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
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
		queryToken := strings.TrimSpace(c.Query("token"))
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		bearerToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		xAPIHeaderToken := strings.TrimSpace(c.GetHeader("X-API-Token"))

		if queryToken != token && bearerToken != token && xAPIHeaderToken != token {
			c.JSON(http.StatusForbidden, gin.H{"error": `Invalid "token" parameter`})
			c.Abort()
			return
		}
		c.Next()
	}
}
