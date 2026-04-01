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

func TestParseStructuredReadingNote(t *testing.T) {
	markdown := "##### 毛泽东\n\n### Page 75 @ 24 March 2026 12:01:32 PM\n\n*造反派是毛泽东的左手，冲击官僚体制需要他们；官僚集团是毛泽东的右手，恢复秩序需要他们。*"
	author, location, content := parseStructuredReadingNote(markdown)

	if author != "毛泽东" {
		t.Fatalf("expected author, got %q", author)
	}
	if location != "第七十五页 - 2026年3月24日 12:01:32" {
		t.Fatalf("expected location, got %q", location)
	}
	if content != "造反派是毛泽东的左手，冲击官僚体制需要他们；官僚集团是毛泽东的右手，恢复秩序需要他们。" {
		t.Fatalf("expected note content, got %q", content)
	}
}

func TestLocalizeReadingLocation(t *testing.T) {
	got := localizeReadingLocation("Page 4 @ 30 March 2026 11:05:50 AM")
	want := "第四页 - 2026年3月30日 11:05:50"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLocalizeReadingLocation_PageOnly(t *testing.T) {
	got := localizeReadingLocation("Page 4")
	want := "第四页"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestNotesToMarkdown(t *testing.T) {
	out := notesToMarkdown([]notesBookGroup{
		{
			Name: "三国演义",
			Notes: []readingNoteView{
				{
					Author:   "罗贯中",
					Location: "Page 1",
					Content:  "桃园结义",
				},
			},
		},
	})

	if !strings.Contains(out, "# 三国演义") {
		t.Fatalf("expected book heading, got %s", out)
	}
	if !strings.Contains(out, "## 三国演义--罗贯中") {
		t.Fatalf("expected book and author heading, got %s", out)
	}
	if !strings.Contains(out, "桃园结义--第一页") {
		t.Fatalf("expected body and location format, got %s", out)
	}
}

func TestFilterAndPaginateReadingNotes(t *testing.T) {
	all := []readingNoteView{
		{ID: "1", BookName: "A", CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "2", BookName: "B", CreatedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)},
		{ID: "3", BookName: "A", CreatedAt: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)},
	}

	filtered := filterNotesByBook(all, "A")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 notes for book A, got %d", len(filtered))
	}

	paged, pagination := paginateReadingNotes(filtered, 1, 1)
	if len(paged) != 1 {
		t.Fatalf("expected 1 note in paged output, got %d", len(paged))
	}
	if pagination["totalPages"] != 2 {
		t.Fatalf("expected totalPages=2, got %v", pagination["totalPages"])
	}
}

func TestFilterGroupsByBook(t *testing.T) {
	groups := []notesBookGroup{
		{Name: "A", Notes: []readingNoteView{{ID: "1"}}},
		{Name: "B", Notes: []readingNoteView{{ID: "2"}}},
	}

	filtered := filterGroupsByBook(groups, "B")
	if len(filtered) != 1 || filtered[0].Name != "B" {
		t.Fatalf("expected only group B, got %+v", filtered)
	}
}

func TestRegroupByBook(t *testing.T) {
	notes := []readingNoteView{
		{ID: "2", BookName: "B"},
		{ID: "1", BookName: "A"},
		{ID: "3", BookName: "B"},
	}
	grouped := regroupByBook(notes)
	if len(grouped) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(grouped))
	}
	if grouped[0].Name != "A" || grouped[1].Name != "B" {
		t.Fatalf("expected sorted group names A,B got %s,%s", grouped[0].Name, grouped[1].Name)
	}
	if len(grouped[1].Notes) != 2 {
		t.Fatalf("expected 2 notes in group B, got %d", len(grouped[1].Notes))
	}
}
