package web

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	downloadhttp "github.com/vanadium23/kompanion/internal/controller/http/download"
)

func serveBookDownload(c *gin.Context, file *os.File, filename string) {
	if err := downloadhttp.Serve(c.Writer, c.Request, file, filename); err != nil {
		c.JSON(http.StatusInternalServerError, passStandartContext(c, gin.H{"message": "internal server error"}))
	}
}
