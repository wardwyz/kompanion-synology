package web

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanadium23/kompanion/internal/entity"
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
		"notes": []entity.ReadingNote{{
			Title:      "Example",
			Body:       "# hello",
			DocumentID: "doc1",
			CreatedAt:  time.Now(),
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
