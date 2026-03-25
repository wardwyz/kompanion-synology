package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/library"
	"github.com/vanadium23/kompanion/internal/stats"
	syncpkg "github.com/vanadium23/kompanion/internal/sync"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type booksRoutes struct {
	shelf    library.Shelf
	stats    stats.ReadingStats
	progress syncpkg.Progress
	logger   logger.Interface
}

func newBooksRoutes(handler *gin.RouterGroup, shelf library.Shelf, stats stats.ReadingStats, progress syncpkg.Progress, l logger.Interface) {
	r := &booksRoutes{
		shelf:    shelf,
		stats:    stats,
		progress: progress,
		logger:   l,
	}

	handler.GET("/", r.listBooks)
	handler.POST("/upload", r.uploadBook)
	handler.GET("/:bookID", r.viewBook)
	handler.POST("/:bookID", r.updateBookMetadata)
	handler.POST("/:bookID/delete", r.deleteBook)
	handler.GET("/:bookID/download", r.downloadBook)
	handler.GET("/:bookID/cover", r.viewBookCover)
}

func (r *booksRoutes) listBooks(c *gin.Context) {
	renderBooksPage(c, r.shelf, r.progress, r.logger, http.StatusOK, gin.H{})
}

func renderBooksPage(
	c *gin.Context,
	shelf library.Shelf,
	progressSync syncpkg.Progress,
	log logger.Interface,
	status int,
	data gin.H,
) {
	page := 1
	perPage := 12 // Show 12 books per page for grid layout
	searchQuery := strings.TrimSpace(c.Query("q"))

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	var (
		books library.PaginatedBookList
		err   error
	)

	if searchQuery == "" {
		books, err = shelf.ListBooks(c.Request.Context(), "created_at", "desc", page, perPage)
		if err != nil {
			c.HTML(500, "error", passStandartContext(c, gin.H{"error": err.Error()}))
			return
		}
	} else {
		var allBooks []entity.Book
		allBooks, err = listAllBooks(c.Request.Context(), shelf, 200)
		if err != nil {
			c.HTML(500, "error", passStandartContext(c, gin.H{"error": err.Error()}))
			return
		}

		filteredBooks := filterBooksByQuery(allBooks, searchQuery)
		books, page = paginateBooks(filteredBooks, page, perPage)
	}

	// Fetch progress for each book
	type BookWithProgress struct {
		entity.Book
		Progress int
	}
	booksWithProgress := make([]BookWithProgress, len(books.Books))
	for i, book := range books.Books {
		progress, err := progressSync.Fetch(c.Request.Context(), book.DocumentID)
		if err != nil {
			log.Error(err, "failed to fetch progress for book %s", book.ID)
			progress = entity.Progress{}
		}
		booksWithProgress[i] = BookWithProgress{
			Book:     book,
			Progress: int(progress.Percentage * 100),
		}
	}

	pageData := gin.H{
		"books":       booksWithProgress,
		"searchQuery": searchQuery,
		"pagination": gin.H{
			"currentPage": page,
			"perPage":     perPage,
			"totalPages":  books.TotalPages(),
			"hasNext":     books.HasNext(),
			"hasPrev":     books.HasPrev(),
			"nextPage":    books.Next(),
			"prevPage":    books.Prev(),
			"firstPage":   books.First(),
			"lastPage":    books.Last(),
		},
	}

	for key, value := range data {
		pageData[key] = value
	}

	c.HTML(status, "books", passStandartContext(c, pageData))
}

func listAllBooks(ctx context.Context, shelf library.Shelf, fetchPerPage int) ([]entity.Book, error) {
	page := 1
	var allBooks []entity.Book

	for {
		books, err := shelf.ListBooks(ctx, "created_at", "desc", page, fetchPerPage)
		if err != nil {
			return nil, err
		}

		allBooks = append(allBooks, books.Books...)
		if !books.HasNext() {
			break
		}
		page = books.Next()
	}

	return allBooks, nil
}

func filterBooksByQuery(books []entity.Book, query string) []entity.Book {
	if query == "" {
		return books
	}

	keyword := strings.ToLower(query)
	filtered := make([]entity.Book, 0, len(books))
	for _, book := range books {
		searchText := strings.ToLower(strings.TrimSpace(book.Title + " " + book.Author + " " + book.Series))
		if strings.Contains(searchText, keyword) {
			filtered = append(filtered, book)
		}
	}

	return filtered
}

func paginateBooks(books []entity.Book, page, perPage int) (library.PaginatedBookList, int) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 12
	}

	totalCount := len(books)
	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + perPage - 1) / perPage
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}

	start := (page - 1) * perPage
	if start > totalCount {
		start = totalCount
	}
	end := start + perPage
	if end > totalCount {
		end = totalCount
	}

	return library.NewPaginatedBookList(books[start:end], perPage, page, totalCount), page
}

