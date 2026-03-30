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
	markdown := "## 导论：道路·理论·制度\n\n### Page 75 @ 24 March 2026 12:01:32 PM\n\n*造反派是毛泽东的左手，冲击官僚体制需要他们；官僚集团是毛泽东的右手，恢复秩序需要他们。*"
	html := string(markdownToHTML(markdown))

	if !strings.Contains(html, "<h2>导论：道路·理论·制度</h2>") {
		t.Fatalf("expected h2 heading, got %s", html)
	}
	if !strings.Contains(html, "<h3>Page 75 @ 24 March 2026 12:01:32 PM</h3>") {
		t.Fatalf("expected h3 heading, got %s", html)
	}
	if !strings.Contains(html, "<em>造反派是毛泽东的左手") {
		t.Fatalf("expected italic sentence, got %s", html)
	}
}
