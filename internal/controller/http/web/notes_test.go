package web

import (
	"strings"
	"testing"
	"time"

	"github.com/vanadium23/kompanion/internal/entity"
)

func TestBookNameFromMarkdown(t *testing.T) {
	note := entity.ReadingNote{Body: "# 三国演义\n\n- 摘录 1\n- 摘录 2"}

	book, body := bookNameFromMarkdown(note)
	if book != "三国演义" {
		t.Fatalf("expected book name, got %q", book)
	}
	if body == note.Body {
		t.Fatalf("expected markdown heading to be removed")
	}
}

func TestBookNameFromMarkdownNoHeading(t *testing.T) {
	note := entity.ReadingNote{Body: "- only content"}

	book, body := bookNameFromMarkdown(note)
	if book != "" {
		t.Fatalf("expected empty book name, got %q", book)
	}
	if body != note.Body {
		t.Fatalf("expected original body to remain unchanged")
	}
}

func TestGroupNotesByBook_DeduplicateSameBodyAndFixTimezone(t *testing.T) {
	r := &notesRoutes{}
	createdAt := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)

	groups := r.groupNotesByBook([]entity.ReadingNote{
		{ID: "1", Title: "A", Body: "# 书A\n\n重复内容", CreatedAt: createdAt},
		{ID: "2", Title: "A2", Body: "# 书A\n\n重复内容", CreatedAt: createdAt.Add(1 * time.Minute)},
	})

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Notes) != 1 {
		t.Fatalf("expected duplicated notes to be removed, got %d", len(groups[0].Notes))
	}
	if groups[0].Notes[0].DisplayCreatedAt != "2026-03-30 08:01:00" {
		t.Fatalf("expected UTC+8 display time, got %s", groups[0].Notes[0].DisplayCreatedAt)
	}
}

func TestMarkdownToHTML(t *testing.T) {
	markdown := "# 书名\n#### 作者\n\n- 摘录 1\n- 摘录 2\n\n正文内容"
	html := string(markdownToHTML(markdown))

	if !strings.Contains(html, "<h3>书名</h3>") {
		t.Fatalf("expected h3 heading, got %s", html)
	}
	if !strings.Contains(html, "<h4>作者</h4>") {
		t.Fatalf("expected h4 author heading, got %s", html)
	}
	if !strings.Contains(html, "<li>摘录 1</li>") || !strings.Contains(html, "<li>摘录 2</li>") {
		t.Fatalf("expected list items, got %s", html)
	}
	if !strings.Contains(html, "<p>正文内容</p>") {
		t.Fatalf("expected body paragraph, got %s", html)
	}
}
