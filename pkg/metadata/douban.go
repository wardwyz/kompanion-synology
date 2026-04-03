package metadata

import (
	"fmt"
	"html"
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
	doubanRatingPattern      = regexp.MustCompile(`<strong[^>]*class="[^"]*\brating_num\b[^"]*"[^>]*>([^<]+)</strong>`)
	doubanSearchSubject      = regexp.MustCompile(`/subject/(\d+)/`)
	doubanInfoBlockPattern   = regexp.MustCompile(`(?s)<div[^>]+id="info"[^>]*>(.*?)</div>`)
	doubanTagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	doubanSpacePattern       = regexp.MustCompile(`\s+`)
	doubanDigitsPattern      = regexp.MustCompile(`\d{4}`)
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
	return mergeScrapedMetadata(original, scraped)
}

func mergeScrapedMetadata(original Metadata, scraped Metadata) Metadata {
	// Prefer embedded/local title by default, but allow scraped title to replace
	// it when scraped title is clearly a cleaner base title (e.g. original title
	// has marketing suffixes in trailing parentheses).
	if strings.TrimSpace(original.Title) != "" &&
		!shouldPreferScrapedTitle(original.Title, scraped.Title) {
		scraped.Title = ""
	}
	return mergeMetadata(original, scraped)
}

func shouldPreferScrapedTitle(originalTitle, scrapedTitle string) bool {
	originalTitle = strings.TrimSpace(originalTitle)
	scrapedTitle = strings.TrimSpace(scrapedTitle)

	if originalTitle == "" || scrapedTitle == "" {
		return false
	}
	if originalTitle == scrapedTitle {
		return false
	}

	// Common case:
	// 原始: 刀锋(“故事高手”毛姆晚年重要作品...)(果麦经典)
	// 刮削: 刀锋
	if strings.HasPrefix(originalTitle, scrapedTitle) {
		return true
	}

	for _, delimiter := range []string{"(", "（"} {
		base := strings.TrimSpace(strings.SplitN(originalTitle, delimiter, 2)[0])
		if base != "" && base == scrapedTitle {
			return true
		}
	}

	return false
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
	if rating := firstSubmatch(doubanRatingPattern, body); rating != "" {
		m.SeriesIndex = normalizeNumericField(rating)
	}
	info := firstSubmatch(doubanInfoBlockPattern, body)
	if author := extractDoubanInfoField(info, "作者"); author != "" {
		m.Author = author
	}
	if series := extractDoubanInfoField(info, "丛书"); series != "" {
		m.Series = series
	}
	if publisher := extractDoubanInfoField(info, "出版社"); publisher != "" {
		m.Publisher = publisher
	}
	if isbn := extractDoubanInfoField(info, "ISBN"); isbn != "" {
		m.ISBN = normalizeISBN(isbn)
	}
	if publishedDate := extractDoubanInfoField(info, "出版年"); publishedDate != "" {
		m.Date = normalizePublishedYear(publishedDate)
	}

	return m
}

func extractDoubanInfoField(infoBlock string, label string) string {
	if strings.TrimSpace(infoBlock) == "" {
		return ""
	}

	pattern := regexp.MustCompile(fmt.Sprintf(`(?is)<span[^>]*>\s*%s\s*[:：]?\s*</span>\s*(.*?)\s*(?:<br\s*/?>|$)`, regexp.QuoteMeta(label)))
	raw := firstSubmatch(pattern, infoBlock)
	if raw == "" {
		return ""
	}

	cleaned := doubanTagPattern.ReplaceAllString(raw, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = doubanSpacePattern.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, ":"))
	return cleaned
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

func normalizePublishedYear(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if year := doubanDigitsPattern.FindString(raw); year != "" {
		return year
	}
	return raw
}

func normalizeNumericField(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimSuffix(raw, ".0")
	return raw
}
