package opds

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
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
	XMLName     xml.Name `xml:"feed"`
	ID          string   `xml:"id"`
	Title       string   `xml:"title"`
	Xmlns       string   `xml:"xmlns,attr"`
	XmlnsDC     string   `xml:"xmlns:dc,attr,omitempty"`
	XmlnsDCTerm string   `xml:"xmlns:dcterms,attr,omitempty"`
	Updated     string   `xml:"updated"`
	Link        []Link   `xml:"link"`
	Entry       []Entry  `xml:"entry"`
}

// Link is link properties.
type Link struct {
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
	Rel  string `xml:"rel,attr,ommitempty"`
}

// Entry is a struct of OPDS entry properties.
type Entry struct {
	ID           string     `xml:"id"`
	Updated      string     `xml:"updated"`
	Title        string     `xml:"title"`
	Author       Author     `xml:"author,ommitempty"`
	Summary      Summary    `xml:"summary,ommitempty"`
	Category     []Category `xml:"category,omitempty"`
	DCIdentifier string     `xml:"dc:identifier,omitempty"`
	DCPublisher  string     `xml:"dc:publisher,omitempty"`
	DCDate       string     `xml:"dc:date,omitempty"`
	DCSeries     string     `xml:"dcterms:series,omitempty"`
	Link         []Link     `xml:"link"`
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
		ID:          id,
		Title:       title,
		Xmlns:       "http://www.w3.org/2005/Atom",
		XmlnsDC:     "http://purl.org/dc/elements/1.1/",
		XmlnsDCTerm: "http://purl.org/dc/terms/",
		Updated:     time.Now().UTC().Format(AtomTime),
		Link:        finalLinks,
		Entry:       entries,
	}
}

func translateBooksToEntries(books []entity.Book) []Entry {
	entries := make([]Entry, 0, len(books))
	for _, book := range books {
		entries = append(entries, Entry{
			ID:      book.ID,
			Updated: book.UpdatedAt.Format(AtomTime),
			Title:   normalizeBookTitle(book),
			Author: Author{
				Name: book.Author,
			},
			Summary: Summary{
				Type: "text",
				Text: truncateText(normalizeBookDescription(book.Description), 300),
			},
			Category:     buildBookCategories(book),
			DCIdentifier: strings.TrimSpace(book.ISBN),
			DCPublisher:  strings.TrimSpace(book.Publisher),
			DCDate:       formatPublicationYear(book.Year),
			DCSeries:     strings.TrimSpace(book.Series),
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
	categories := make([]Category, 0, 8)
	if book.Series != "" {
		seriesTerm := book.Series
		if book.SeriesIndex != nil && book.SeriesIndex.Valid {
			seriesTerm = fmt.Sprintf("%s #%s", seriesTerm, book.SeriesIndex.Decimal.String())
		}
		categories = append(categories, Category{Term: seriesTerm, Label: "丛书"})
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
	if book.ISBN != "" {
		categories = append(categories, Category{Term: book.ISBN, Label: "ISBN"})
	}
	for _, keyword := range buildKeywords(book) {
		categories = append(categories, Category{Term: keyword, Label: "关键字"})
	}
	return categories
}

func normalizeBookTitle(book entity.Book) string {
	title := strings.TrimSpace(book.Title)
	if title != "" {
		return title
	}
	filename := strings.TrimSpace(book.FilePath)
	if filename == "" {
		return strings.TrimSpace(book.ID)
	}
	base := strings.TrimSpace(filepath.Base(filename))
	ext := filepath.Ext(base)
	return strings.TrimSpace(strings.TrimSuffix(base, ext))
}

func normalizeBookDescription(description string) string {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return "暂无简介"
	}
	return trimmed
}

func formatPublicationYear(year int) string {
	if year <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", year)
}

func buildKeywords(book entity.Book) []string {
	candidates := []string{
		strings.TrimSpace(book.Title),
		strings.TrimSpace(book.Author),
		strings.TrimSpace(book.Series),
		strings.TrimSpace(book.Publisher),
		strings.TrimSpace(book.Format),
	}
	keywords := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		keywords = append(keywords, candidate)
	}
	return keywords
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
