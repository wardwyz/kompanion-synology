package opds

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/vanadium23/kompanion/internal/entity"
)

func TestTranslateBooksToEntries_MetadataCompletion(t *testing.T) {
	idx := decimal.NewNullDecimal(decimal.RequireFromString("2.5"))
	books := []entity.Book{{
		ID:          "book-1",
		Title:       "三体",
		Author:      "刘慈欣",
		Description: "科幻经典",
		Publisher:   "重庆出版社",
		Year:        2008,
		Series:      "地球往事",
		SeriesIndex: &idx,
		ISBN:        "9787536692930",
		Format:      "epub",
		FilePath:    "book-1.epub",
		UpdatedAt:   time.Date(2026, 4, 2, 3, 4, 5, 0, time.UTC),
	}}

	entries := translateBooksToEntries(books)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "三体" {
		t.Fatalf("expected title to be book title, got %q", entry.Title)
	}
	if entry.Summary.Text != "科幻经典" {
		t.Fatalf("expected summary to use description, got %q", entry.Summary.Text)
	}
	if entry.DCIdentifier != "9787536692930" {
		t.Fatalf("unexpected dc:identifier: %q", entry.DCIdentifier)
	}
	if entry.DCSeries != "地球往事" {
		t.Fatalf("unexpected dcterms:series: %q", entry.DCSeries)
	}

	hasSeriesCategory := false
	hasKeywordCategory := false
	for _, cat := range entry.Category {
		if cat.Label == "丛书" && cat.Term == "地球往事 #2.5" {
			hasSeriesCategory = true
		}
		if cat.Label == "关键字" && cat.Term == "三体" {
			hasKeywordCategory = true
		}
	}
	if !hasSeriesCategory {
		t.Fatalf("expected series category with index")
	}
	if !hasKeywordCategory {
		t.Fatalf("expected keyword category")
	}
}

func TestTranslateBooksToEntries_DefaultTitleAndDescription(t *testing.T) {
	books := []entity.Book{{
		ID:        "book-2",
		Author:    "佚名",
		FilePath:  "book-2.epub",
		UpdatedAt: time.Date(2026, 4, 2, 3, 4, 5, 0, time.UTC),
	}}

	entries := translateBooksToEntries(books)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "book-2" {
		t.Fatalf("expected fallback title from filename, got %q", entry.Title)
	}
	if entry.Summary.Text != "暂无简介" {
		t.Fatalf("expected default summary, got %q", entry.Summary.Text)
	}
}
