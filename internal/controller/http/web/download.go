package web

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func serveBookDownload(c *gin.Context, file *os.File, filename string) {
	if _, err := file.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, passStandartContext(c, gin.H{"message": "internal server error"}))
		return
	}

	stat, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, passStandartContext(c, gin.H{"message": "internal server error"}))
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Accept-Ranges", "bytes")

	http.ServeContent(c.Writer, c.Request, filename, stat.ModTime(), file)
}
