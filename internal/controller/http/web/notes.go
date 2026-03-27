package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/notes"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type notesRoutes struct {
	notes  notes.Service
	logger logger.Interface
}

func newNotesRoutes(handler *gin.RouterGroup, noteSvc notes.Service, l logger.Interface) {
	r := &notesRoutes{notes: noteSvc, logger: l}
	handler.GET("/", r.list)
}

func (r *notesRoutes) list(c *gin.Context) {
	items, err := r.notes.List(c.Request.Context(), 200)
	if err != nil {
		r.logger.Error(err, "failed to list notes")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}

	c.HTML(http.StatusOK, "notes", passStandartContext(c, gin.H{"notes": items}))
}
