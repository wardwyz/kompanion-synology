package web

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotesTemplateRender(t *testing.T) {
	master := filepath.Join("../../../../web/templates/layouts/master.html")
	notes := filepath.Join("../../../../web/templates/notes.html")
	tmpl, err := template.New("master").Funcs(template.FuncMap{
		"Version":   func() string { return "test" },
		"LoadTimes": func(_ time.Time) string { return "0ms" },
	}).ParseFiles(master, notes)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	data := map[string]interface{}{
		"isAuthenticated": true,
		"startTime":       time.Now(),
		"selectedBook":    "all",
		"bookOptions":     []string{"Book A"},
		"notes": []readingNoteView{{
			BookName:         "Book A",
			Title:            "Example",
			DocumentID:       "doc1",
			CreatedAt:        time.Now(),
			DisplayCreatedAt: "2026-03-30 08:00:00",
			BodyMarkdown:     "# hello",
		}},
		"pagination": map[string]interface{}{
			"currentPage": 1,
			"totalPages":  1,
			"hasPrev":     false,
			"hasNext":     false,
		},
		"groups": []notesBookGroup{{
			Name: "Book A",
			Notes: []readingNoteView{{
				Title:            "Example",
				DocumentID:       "doc1",
				CreatedAt:        time.Now(),
				DisplayCreatedAt: "2026-03-30 08:00:00",
				BodyMarkdown:     "# hello",
			}},
		}},
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "master.html", data); err != nil {
		t.Fatalf("execute template: %v", err)
	}

	html := out.String()
	if !strings.Contains(html, "阅读笔记") || !strings.Contains(html, "Example") {
		t.Fatalf("rendered html missing expected content: %s", html)
	}
}