func (r *booksRoutes) uploadBook(c *gin.Context) {
	// single uploadedBookFile
	uploadedBookFile, err := c.FormFile("book")
	if err != nil {
		r.logger.Error(err, "http - v1 - shelf - uploadBook")
		renderBooksPage(c, r.shelf, r.progress, r.logger, http.StatusBadRequest, gin.H{
			"uploadError": "请选择要上传的书籍文件。",
		})
		return
	}

	// make by temp files
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		r.logger.Error(err, "http - v1 - shelf - putBook")
		renderBooksPage(c, r.shelf, r.progress, r.logger, http.StatusInternalServerError, gin.H{
			"uploadError": "服务器暂时无法创建上传临时文件，请稍后重试。",
		})
		return
	}
	filepath := tempFile.Name()
	defer os.Remove(filepath)
	defer tempFile.Close()
	if err := c.SaveUploadedFile(uploadedBookFile, filepath); err != nil {
		r.logger.Error(err, "http - web - shelf - saveUploadedFile")
		renderBooksPage(c, r.shelf, r.progress, r.logger, http.StatusBadRequest, gin.H{
			"uploadError": "上传文件失败，请确认文件大小和网络连接后重试。",
		})
		return
	}

	book, err := r.shelf.StoreBook(c.Request.Context(), tempFile, uploadedBookFile.Filename)
	if err != nil && err != entity.ErrBookAlreadyExists {
		r.logger.Error(err, "http - v1 - shelf - putBook")
		renderBooksPage(c, r.shelf, r.progress, r.logger, http.StatusBadRequest, gin.H{
			"uploadError": "文件上传成功，但解析失败。请确认上传的是支持的电子书格式（EPUB/PDF/FB2）。",
		})
		return
	}
	c.Redirect(302, "/books/"+book.ID)
}

func (r *booksRoutes) downloadBook(c *gin.Context) {
	bookID := c.Param("bookID")

	book, file, err := r.shelf.DownloadBook(c.Request.Context(), bookID)
	if err != nil {
		c.JSON(500, passStandartContext(c, gin.H{"message": "internal server error"}))
		return
	}
	defer file.Close()

	c.Header("Content-Disposition", "attachment; filename="+book.Filename())
	c.Header("Content-Type", "application/octet-stream")
	c.File(file.Name())
}

func (r *booksRoutes) viewBook(c *gin.Context) {
	bookID := c.Param("bookID")

	book, err := r.shelf.ViewBook(c.Request.Context(), bookID)
	if err != nil {
		c.HTML(500, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
	}

	bookStats, err := r.stats.GetBookStats(c.Request.Context(), book.DocumentID)
	if err != nil {
		r.logger.Error(err, "failed to get book stats")
		bookStats = &stats.BookStats{} // Use empty stats in case of error
	}

	c.HTML(200, "book", passStandartContext(c, gin.H{
		"book":  book,
		"stats": bookStats,
	}))
}

func (r *booksRoutes) updateBookMetadata(c *gin.Context) {
	bookID := c.Param("bookID")

	var metadata entity.Book
	if err := c.ShouldBind(&metadata); err != nil {
		r.logger.Error(err, "http - v1 - shelf - updateBookMetadata")
		// TODO: move to template
		c.JSON(400, passStandartContext(c, gin.H{"message": "invalid request"}))
		return
	}

	book, err := r.shelf.UpdateBookMetadata(c.Request.Context(), bookID, metadata)
	if err != nil {
		r.logger.Error(err, "http - v1 - shelf - updateBookMetadata")
		// TODO: move to template
		c.JSON(500, passStandartContext(c, gin.H{"message": "internal server error"}))
		return
	}

	// TODO: why not redirect?
	c.HTML(200, "book", passStandartContext(c, gin.H{"book": book}))
}

func (r *booksRoutes) viewBookCover(c *gin.Context) {
	bookID := c.Param("bookID")

	book, err := r.shelf.ViewBook(c.Request.Context(), bookID)
	if err != nil {
		c.JSON(500, passStandartContext(c, gin.H{"message": "internal server error"}))
		return
	}

	cover, err := r.shelf.ViewCover(c.Request.Context(), bookID)

	if err != nil {
		width := 600
		height := 800
		backgroundColor := "#6496FA" // Цвет фона (голубой)
		textColor := "white"         // Цвет текста
		title := book.Title
		subtitle := book.Author
		fontSizeTitle := 48
		fontSizeSubtitle := 24

		svgContent := fmt.Sprintf(`
		<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">
			<rect width="100%%" height="100%%" fill="%s" />
			<text x="50%%" y="40%%" font-family="Arial" font-size="%d" fill="%s" text-anchor="middle">%s</text>
			<text x="50%%" y="55%%" font-family="Arial" font-size="%d" fill="%s" text-anchor="middle">%s</text>
		</svg>
		`, width, height, backgroundColor, fontSizeTitle, textColor, title, fontSizeSubtitle, textColor, subtitle)

		c.Data(200, "image/svg+xml", []byte(svgContent))
		return
	}
	c.File(cover.Name())
}

func (r *booksRoutes) deleteBook(c *gin.Context) {
	bookID := c.Param("bookID")

	if err := r.shelf.DeleteBook(c.Request.Context(), bookID); err != nil {
		r.logger.Error(err, "http - web - shelf - deleteBook")
		c.HTML(500, "error", passStandartContext(c, gin.H{"error": "删除书籍失败"}))
		return
	}

	c.Redirect(302, "/books")
}
