package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/internal/library"
	"github.com/vanadium23/kompanion/internal/stats"
	syncpkg "github.com/vanadium23/kompanion/internal/sync"
	"github.com/vanadium23/kompanion/pkg/logger"
)

type booksRoutes struct {
	shelf       library.Shelf
	stats       stats.ReadingStats
	progress    syncpkg.Progress
	logger      logger.Interface
	httpClient  *http.Client
	zLibraryURL string
}

func newBooksRoutes(handler *gin.RouterGroup, shelf library.Shelf, stats stats.ReadingStats, progress syncpkg.Progress, l logger.Interface, zLibraryURL string) {
	r := &booksRoutes{
		shelf:       shelf,
		stats:       stats,
		progress:    progress,
		logger:      l,
		zLibraryURL: zLibraryURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}

	handler.GET("/", r.listBooks)
	handler.POST("/upload", r.uploadBook)
	handler.POST("/upload-url", r.uploadBookFromURL)
	handler.GET("/:bookID", r.viewBook)
	handler.POST("/:bookID", r.updateBookMetadata)
	handler.POST("/:bookID/delete", r.deleteBook)
	handler.GET("/:bookID/download", r.downloadBook)
	handler.GET("/:bookID/cover", r.viewBookCover)
}

func (r *booksRoutes) listBooks(c *gin.Context) {
	renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusOK, gin.H{})
}

func renderBooksPage(
	c *gin.Context,
	shelf library.Shelf,
	progressSync syncpkg.Progress,
	log logger.Interface,
	zLibraryURL string,
	status int,
	data gin.H,
) {
	page := 1
	perPage := 12 // Show 12 books per page for grid layout
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	books, err := shelf.ListBooks(c.Request.Context(), "created_at", "desc", page, perPage)
	if err != nil {
		c.HTML(500, "error", passStandartContext(c, gin.H{"error": err.Error()}))
		return
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
		"zLibraryURL": zLibraryURL,
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

func (r *booksRoutes) uploadBook(c *gin.Context) {
	// single uploadedBookFile
	uploadedBookFile, err := c.FormFile("book")
	if err != nil {
		r.logger.Error(err, "http - v1 - shelf - uploadBook")
		renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusBadRequest, gin.H{
			"uploadError": "请选择要上传的书籍文件。",
		})
		return
	}

	// make by temp files
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		r.logger.Error(err, "http - v1 - shelf - putBook")
		renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusInternalServerError, gin.H{
			"uploadError": "服务器暂时无法创建上传临时文件，请稍后重试。",
		})
		return
	}
	filepath := tempFile.Name()
	defer os.Remove(filepath)
	defer tempFile.Close()
	if err := c.SaveUploadedFile(uploadedBookFile, filepath); err != nil {
		r.logger.Error(err, "http - web - shelf - saveUploadedFile")
		renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusBadRequest, gin.H{
			"uploadError": "上传文件失败，请确认文件大小和网络连接后重试。",
		})
		return
	}

	book, err := r.shelf.StoreBook(c.Request.Context(), tempFile, uploadedBookFile.Filename)
	if err != nil && err != entity.ErrBookAlreadyExists {
		r.logger.Error(err, "http - v1 - shelf - putBook")
		renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusBadRequest, gin.H{
			"uploadError": "文件上传成功，但解析失败。请确认上传的是支持的电子书格式（EPUB/PDF/FB2）。",
		})
		return
	}
	c.Redirect(302, "/books/"+book.ID)
}

func (r *booksRoutes) uploadBookFromURL(c *gin.Context) {
	remoteURL := strings.TrimSpace(c.PostForm("url"))
	if remoteURL == "" {
		renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusBadRequest, gin.H{
			"remoteUploadError": "请输入可直接下载电子书文件的链接。",
		})
		return
	}

	book, err := r.fetchAndStoreRemoteBook(c.Request.Context(), remoteURL)
	if err != nil {
		r.logger.Error(err, "http - web - shelf - uploadBookFromURL")
		renderBooksPage(c, r.shelf, r.progress, r.logger, r.zLibraryURL, http.StatusBadRequest, gin.H{
			"remoteUploadError": err.Error(),
			"remoteUploadURL":   remoteURL,
		})
		return
	}

	c.Redirect(302, "/books/"+book.ID)
}

func (r *booksRoutes) fetchAndStoreRemoteBook(ctx context.Context, remoteURL string) (entity.Book, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return entity.Book{}, errors.New("下载链接格式不正确，请粘贴完整的 http(s) 链接")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return entity.Book{}, errors.New("仅支持 http 或 https 下载链接")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return entity.Book{}, errors.New("无法创建远程下载请求")
	}
	req.Header.Set("User-Agent", "KOmpanion/remote-import")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return entity.Book{}, errors.New("无法访问远程下载链接，请确认链接有效并且服务器可访问")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return entity.Book{}, fmt.Errorf("远程站点返回了 %d，无法直接下载文件", resp.StatusCode)
	}

	filename := remoteFilename(resp, parsedURL)
	if !isSupportedBookFilename(filename) {
		return entity.Book{}, errors.New("远程链接不是支持的电子书文件（仅支持 EPUB/PDF/FB2）")
	}

	tempFile, err := os.CreateTemp("", "kompanion-remote-*")
	if err != nil {
		return entity.Book{}, errors.New("服务器暂时无法创建下载临时文件")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return entity.Book{}, errors.New("远程文件下载到服务器时失败，请稍后重试")
	}
	if _, err := tempFile.Seek(0, 0); err != nil {
		return entity.Book{}, errors.New("服务器处理远程文件时失败")
	}

	book, err := r.shelf.StoreBook(ctx, tempFile, filename)
	if err != nil {
		if errors.Is(err, entity.ErrBookAlreadyExists) {
			return book, nil
		}
		return entity.Book{}, errors.New("远程文件已下载，但解析失败。请确认它是 EPUB/PDF/FB2 格式")
	}

	return book, nil
}

func remoteFilename(resp *http.Response, sourceURL *url.URL) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if filename := strings.TrimSpace(params["filename*"]); filename != "" {
				if idx := strings.Index(filename, "''"); idx >= 0 {
					filename = filename[idx+2:]
				}
				if unescaped, err := url.QueryUnescape(filename); err == nil {
					return path.Base(unescaped)
				}
				return path.Base(filename)
			}
			if filename := strings.TrimSpace(params["filename"]); filename != "" {
				return path.Base(filename)
			}
		}
	}

	name := path.Base(sourceURL.Path)
	if name == "." || name == "/" || name == "" {
		return "downloaded-book"
	}
	return name
}

func isSupportedBookFilename(filename string) bool {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".epub", ".pdf", ".fb2":
		return true
	default:
		return false
	}
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
