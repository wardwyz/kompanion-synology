package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/storage"
	"github.com/vanadium23/kompanion/pkg/logger"
)

const joplinIndexPath = "joplin/index.json"

type joplinNoteMeta struct {
	ID         string `json:"id"`
	ParentID   string `json:"parent_id"`
	Title      string `json:"title"`
	StorageKey string `json:"storage_key"`
	UpdatedAt  string `json:"updated_at"`
}

type joplinNoteIndex struct {
	Items []joplinNoteMeta `json:"items"`
}

type notesRoutes struct {
	storage storage.Storage
	logger  logger.Interface
}

func newNotesRoutes(handler *gin.RouterGroup, storage storage.Storage, l logger.Interface) {
	r := &notesRoutes{
		storage: storage,
		logger:  l,
	}

	handler.GET("/", r.listNotes)
	handler.GET("/:noteID", r.viewNote)
}

func (r *notesRoutes) listNotes(c *gin.Context) {
	index, err := r.loadIndex(c)
	if err != nil {
		r.logger.Error(err, "http - web - notes - listNotes")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}

	c.HTML(http.StatusOK, "notes", passStandartContext(c, gin.H{
		"notes": index.Items,
	}))
}

func (r *notesRoutes) viewNote(c *gin.Context) {
	index, err := r.loadIndex(c)
	if err != nil {
		r.logger.Error(err, "http - web - notes - viewNote loadIndex")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}

	noteID := c.Param("noteID")
	noteIndex := slices.IndexFunc(index.Items, func(note joplinNoteMeta) bool {
		return note.ID == noteID
	})
	if noteIndex < 0 {
		c.HTML(http.StatusNotFound, "error", passStandartContext(c, gin.H{"error": "笔记不存在"}))
		return
	}

	note := index.Items[noteIndex]
	noteFile, err := r.storage.Read(c.Request.Context(), note.StorageKey)
	if err != nil {
		r.logger.Error(err, "http - web - notes - viewNote read")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": "无法读取笔记内容"}))
		return
	}
	defer noteFile.Close()

	content, err := os.ReadFile(noteFile.Name())
	if err != nil {
		r.logger.Error(err, "http - web - notes - viewNote read file")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": "无法读取笔记内容"}))
		return
	}

	c.HTML(http.StatusOK, "note", passStandartContext(c, gin.H{
		"note":     note,
		"markdown": string(content),
	}))
}

func (r *notesRoutes) loadIndex(c *gin.Context) (*joplinNoteIndex, error) {
	file, err := r.storage.Read(c.Request.Context(), joplinIndexPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return &joplinNoteIndex{Items: []joplinNoteMeta{}}, nil
		}
		return nil, err
	}
	defer file.Close()

	var index joplinNoteIndex
	err = json.NewDecoder(file).Decode(&index)
	if err != nil {
		return nil, err
	}
	return &index, nil
}
