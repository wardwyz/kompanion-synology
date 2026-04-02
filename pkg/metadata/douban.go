package metadata

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	doubanTitlePattern       = regexp.MustCompile(`<meta\s+property="og:title"\s+content="([^"]+)"`)
	doubanDescriptionPattern = regexp.MustCompile(`<meta\s+property="og:description"\s+content="([^"]+)"`)
	doubanSearchSubject      = regexp.MustCompile(`/subject/(\d+)/`)
	doubanAuthorPattern      = regexp.MustCompile(`作者[^<]*</span>\s*([^<\n]+)`) // from info block
)

const doubanBaseURL = "https://book.douban.com"

var doubanHTTPClient = &http.Client{Timeout: 8 * time.Second}

// AutoScrapeDouban tries to enrich metadata with Douban data.
// If scraping fails or no data is found, it returns original metadata unchanged.
func AutoScrapeDouban(original Metadata) Metadata {
	scraped, err := scrapeDouban(original)
	if err != nil {
		return original
	}
	return mergeMetadata(original, scraped)
}

func scrapeDouban(original Metadata) (Metadata, error) {
	title := strings.TrimSpace(original.Title)
	isbn := strings.TrimSpace(original.ISBN)

	if title != "" {
		scraped, err := scrapeDoubanByKeyword(strings.TrimSpace(title + " " + original.Author))
		if err == nil {
			return scraped, nil
		}
		if isbn != "" {
			return scrapeDoubanByISBN(isbn)
		}
		return Metadata{}, err
	}

	if isbn != "" {
		return scrapeDoubanByISBN(isbn)
	}

	return Metadata{}, fmt.Errorf("insufficient input for douban scraping")
}

func scrapeDoubanByISBN(isbn string) (Metadata, error) {
	u := fmt.Sprintf("%s/isbn/%s/", doubanBaseURL, url.PathEscape(normalizeISBN(isbn)))
	body, err := fetchHTTPBody(u)
	if err != nil {
		return Metadata{}, err
	}
	return parseDoubanBookPage(body), nil
}

func scrapeDoubanByKeyword(keyword string) (Metadata, error) {
	searchURL := fmt.Sprintf("%s/subject_search?search_text=%s&cat=1001", doubanBaseURL, url.QueryEscape(keyword))
	body, err := fetchHTTPBody(searchURL)
	if err != nil {
		return Metadata{}, err
	}

	match := doubanSearchSubject.FindStringSubmatch(body)
	if len(match) < 2 {
		return Metadata{}, fmt.Errorf("douban subject not found")
	}

	bookURL := fmt.Sprintf("%s/subject/%s/", doubanBaseURL, match[1])
	bookBody, err := fetchHTTPBody(bookURL)
	if err != nil {
		return Metadata{}, err
	}
	return parseDoubanBookPage(bookBody), nil
}

func fetchHTTPBody(rawURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := doubanHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("douban request failed with status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func parseDoubanBookPage(body string) Metadata {
	m := Metadata{}

	if title := firstSubmatch(doubanTitlePattern, body); title != "" {
		m.Title = htmlUnescape(title)
	}
	if description := firstSubmatch(doubanDescriptionPattern, body); description != "" {
		m.Description = htmlUnescape(description)
	}
	if author := firstSubmatch(doubanAuthorPattern, body); author != "" {
		m.Author = strings.TrimSpace(htmlUnescape(author))
	}

	return m
}

func firstSubmatch(pattern *regexp.Regexp, content string) string {
	match := pattern.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func normalizeISBN(isbn string) string {
	replacer := strings.NewReplacer("-", "", " ", "")
	return replacer.Replace(strings.TrimSpace(isbn))
}

func htmlUnescape(v string) string {
	v = strings.ReplaceAll(v, "&quot;", `"`)
	v = strings.ReplaceAll(v, "&amp;", "&")
	v = strings.ReplaceAll(v, "&#39;", "'")
	v = strings.ReplaceAll(v, "&lt;", "<")
	v = strings.ReplaceAll(v, "&gt;", ">")
	return v
}
