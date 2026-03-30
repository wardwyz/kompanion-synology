package web

import (
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/notes"
	"github.com/vanadium23/kompanion/pkg/logger"
)

var markdownBookHeadingPattern = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
var notesDisplayLocation = time.FixedZone("UTC+8", 8*60*60)

type notesRoutes struct {
	notes  notes.Service
	logger logger.Interface
}

type notesBookGroup struct {
	Name  string
	Notes []readingNoteView
}

type readingNoteView struct {
	ID               string
	Title            string
	DocumentID       string
	DisplayCreatedAt string
	CreatedAt        time.Time
	BodyMarkdown     template.HTML
}

func newNotesRoutes(handler *gin.RouterGroup, noteSvc notes.Service, l logger.Interface) {
	r := &notesRoutes{notes: noteSvc, logger: l}
	handler.GET("/", r.list)
	handler.POST("/:id/delete", r.delete)
}

func (r *notesRoutes) list(c *gin.Context) {
	items, err := r.notes.List(c.Request.Context(), 200)
	if err != nil {
		r.logger.Error(err, "failed to list notes")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}

	groups := r.groupNotesByBook(items)
	c.HTML(http.StatusOK, "notes", passStandartContext(c, gin.H{"groups": groups}))
}

func (r *notesRoutes) delete(c *gin.Context) {
	if err := r.notes.Delete(c.Request.Context(), c.Param("id")); err != nil {
		r.logger.Error(err, "failed to delete note")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}
	c.Redirect(http.StatusFound, "/notes/")
}

func (r *notesRoutes) groupNotesByBook(items []entity.ReadingNote) []notesBookGroup {
	type groupedNote struct {
		note entity.ReadingNote
		body string
	}

	groupedUnique := make(map[string]map[string]groupedNote)
	for _, item := range items {
		bookName, markdownBody := bookNameFromMarkdown(item)
		if bookName == "" {
			bookName = "未分类"
		}
		normalizedBody := normalizeNoteBody(markdownBody)
		if _, ok := groupedUnique[bookName]; !ok {
			groupedUnique[bookName] = make(map[string]groupedNote)
		}
		existing, duplicated := groupedUnique[bookName][normalizedBody]
		if !duplicated || item.CreatedAt.After(existing.note.CreatedAt) {
			groupedUnique[bookName][normalizedBody] = groupedNote{note: item, body: markdownBody}
		}
	}

	out := make([]notesBookGroup, 0, len(groupedUnique))
	for name, deduped := range groupedUnique {
		groupedNotes := make([]readingNoteView, 0, len(deduped))
		for _, selected := range deduped {
			groupedNotes = append(groupedNotes, readingNoteView{
				ID:               selected.note.ID,
				Title:            selected.note.Title,
				DocumentID:       selected.note.DocumentID,
				CreatedAt:        selected.note.CreatedAt,
				DisplayCreatedAt: selected.note.CreatedAt.In(notesDisplayLocation).Format("2006-01-02 15:04:05"),
				BodyMarkdown:     template.HTML(template.HTMLEscapeString(selected.body)),
			})
		}
		sort.Slice(groupedNotes, func(i, j int) bool { return groupedNotes[i].CreatedAt.After(groupedNotes[j].CreatedAt) })
		out = append(out, notesBookGroup{Name: name, Notes: groupedNotes})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func normalizeNoteBody(body string) string {
	return strings.TrimSpace(body)
}

func bookNameFromMarkdown(note entity.ReadingNote) (string, string) {
	body := strings.TrimSpace(note.Body)
	if body == "" {
		return "", note.Body
	}
	match := markdownBookHeadingPattern.FindStringSubmatchIndex(body)
	if len(match) != 4 {
		return "", note.Body
	}
	bookName := strings.TrimSpace(body[match[2]:match[3]])
	bodyWithoutHeading := strings.TrimSpace(body[:match[0]] + body[match[1]:])
	return bookName, bodyWithoutHeading
}
