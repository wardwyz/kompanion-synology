package v1

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/storage"
	"github.com/vanadium23/kompanion/pkg/logger"
)

const joplinFolderID = "kompanion-folder"
const joplinIndexPath = "joplin/index.json"

type joplinRoutes struct {
	storage storage.Storage
	l       logger.Interface
}

type joplinNote struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

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

func newJoplinRoutes(handler *gin.RouterGroup, st storage.Storage, l logger.Interface) {
	r := &joplinRoutes{
		storage: st,
		l:       l,
	}

	// Joplin clipper compatibility layer (minimal subset used by KOReader exporter).
	handler.GET("/ping", r.ping)
	handler.GET("/folders", r.folders)
	handler.POST("/folders", r.createFolder)
	handler.POST("/notes", r.createNote)
	handler.PUT("/notes/:id", r.updateNote)
}

func (r *joplinRoutes) ping(c *gin.Context) {
	c.String(http.StatusOK, "JoplinClipperServer")
}

func (r *joplinRoutes) folders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"items": []gin.H{
			{"id": joplinFolderID, "title": "Kompanion"},
		},
		"has_more": false,
	})
}

func (r *joplinRoutes) createFolder(c *gin.Context) {
	var request struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = "Kompanion"
	}

	c.JSON(http.StatusOK, gin.H{"id": joplinFolderID, "title": title})
}

func (r *joplinRoutes) createNote(c *gin.Context) {
	var request joplinNote
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if strings.TrimSpace(request.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body is required"})
		return
	}

	noteID, err := newJoplinID()
	if err != nil {
		r.l.Error("joplin createNote generate id: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create note id"})
		return
	}
	request.ID = noteID
	if strings.TrimSpace(request.ParentID) == "" {
		request.ParentID = joplinFolderID
	}

	if err = r.storeNote(c, request); err != nil {
		r.l.Error("joplin createNote store note: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot store note"})
		return
	}

	c.JSON(http.StatusOK, request)
}

func (r *joplinRoutes) updateNote(c *gin.Context) {
	var request joplinNote
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	noteID := strings.TrimSpace(c.Param("id"))
	if noteID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if strings.TrimSpace(request.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body is required"})
		return
	}

	request.ID = noteID
	if strings.TrimSpace(request.ParentID) == "" {
		request.ParentID = joplinFolderID
	}

	if err := r.storeNote(c, request); err != nil {
		r.l.Error("joplin updateNote store note: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot store note"})
		return
	}

	c.JSON(http.StatusOK, request)
}

func (r *joplinRoutes) storeNote(c *gin.Context, note joplinNote) error {
	tempFile, err := os.CreateTemp("", "joplin-note-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err = tempFile.WriteString(note.Body); err != nil {
		return err
	}
	if err = tempFile.Sync(); err != nil {
		return err
	}

	storagePath := noteStoragePath(note.ID)
	if err = r.overwrite(c, tempFile.Name(), storagePath); err != nil {
		return err
	}

	return r.updateNoteIndex(c, note, storagePath)
}

func (r *joplinRoutes) overwrite(c *gin.Context, sourcePath, targetPath string) error {
	err := r.storage.Delete(c, targetPath)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	return r.storage.Write(c, sourcePath, targetPath)
}

func (r *joplinRoutes) updateNoteIndex(c *gin.Context, note joplinNote, storagePath string) error {
	index, err := r.loadIndex(c)
	if err != nil {
		return err
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	newMeta := joplinNoteMeta{
		ID:         note.ID,
		ParentID:   note.ParentID,
		Title:      note.Title,
		StorageKey: storagePath,
		UpdatedAt:  updatedAt,
	}

	indexOfItem := slices.IndexFunc(index.Items, func(item joplinNoteMeta) bool {
		return item.ID == note.ID
	})
	if indexOfItem >= 0 {
		index.Items[indexOfItem] = newMeta
	} else {
		index.Items = append(index.Items, newMeta)
	}

	return r.saveIndex(c, index)
}

func (r *joplinRoutes) loadIndex(c *gin.Context) (*joplinNoteIndex, error) {
	file, err := r.storage.Read(c, joplinIndexPath)
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

func (r *joplinRoutes) saveIndex(c *gin.Context, index *joplinNoteIndex) error {
	tempFile, err := os.CreateTemp("", "joplin-index-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(index); err != nil {
		return err
	}

	if err = tempFile.Sync(); err != nil {
		return err
	}

	return r.overwrite(c, tempFile.Name(), joplinIndexPath)
}

func noteStoragePath(noteID string) string {
	return filepath.ToSlash(fmt.Sprintf("joplin/%s.md", noteID))
}

func newJoplinID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
