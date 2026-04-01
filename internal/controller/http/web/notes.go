package web

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/notes"
	"github.com/vanadium23/kompanion/pkg/logger"
)

var markdownBookHeadingPattern = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
var notesDisplayLocation = time.FixedZone("UTC+8", 8*60*60)
var markdownItalicLinePattern = regexp.MustCompile(`^\*(.+)\*$`)
var markdownAuthorPattern = regexp.MustCompile(`(?m)^#####\s+(.+?)\s*$`)
var markdownLocationOrItalicPattern = regexp.MustCompile(`(?s)###\s+([^\n]+)|\*([^*]+)\*`)

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
	BookName         string
	Title            string
	Author           string
	Location         string
	Content          string
	DocumentID       string
	DisplayCreatedAt string
	CreatedAt        time.Time
	BodyRaw          string
}

func newNotesRoutes(handler *gin.RouterGroup, noteSvc notes.Service, l logger.Interface) {
	r := &notesRoutes{notes: noteSvc, logger: l}
	handler.GET("/", r.list)
	handler.GET("/export.md", r.exportMarkdown)
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
	selectedBook := strings.TrimSpace(c.DefaultQuery("book", "all"))
	page := parsePositiveInt(c.Query("page"), 1)
	perPage := 20

	bookOptions := make([]string, 0, len(groups))
	bookOptionSet := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if len(group.Notes) == 0 {
			continue
		}
		bookOptions = append(bookOptions, group.Name)
		bookOptionSet[group.Name] = struct{}{}
	}
	if selectedBook != "all" {
		if _, ok := bookOptionSet[selectedBook]; !ok {
			selectedBook = "all"
		}
	}

	visibleGroups := filterGroupsByBook(groups, selectedBook)
	visibleNotes := flattenNotes(visibleGroups)
	pagedNotes, pagination := paginateReadingNotes(visibleNotes, page, perPage)
	visibleGroups = regroupByBook(pagedNotes)

	c.HTML(http.StatusOK, "notes", passStandartContext(c, gin.H{
		"groups":       visibleGroups,
		"notes":        pagedNotes,
		"bookOptions":  bookOptions,
		"selectedBook": selectedBook,
		"pagination":   pagination,
	}))
}

func (r *notesRoutes) delete(c *gin.Context) {
	if err := r.notes.Delete(c.Request.Context(), c.Param("id")); err != nil {
		r.logger.Error(err, "failed to delete note")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}
	c.Redirect(http.StatusFound, "/notes/")
}

