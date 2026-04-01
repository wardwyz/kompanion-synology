package opds

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/auth"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/library"
	"github.com/vanadium23/kompanion/internal/sync"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type OPDSRouter struct {
	books  library.Shelf
	logger logger.Interface
}

func NewRouter(
	handler *gin.Engine,
	l logger.Interface,
	a auth.AuthInterface,
	p sync.Progress,
	shelf library.Shelf) {
	sh := &OPDSRouter{shelf, l}

	h := handler.Group("/opds")
	h.Use(basicAuth(a))
	{
		h.GET("/", sh.listShelves)
		h.GET("/newest/", sh.listNewest)
		h.GET("/series/", sh.listBySeries)
		h.GET("/series/books/", sh.listSeriesBooks)
		h.GET("/author/", sh.listByAuthor)
		h.GET("/author/books/", sh.listAuthorBooks)
		h.GET("/publisher/", sh.listByPublisher)
		h.GET("/publisher/books/", sh.listPublisherBooks)
		h.GET("/book/:bookID/download", sh.downloadBook)
		h.GET("/book/:bookID/cover", sh.viewBookCover)
		// TODO: search
	}
}

func (r *OPDSRouter) listShelves(c *gin.Context) {
	shelves := []Entry{
		{
			ID:      "urn:kompanion:newest",
			Updated: time.Now().UTC().Format(AtomTime),
			Title:   "By Newest",
			Link: []Link{
				{
					Href: "/opds/newest/",
					Type: "application/atom+xml;type=feed;profile=opds-catalog",
				},
			},
		},
		{
			ID:      "urn:kompanion:series",
			Updated: time.Now().UTC().Format(AtomTime),
			Title:   "By Series",
			Link: []Link{
				{
					Href: "/opds/series/",
					Type: "application/atom+xml;type=feed;profile=opds-catalog",
				},
			},
		},
		{
			ID:      "urn:kompanion:author",
			Updated: time.Now().UTC().Format(AtomTime),
			Title:   "By Author",
			Link: []Link{
				{
					Href: "/opds/author/",
					Type: "application/atom+xml;type=feed;profile=opds-catalog",
				},
			},
		},
		{
			ID:      "urn:kompanion:publisher",
			Updated: time.Now().UTC().Format(AtomTime),
			Title:   "By Publisher",
			Link: []Link{
				{
					Href: "/opds/publisher/",
					Type: "application/atom+xml;type=feed;profile=opds-catalog",
				},
			},
		},
	}
	links := []Link{}
	feed := BuildFeed("urn:kompanion:main", "KOmpanion library", "/opds", shelves, links)
	c.XML(http.StatusOK, feed)
}

func (r *OPDSRouter) listNewest(c *gin.Context) {
	r.listBySort(c, "created_at", "desc", "urn:kompanion:newest", "/opds/newest/", "KOmpanion library")
}

func (r *OPDSRouter) listBySeries(c *gin.Context) {
	r.listCategoryFeed(c, "series", "/opds/series/books/", "urn:kompanion:series", "KOmpanion series")
}

func (r *OPDSRouter) listSeriesBooks(c *gin.Context) {
	r.listBooksByCategory(c, "series", "urn:kompanion:series:books", "/opds/series/books/", "KOmpanion books by series")
}

func (r *OPDSRouter) listByAuthor(c *gin.Context) {
	r.listCategoryFeed(c, "author", "/opds/author/books/", "urn:kompanion:author", "KOmpanion authors")
}

func (r *OPDSRouter) listAuthorBooks(c *gin.Context) {
	r.listBooksByCategory(c, "author", "urn:kompanion:author:books", "/opds/author/books/", "KOmpanion books by author")
}

func (r *OPDSRouter) listByPublisher(c *gin.Context) {
	r.listCategoryFeed(c, "publisher", "/opds/publisher/books/", "urn:kompanion:publisher", "KOmpanion publishers")
}

func (r *OPDSRouter) listPublisherBooks(c *gin.Context) {
	r.listBooksByCategory(c, "publisher", "urn:kompanion:publisher:books", "/opds/publisher/books/", "KOmpanion books by publisher")
}

