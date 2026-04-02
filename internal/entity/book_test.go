package entity

import "testing"

func TestBookFilename_UsesOriginalUploadedNameFromPath(t *testing.T) {
	book := Book{
		ID:       "book-1",
		Title:    "三体",
		Author:   "刘慈欣",
		FilePath: "/data/library/2026/04/02/book-1__刀锋.epub",
	}

	if got := book.Filename(); got != "刀锋.epub" {
		t.Fatalf("expected filename to use original upload name, got %q", got)
	}
}

func TestBookFilename_FallbackToTitleWhenOriginalNameMissing(t *testing.T) {
	book := Book{
		ID:       "book-1",
		Title:    "三体",
		Author:   "刘慈欣",
		FilePath: "/data/library/book-1.epub",
	}

	if got := book.Filename(); got != "三体.epub" {
		t.Fatalf("expected filename to fallback to title, got %q", got)
	}
}

func TestBookFilename_FallbackToID(t *testing.T) {
	book := Book{
		ID:       "book-2",
		FilePath: "/data/library/book-2.fb2",
	}

	if got := book.Filename(); got != "book-2.fb2" {
		t.Fatalf("expected filename to fallback to id, got %q", got)
	}
}

func TestBookFilename_DefaultFallbackWhenMissingAll(t *testing.T) {
	book := Book{}
	if got := book.Filename(); got != "book.epub" {
		t.Fatalf("expected default fallback filename, got %q", got)
	}
}
