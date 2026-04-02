package entity

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var ErrBookAlreadyExists = errors.New("Book already exists")

// Book represents a book entity in the database.
type Book struct {
	ID          string                 // unique identifier for the book
	Title       string                 `form:"title"`        // title of the book
	Author      string                 `form:"author"`       // author of the book
	Description string                 `form:"description"`  // description/summary of the book
	Publisher   string                 `form:"publisher"`    // publisher of the book
	Year        int                    `form:"year"`         // year of publication
	Series      string                 `form:"series"`       // series the book belongs to
	SeriesIndex *decimal.NullDecimal   `form:"series_index"` // position in the series (nullable)
	CreatedAt   time.Time              // timestamp of when the book was created
	UpdatedAt   time.Time              // timestamp of when the book was last updated
	ISBN        string                 // ISBN of the book
	DocumentID  string                 // md5 hash for file content
	FilePath    string                 // path to the book file
	Format      string                 // format of the book file
	CoverPath   string                 // path to the cover image
}

func (b Book) extension() string {
	ext := strings.TrimPrefix(filepath.Ext(strings.TrimSpace(b.FilePath)), ".")
	if ext == "" {
		return "epub"
	}
	return ext
}

func (b Book) Filename() string {
	title := strings.TrimSpace(b.Title)
	if title == "" {
		title = strings.TrimSpace(b.ID)
	}
	if title == "" {
		title = "book"
	}
	return title + "." + b.extension()
}

func (b Book) MimeType() string {
	switch b.extension() {
	case "epub":
		return "application/epub+zip"
	case "pdf":
		return "application/pdf"
	case "mobi":
		return "application/x-mobipocket-ebook"
	case "fb2":
		return "application/fb2"
	default:
		return ""
	}
}