func (r *OPDSRouter) listBySort(c *gin.Context, sortBy, sortOrder, feedID, baseUrl, title string) {
	pageStr := c.Query("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 1
	}
	books, err := r.books.ListBooks(c.Request.Context(), sortBy, sortOrder, page, 10)
	if err != nil {
		r.logger.Error("failed to list books", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error", "code": 1001})
		return
	}
	entries := translateBooksToEntries(books.Books)
	navLinks := formNavLinks(baseUrl, books)
	feed := BuildFeed(feedID, title, baseUrl, entries, navLinks)
	c.XML(http.StatusOK, feed)
}

func (r *OPDSRouter) listCategoryFeed(c *gin.Context, groupBy, categoryBooksPath, feedID, title string) {
	groups, err := r.loadGroupedBooks(c, groupBy)
	if err != nil {
		r.logger.Error("failed to list category feed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error", "code": 1001})
		return
	}

	entries := make([]Entry, 0, len(groups))
	for _, category := range sortedCategories(groups) {
		bookCount := len(groups[category])
		entries = append(entries, Entry{
			ID:      fmt.Sprintf("urn:kompanion:%s:%s", groupBy, url.QueryEscape(category)),
			Updated: time.Now().UTC().Format(AtomTime),
			Title:   fmt.Sprintf("%s (%d)", category, bookCount),
			Link: []Link{{
				Href: fmt.Sprintf("%s?name=%s", categoryBooksPath, url.QueryEscape(category)),
				Type: DirMime,
			}},
		})
	}

	feed := BuildFeed(feedID, title, c.Request.URL.Path, entries, []Link{})
	c.XML(http.StatusOK, feed)
}

func (r *OPDSRouter) listBooksByCategory(c *gin.Context, groupBy, feedID, basePath, title string) {
	groups, err := r.loadGroupedBooks(c, groupBy)
	if err != nil {
		r.logger.Error("failed to list category books", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error", "code": 1001})
		return
	}

	category := normalizeCategory(c.Query("name"))
	books := groups[category]

	page := parsePage(c.Query("page"))
	perPage := 10
	start := (page - 1) * perPage
	if start > len(books) {
		start = len(books)
	}
	end := start + perPage
	if end > len(books) {
		end = len(books)
	}

	pageBooks := books[start:end]
	entries := translateBooksToEntries(pageBooks)
	paginated := library.NewPaginatedBookList(pageBooks, perPage, page, len(books))
	navLinks := formNavLinks(fmt.Sprintf("%s?name=%s", basePath, url.QueryEscape(category)), paginated)

	feed := BuildFeed(fmt.Sprintf("%s:%s", feedID, url.QueryEscape(category)), fmt.Sprintf("%s: %s", title, category), basePath, entries, navLinks)
	c.XML(http.StatusOK, feed)
}

func (r *OPDSRouter) loadGroupedBooks(c *gin.Context, groupBy string) (map[string][]entity.Book, error) {
	allBooks, err := r.listAllBooks(c)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]entity.Book)
	for _, book := range allBooks {
		category := ""
		switch groupBy {
		case "series":
			category = book.Series
		case "author":
			category = book.Author
		case "publisher":
			category = book.Publisher
		}
		normalizedCategory := normalizeCategory(category)
		groups[normalizedCategory] = append(groups[normalizedCategory], book)
	}

	return groups, nil
}

func (r *OPDSRouter) listAllBooks(c *gin.Context) ([]entity.Book, error) {
	const pageSize = 100
	page := 1
	books := make([]entity.Book, 0)

	for {
		result, err := r.books.ListBooks(c.Request.Context(), "created_at", "desc", page, pageSize)
		if err != nil {
			return nil, err
		}

		for _, b := range result.Books {
			books = append(books, b)
		}

		if len(result.Books) < pageSize {
			break
		}
		page++
	}

	return books, nil
}

func parsePage(pageRaw string) int {
	page, err := strconv.Atoi(pageRaw)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func normalizeCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return "未分类"
	}
	return category
}

func sortedCategories(groups map[string][]entity.Book) []string {
	categories := make([]string, 0, len(groups))
	for category := range groups {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	if len(categories) > 0 && categories[0] != "未分类" {
		for i, category := range categories {
			if category == "未分类" {
				categories = append([]string{"未分类"}, append(categories[:i], categories[i+1:]...)...)
				break
			}
		}
	}

	return categories
}

func (r *OPDSRouter) downloadBook(c *gin.Context) {
	bookID := c.Param("bookID")

	book, file, err := r.books.DownloadBook(c.Request.Context(), bookID)
	if err != nil {
		r.logger.Error(err, "http - v1 - shelf - downloadBook")
		c.JSON(500, gin.H{"message": "internal server error"})
		return
	}
	defer file.Close()

	c.Header("Content-Disposition", "attachment; filename="+book.Filename())
	c.Header("Content-Type", "application/octet-stream")
	c.File(file.Name())
}

func (r *OPDSRouter) viewBookCover(c *gin.Context) {
	bookID := c.Param("bookID")

	cover, err := r.books.ViewCover(c.Request.Context(), bookID)
	if err != nil {
		r.logger.Error(err, "http - opds - shelf - viewBookCover")
		c.Status(http.StatusNotFound)
		return
	}
	defer cover.Close()

	c.Header("Content-Type", "image/jpeg")
	c.File(cover.Name())
}

func basicAuth(auth auth.AuthInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="KOmpanion OPDS"`)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized", "code": 2001})
			c.Abort()
			return
		}
		if !auth.CheckDevicePassword(c.Request.Context(), username, password, true) {
			if !auth.CheckPassword(c.Request.Context(), username, password) {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized", "code": 2001})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
