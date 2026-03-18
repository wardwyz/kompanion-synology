package download

import (
	"fmt"
	"net/http"
	"os"
)

func Serve(w http.ResponseWriter, r *http.Request, file *os.File, filename string) error {
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, filename, stat.ModTime(), file)
	return nil
}
