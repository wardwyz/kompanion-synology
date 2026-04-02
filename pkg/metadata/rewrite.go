package metadata

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	opfTitlePattern       = regexp.MustCompile(`(?is)<dc:title[^>]*>.*?</dc:title>`)
	opfCreatorPattern     = regexp.MustCompile(`(?is)<dc:creator[^>]*>.*?</dc:creator>`)
	opfDescriptionPattern = regexp.MustCompile(`(?is)<dc:description[^>]*>.*?</dc:description>`)
	opfPublisherPattern   = regexp.MustCompile(`(?is)<dc:publisher[^>]*>.*?</dc:publisher>`)
	opfIdentifierPattern  = regexp.MustCompile(`(?is)<dc:identifier[^>]*>.*?</dc:identifier>`)
	fb2TitlePattern       = regexp.MustCompile(`(?is)<book-title>.*?</book-title>`)
)

// RewriteDownloadedMetadata rewrites metadata fields in a downloaded file copy.
// It never mutates the original source file and only rewrites supported formats.
func RewriteDownloadedMetadata(src *os.File, format string, m Metadata) (*os.File, error) {
	if src == nil {
		return nil, fmt.Errorf("source file is nil")
	}
	if strings.TrimSpace(m.Title) == "" && strings.TrimSpace(m.Author) == "" && strings.TrimSpace(m.Description) == "" {
		return src, nil
	}

	resolvedFormat := strings.ToLower(strings.TrimSpace(format))
	if resolvedFormat == "" {
		resolvedFormat = strings.TrimPrefix(strings.ToLower(filepath.Ext(src.Name())), ".")
	}

	switch resolvedFormat {
	case "epub":
		return rewriteEPUBTempCopy(src, m)
	case "fb2":
		return rewriteFB2TempCopy(src, m)
	default:
		return src, nil
	}
}

func rewriteEPUBTempCopy(src *os.File, m Metadata) (*os.File, error) {
	tempOut, err := os.CreateTemp("", "kompanion-download-*.epub")
	if err != nil {
		return src, err
	}
	defer func() {
		_ = tempOut.Close()
	}()

	if err := rewriteEPUBMetadata(src.Name(), tempOut.Name(), m); err != nil {
		_ = os.Remove(tempOut.Name())
		return src, err
	}

	result, err := os.Open(tempOut.Name())
	if err != nil {
		_ = os.Remove(tempOut.Name())
		return src, err
	}
	_ = os.Remove(tempOut.Name())
	return result, nil
}

func rewriteFB2TempCopy(src *os.File, m Metadata) (*os.File, error) {
	content, err := os.ReadFile(src.Name())
	if err != nil {
		return src, err
	}
	updated := replaceFB2Metadata(content, m)
	if bytes.Equal(content, updated) {
		return src, nil
	}

	tempOut, err := os.CreateTemp("", "kompanion-download-*.fb2")
	if err != nil {
		return src, err
	}
	if _, err := tempOut.Write(updated); err != nil {
		_ = tempOut.Close()
		_ = os.Remove(tempOut.Name())
		return src, err
	}
	if err := tempOut.Close(); err != nil {
		_ = os.Remove(tempOut.Name())
		return src, err
	}

	result, err := os.Open(tempOut.Name())
	if err != nil {
		_ = os.Remove(tempOut.Name())
		return src, err
	}
	_ = os.Remove(tempOut.Name())
	return result, nil
}

func rewriteEPUBMetadata(srcPath, dstPath string, m Metadata) error {
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	rootfilePath, err := epubRootfilePath(&reader.Reader)
	if err != nil {
		return err
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	writer := zip.NewWriter(dst)
	defer writer.Close()

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return err
		}

		header := file.FileHeader
		w, err := writer.CreateHeader(&header)
		if err != nil {
			rc.Close()
			return err
		}

		if file.Name == rootfilePath {
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}
			updated := replaceOPFMetadata(content, m)
			if _, err := w.Write(updated); err != nil {
				return err
			}
			continue
		}

		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}

	return nil
}

func epubRootfilePath(reader *zip.Reader) (string, error) {
	const containerPath = "META-INF/container.xml"
	for _, file := range reader.File {
		if file.Name != containerPath {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}

		var container struct {
			Rootfiles []struct {
				FullPath string `xml:"full-path,attr"`
			} `xml:"rootfiles>rootfile"`
		}
		if err := xml.Unmarshal(data, &container); err != nil {
			return "", err
		}
		if len(container.Rootfiles) == 0 || strings.TrimSpace(container.Rootfiles[0].FullPath) == "" {
			return "", fmt.Errorf("epub rootfile not found")
		}
		return strings.TrimSpace(container.Rootfiles[0].FullPath), nil
	}
	return "", fmt.Errorf("container.xml not found")
}

func replaceOPFMetadata(content []byte, m Metadata) []byte {
	updated := string(content)
	if title := strings.TrimSpace(m.Title); title != "" {
		replacement := "<dc:title>" + xmlEscape(title) + "</dc:title>"
		updated = opfTitlePattern.ReplaceAllString(updated, replacement)
	}
	if author := strings.TrimSpace(m.Author); author != "" {
		replacement := "<dc:creator>" + xmlEscape(author) + "</dc:creator>"
		updated = opfCreatorPattern.ReplaceAllString(updated, replacement)
	}
	if desc := strings.TrimSpace(m.Description); desc != "" {
		replacement := "<dc:description>" + xmlEscape(desc) + "</dc:description>"
		updated = opfDescriptionPattern.ReplaceAllString(updated, replacement)
	}
	if publisher := strings.TrimSpace(m.Publisher); publisher != "" {
		replacement := "<dc:publisher>" + xmlEscape(publisher) + "</dc:publisher>"
		updated = opfPublisherPattern.ReplaceAllString(updated, replacement)
	}
	if isbn := strings.TrimSpace(m.ISBN); isbn != "" {
		replacement := "<dc:identifier>" + xmlEscape(isbn) + "</dc:identifier>"
		updated = opfIdentifierPattern.ReplaceAllString(updated, replacement)
	}
	return []byte(updated)
}

func replaceFB2Metadata(content []byte, m Metadata) []byte {
	title := strings.TrimSpace(m.Title)
	if title == "" {
		return content
	}
	updated := fb2TitlePattern.ReplaceAllString(string(content), "<book-title>"+xmlEscape(title)+"</book-title>")
	return []byte(updated)
}

func xmlEscape(v string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(v)); err != nil {
		return v
	}
	return b.String()
}
