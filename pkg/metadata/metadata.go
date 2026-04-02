package metadata

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Metadata struct {
	ISBN        string
	Title       string
	Description string
	Author      string
	Date        string
	Publisher   string
	Language    string
	Format      string
	Series      string
	SeriesIndex string
	Cover       []byte
}

// ApplyDefaultsAndAutoScrape applies local defaults first, then tries Douban scraping.
// If scraping fails, defaults remain unchanged.
func ApplyDefaultsAndAutoScrape(m Metadata, uploadedFilename string) Metadata {
	defaults := applyDefaults(m, uploadedFilename)
	if !doubanAutoScrapeEnabled() {
		return defaults
	}
	return AutoScrapeDouban(defaults)
}

func doubanAutoScrapeEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KOMPANION_DOUBAN_AUTO_SCRAPE")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// ExtractBookMetadata extracts metadata from a book file
func ExtractBookMetadata(tempFile *os.File) (Metadata, error) {
	extension, err := guessExtention(tempFile)
	if err != nil {
		return Metadata{}, err
	}
	var m Metadata
	switch extension {
	case "pdf":
		m, err = extractPdfMetadata(tempFile)
		if err != nil {
			return Metadata{}, err
		}
	case "epub":
		m, err = getEpubMetadata(tempFile)
		if err != nil {
			return Metadata{}, err
		}
	case "fb2":
		m, err = getFb2Metatada(tempFile)
		if err != nil {
			return Metadata{}, err
		}
	}
	m.Format = extension
	return m, nil
}

func guessExtention(file *os.File) (string, error) {
	// TODO: move extensions to enum
	data := make([]byte, 100*1024)
	_, err := file.ReadAt(data, 0)
	if err != nil && err != io.EOF {
		return "", err
	}
	mimeType := http.DetectContentType(data)
	fmt.Println(mimeType)
	switch mimeType {
	case "application/pdf":
		return "pdf", nil
	case "application/epub+zip":
		return "epub", nil
	case "application/zip":
		return "epub", nil
	case "application/x-fictionbook+xml":
		return "fb2", nil
	case "text/xml; charset=utf-8":
		return "fb2", nil
	default:
		return "", nil
	}
}

func applyDefaults(m Metadata, uploadedFilename string) Metadata {
	parsedTitle, parsedAuthor := parseTitleAuthorFromFilename(uploadedFilename)

	if strings.TrimSpace(m.Title) == "" {
		if parsedTitle == "" {
			m.Title = "Unknown Title"
		} else {
			m.Title = parsedTitle
		}
	}
	if strings.TrimSpace(m.Author) == "" {
		if parsedAuthor != "" {
			m.Author = parsedAuthor
		} else {
			m.Author = "Unknown Author"
		}
	}
	if strings.TrimSpace(m.Description) == "" {
		m.Description = "No description available"
	}
	return m
}

func parseTitleAuthorFromFilename(uploadedFilename string) (string, string) {
	base := strings.TrimSpace(strings.TrimSuffix(filepath.Base(uploadedFilename), filepath.Ext(uploadedFilename)))
	if base == "" {
		return "", ""
	}

	for _, delimiter := range []string{" -- ", " —— ", " - ", " — ", " + ", " ＋ "} {
		parts := strings.SplitN(base, delimiter, 2)
		if len(parts) != 2 {
			continue
		}
		title := strings.TrimSpace(parts[0])
		author := strings.TrimSpace(parts[1])
		if title != "" {
			return title, author
		}
	}

	// Some libraries use compact naming like "书名+作者.epub" without spaces.
	for _, delimiter := range []string{"+", "＋"} {
		parts := strings.SplitN(base, delimiter, 2)
		if len(parts) != 2 {
			continue
		}
		title := strings.TrimSpace(parts[0])
		author := strings.TrimSpace(parts[1])
		if title != "" && author != "" {
			return title, author
		}
	}

	return base, ""
}

func mergeMetadata(base Metadata, override Metadata) Metadata {
	if strings.TrimSpace(override.ISBN) != "" {
		base.ISBN = strings.TrimSpace(override.ISBN)
	}
	if strings.TrimSpace(override.Title) != "" {
		base.Title = strings.TrimSpace(override.Title)
	}
	if strings.TrimSpace(override.Description) != "" {
		base.Description = strings.TrimSpace(override.Description)
	}
	if strings.TrimSpace(override.Author) != "" {
		base.Author = strings.TrimSpace(override.Author)
	}
	if strings.TrimSpace(override.Date) != "" {
		base.Date = strings.TrimSpace(override.Date)
	}
	if strings.TrimSpace(override.Publisher) != "" {
		base.Publisher = strings.TrimSpace(override.Publisher)
	}
	if strings.TrimSpace(override.Language) != "" {
		base.Language = strings.TrimSpace(override.Language)
	}
	if strings.TrimSpace(override.Series) != "" {
		base.Series = strings.TrimSpace(override.Series)
	}
	if strings.TrimSpace(override.SeriesIndex) != "" {
		base.SeriesIndex = strings.TrimSpace(override.SeriesIndex)
	}
	if len(override.Cover) > 0 {
		base.Cover = override.Cover
	}
	return base
}