func (r *notesRoutes) exportMarkdown(c *gin.Context) {
	items, err := r.notes.List(c.Request.Context(), 200)
	if err != nil {
		r.logger.Error(err, "failed to list notes for markdown export")
		c.HTML(http.StatusInternalServerError, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}

	groups := r.groupNotesByBook(items)
	filename := fmt.Sprintf("reading-notes-%s.md", time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(notesToMarkdown(groups)))
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
			author, location, content := parseStructuredReadingNote(selected.body)
			groupedNotes = append(groupedNotes, readingNoteView{
				ID:               selected.note.ID,
				BookName:         name,
				Title:            selected.note.Title,
				Author:           author,
				Location:         location,
				Content:          content,
				DocumentID:       selected.note.DocumentID,
				CreatedAt:        selected.note.CreatedAt,
				DisplayCreatedAt: selected.note.CreatedAt.In(notesDisplayLocation).Format("2006-01-02 15:04:05"),
				BodyRaw:          selected.body,
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

func notesToMarkdown(groups []notesBookGroup) string {
	var b strings.Builder
	for gi, group := range groups {
		b.WriteString("# ")
		b.WriteString(group.Name)
		b.WriteString("\n\n")
		for ni, note := range group.Notes {
			if note.Author != "" {
				b.WriteString("## ")
				b.WriteString(group.Name)
				b.WriteString("--")
				b.WriteString(note.Author)
				b.WriteString("\n\n")
			}
			if note.Content != "" {
				b.WriteString(note.Content)
			}
			if note.Location != "" {
				if note.Content != "" {
					b.WriteString("--")
				}
				b.WriteString(note.Location)
			}
			b.WriteString("\n")

			if ni < len(group.Notes)-1 {
				b.WriteString("---\n\n")
			}
		}
		if gi < len(groups)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func parsePositiveInt(raw string, fallback int) int {
	if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && value > 0 {
		return value
	}
	return fallback
}

func flattenNotes(groups []notesBookGroup) []readingNoteView {
	out := make([]readingNoteView, 0)
	for _, group := range groups {
		out = append(out, group.Notes...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func filterNotesByBook(notes []readingNoteView, bookName string) []readingNoteView {
	if bookName == "" || bookName == "all" {
		return notes
	}
	filtered := make([]readingNoteView, 0, len(notes))
	for _, note := range notes {
		if note.BookName == bookName {
			filtered = append(filtered, note)
		}
	}
	return filtered
}

func filterGroupsByBook(groups []notesBookGroup, bookName string) []notesBookGroup {
	if bookName == "" || bookName == "all" {
		return groups
	}
	filtered := make([]notesBookGroup, 0, 1)
	for _, group := range groups {
		if group.Name == bookName {
			filtered = append(filtered, group)
			break
		}
	}
	return filtered
}

func regroupByBook(notes []readingNoteView) []notesBookGroup {
	if len(notes) == 0 {
		return nil
	}
	groupMap := make(map[string][]readingNoteView)
	for _, note := range notes {
		groupMap[note.BookName] = append(groupMap[note.BookName], note)
	}
	groupNames := make([]string, 0, len(groupMap))
	for name := range groupMap {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	out := make([]notesBookGroup, 0, len(groupNames))
	for _, name := range groupNames {
		out = append(out, notesBookGroup{Name: name, Notes: groupMap[name]})
	}
	return out
}

func paginateReadingNotes(notes []readingNoteView, page, perPage int) ([]readingNoteView, gin.H) {
	if perPage <= 0 {
		perPage = 20
	}
	total := len(notes)
	totalPages := 1
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	end := start + perPage
	if end > total {
		end = total
	}

	paged := make([]readingNoteView, 0)
	if total > 0 && start < total {
		paged = notes[start:end]
	}

	return paged, gin.H{
		"currentPage": page,
		"perPage":     perPage,
		"totalPages":  totalPages,
		"hasNext":     page < totalPages,
		"hasPrev":     page > 1,
		"nextPage":    page + 1,
		"prevPage":    page - 1,
	}
}

func markdownToHTML(markdown string) template.HTML {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}

	lines := strings.Split(markdown, "\n")
	var b strings.Builder
	inList := false

	closeList := func() {
		if inList {
			b.WriteString("</ul>")
			inList = false
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			closeList()
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "##### "):
			closeList()
			b.WriteString("<h5>")
			b.WriteString(template.HTMLEscapeString(strings.TrimSpace(strings.TrimPrefix(trimmed, "##### "))))
			b.WriteString("</h5>")
		case strings.HasPrefix(trimmed, "##### "):
			closeList()
			b.WriteString("<h4>")
			b.WriteString(template.HTMLEscapeString(strings.TrimSpace(strings.TrimPrefix(trimmed, "#### "))))
			b.WriteString("</h4>")
		case strings.HasPrefix(trimmed, "### "):
			closeList()
			b.WriteString("<h3>")
			b.WriteString(template.HTMLEscapeString(strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))))
			b.WriteString("</h3>")
		case strings.HasPrefix(trimmed, "## "):
			closeList()
			b.WriteString("<h2>")
			b.WriteString(template.HTMLEscapeString(strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))))
			b.WriteString("</h2>")
		case strings.HasPrefix(trimmed, "# "):
			closeList()
			b.WriteString("<h1>")
			b.WriteString(template.HTMLEscapeString(strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))))
			b.WriteString("</h1>")
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			if !inList {
				b.WriteString("<ul>")
				inList = true
			}
			item := strings.TrimSpace(trimmed[2:])
			b.WriteString("<li>")
			b.WriteString(template.HTMLEscapeString(item))
			b.WriteString("</li>")
		default:
			closeList()
			if m := markdownItalicLinePattern.FindStringSubmatch(trimmed); len(m) == 2 {
				b.WriteString("<p><em>")
				b.WriteString(template.HTMLEscapeString(strings.TrimSpace(m[1])))
				b.WriteString("</em></p>")
				continue
			}
			b.WriteString("<p>")
			b.WriteString(template.HTMLEscapeString(trimmed))
			b.WriteString("</p>")
		}
	}
	closeList()

	return template.HTML(b.String())
}

func parseStructuredReadingNote(markdown string) (author, location, content string) {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")

	if m := markdownAuthorPattern.FindStringSubmatch(normalized); len(m) == 2 {
		author = strings.TrimSpace(m[1])
	}

	type notePart struct {
		text     string
		location string
	}
	parts := make([]notePart, 0)
	currentLocation := ""
	hasLocatedPart := false

	matches := markdownLocationOrItalicPattern.FindAllStringSubmatch(normalized, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		if strings.TrimSpace(m[1]) != "" {
			currentLocation = strings.TrimSpace(m[1])
			location = currentLocation
			continue
		}
		text := strings.TrimSpace(m[2])
		if text == "" {
			continue
		}
		part := notePart{text: text, location: currentLocation}
		if part.location != "" {
			hasLocatedPart = true
		}
		parts = append(parts, part)
	}

	if hasLocatedPart {
		filtered := make([]notePart, 0, len(parts))
		for _, part := range parts {
			if part.location == "" {
				continue
			}
			filtered = append(filtered, part)
		}
		parts = filtered
	}

	if len(parts) == 0 {
		return author, location, ""
	}
	if len(parts) == 1 {
		return author, parts[0].location, parts[0].text
	}

	contentParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.location != "" {
			contentParts = append(contentParts, part.text+"--"+part.location)
			continue
		}
		contentParts = append(contentParts, part.text)
	}
	return author, "", strings.Join(contentParts, "\n")
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
