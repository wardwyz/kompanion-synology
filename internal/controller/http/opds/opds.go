package opds

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/library"
)

const (
	AtomTime = "2006-01-02T15:04:05Z"
	DirMime  = "application/atom+xml;profile=opds-catalog;kind=navigation"
	DirRel   = "subsection"
	FileRel  = "http://opds-spec.org/acquisition"
	CoverRel = "http://opds-spec.org/image"
	ThumbRel = "http://opds-spec.org/image/thumbnail"
)

// Feed is a main frame of OPDS.
type Feed struct {
	XMLName xml.Name `xml:"feed"`
	ID      string   `xml:"id"`
	Title   string   `xml:"title"`
	Xmlns   string   `xml:"xmlns,attr"`
	Updated string   `xml:"updated"`
	Link    []Link   `xml:"link"`
	Entry   []Entry  `xml:"entry"`
}

// Link is link properties.
type Link struct {
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
	Rel  string `xml:"rel,attr,ommitempty"`
}

// Entry is a struct of OPDS entry properties.
type Entry struct {
	ID       string     `xml:"id"`
	Updated  string     `xml:"updated"`
	Title    string     `xml:"title"`
	Author   Author     `xml:"author,ommitempty"`
	Summary  Summary    `xml:"summary,ommitempty"`
	Category []Category `xml:"category,omitempty"`
	Link     []Link     `xml:"link"`
}

type Author struct {
	Name string `xml:"name"`
}

type Summary struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type Category struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr,omitempty"`
}

func BuildFeed(id, title, href string, entries []Entry, additionalLinks []Link) *Feed {
	finalLinks := []Link{
		{
			Href: "/opds/",
			Type: DirMime,
			Rel:  "start",
		},
		{
			Href: href,
			Type: DirMime,
			Rel:  "self",
		},
		{
			Href: "/opds/search/{searchTerms}/",
			Type: "application/atom+xml",
			Rel:  "search",
		},
	}
	finalLinks = append(finalLinks, additionalLinks...)
	return &Feed{
		ID:      id,
		Title:   title,
		Xmlns:   "http://www.w3.org/2005/Atom",
		Updated: time.Now().UTC().Format(AtomTime),
		Link:    finalLinks,
		Entry:   entries,
	}
}

func translateBooksToEntries(books []entity.Book) []Entry {
	entries := make([]Entry, 0, len(books))
	for _, book := range books {
		entries = append(entries, Entry{
			ID:      book.ID,
			Updated: book.UpdatedAt.Format(AtomTime),
			Title:   book.Title,
			Author: Author{
				Name: book.Author,
			},
			Summary: Summary{
				Type: "text",
				Text: truncateText(book.Description, 300),
			},
			Category: buildBookCategories(book),
			Link: []Link{
				{
					Href: fmt.Sprintf("/opds/book/%s/download", book.ID),
					Type: book.MimeType(),
					Rel:  FileRel,
				},
				{
					Href: fmt.Sprintf("/opds/book/%s/cover", book.ID),
					Type: "image/jpeg",
					Rel:  CoverRel,
				},
				{
					Href: fmt.Sprintf("/opds/book/%s/cover", book.ID),
					Type: "image/jpeg",
					Rel:  ThumbRel,
				},
			},
		})
	}
	return entries
}

func buildBookCategories(book entity.Book) []Category {
	categories := make([]Category, 0, 4)
	if book.Series != "" {
		categories = append(categories, Category{Term: book.Series, Label: "系列"})
	}
	if book.Publisher != "" {
		categories = append(categories, Category{Term: book.Publisher, Label: "出版社"})
	}
	if book.Format != "" {
		categories = append(categories, Category{Term: book.Format, Label: "格式"})
	}
	if book.Year > 0 {
		categories = append(categories, Category{Term: fmt.Sprintf("%d", book.Year), Label: "出版年份"})
	}
	return categories
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func formNavLinks(baseURL string, books library.PaginatedBookList) []Link {
	links := []Link{
		{
			Href: baseURL,
			Type: DirMime,
			Rel:  "start",
		},
		{
			Href: fmt.Sprintf("%s?page=%d", baseURL, books.Last()),
			Type: DirMime,
			Rel:  "last",
		},
	}
	if books.HasNext() {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?page=%d", baseURL, books.Next()),
			Type: DirMime,
			Rel:  "next",
		})
	}
	if books.HasPrev() {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?page=%d", baseURL, books.Prev()),
			Type: DirMime,
			Rel:  "prev",
		})
	}
	return links
}
