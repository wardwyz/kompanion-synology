package metadata

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// RewriteEPUBMetadata creates a temporary EPUB file with updated metadata fields.
// If metadata fields are empty, the original values are kept.
func RewriteEPUBMetadata(srcPath string, data Metadata) (*os.File, error) {
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	opfPath, err := findEPUBPackagePath(&zr.Reader)
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "kompanion-epub-*.epub")
	if err != nil {
		return nil, err
	}

	zw := zip.NewWriter(tmpFile)

	for _, f := range zr.File {
		content, readErr := readFileContent(f)
		if readErr != nil {
			_ = zw.Close()
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, readErr
		}

		if f.Name == opfPath {
			content = rewriteOPFMetadata(content, data)
		}

		h := f.FileHeader
		w, createErr := zw.CreateHeader(&h)
		if createErr != nil {
			_ = zw.Close()
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, createErr
		}
		if _, writeErr := w.Write(content); writeErr != nil {
			_ = zw.Close()
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, writeErr
		}
	}

	if err = zw.Close(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, err
	}

	if _, err = tmpFile.Seek(0, io.SeekStart); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, err
	}

	return tmpFile, nil
}

func findEPUBPackagePath(reader *zip.Reader) (string, error) {
	for _, f := range reader.File {
		if f.Name != "META-INF/container.xml" {
			continue
		}
		container, err := parseContainerXML(f)
		if err != nil {
			return "", err
		}
		if len(container.Rootfiles) == 0 {
			return "", fmt.Errorf("epub container.xml has no rootfile")
		}
		return container.Rootfiles[0].FullPath, nil
	}

	return "", fmt.Errorf("epub container.xml not found")
}

func rewriteOPFMetadata(opf []byte, data Metadata) []byte {
	result := string(opf)

	result = replaceXMLTag(result, "title", strings.TrimSpace(data.Title))
	result = replaceXMLTag(result, "creator", strings.TrimSpace(data.Author))
	result = replaceXMLTag(result, "description", strings.TrimSpace(data.Description))
	result = replaceXMLTag(result, "publisher", strings.TrimSpace(data.Publisher))
	result = replaceXMLTag(result, "identifier", strings.TrimSpace(data.ISBN))

	result = replaceMetaNameContent(result, "calibre:series", strings.TrimSpace(data.Series))
	result = replaceMetaNameContent(result, "calibre:series_index", strings.TrimSpace(data.SeriesIndex))

	return []byte(result)
}

func replaceXMLTag(content, tagName, replacement string) string {
	if replacement == "" {
		return content
	}

	pattern := regexp.MustCompile(`(?s)<(?:[[:alnum:]_\-]+:)?` + regexp.QuoteMeta(tagName) + `\b[^>]*>.*?</(?:[[:alnum:]_\-]+:)?` + regexp.QuoteMeta(tagName) + `>`)
	match := pattern.FindString(content)
	if match == "" {
		return content
	}

	openEnd := strings.Index(match, ">")
	closeStart := strings.LastIndex(match, "</")
	if openEnd < 0 || closeStart <= openEnd {
		return content
	}

	escaped := escapeXMLText(replacement)
	rewritten := match[:openEnd+1] + escaped + match[closeStart:]
	return strings.Replace(content, match, rewritten, 1)
}

func replaceMetaNameContent(content, metaName, replacement string) string {
	if replacement == "" {
		return content
	}

	namePattern := regexp.MustCompile(`<meta\b[^>]*\bname=["']` + regexp.QuoteMeta(metaName) + `["'][^>]*>`)
	metaTag := namePattern.FindString(content)
	if metaTag == "" {
		return content
	}

	contentPattern := regexp.MustCompile(`\bcontent=["'][^"']*["']`)
	newAttr := `content="` + escapeXMLAttr(replacement) + `"`
	if contentPattern.MatchString(metaTag) {
		updatedTag := contentPattern.ReplaceAllString(metaTag, newAttr)
		return strings.Replace(content, metaTag, updatedTag, 1)
	}

	updatedTag := strings.TrimSuffix(metaTag, ">") + " " + newAttr + ">"
	return strings.Replace(content, metaTag, updatedTag, 1)
}

func escapeXMLText(value string) string {
	v := bytes.NewBuffer(nil)
	xmlEscaper(v, value)
	return v.String()
}

func escapeXMLAttr(value string) string {
	v := strings.ReplaceAll(value, "&", "&amp;")
	v = strings.ReplaceAll(v, `"`, "&quot;")
	v = strings.ReplaceAll(v, "<", "&lt;")
	v = strings.ReplaceAll(v, ">", "&gt;")
	return v
}

func xmlEscaper(buf *bytes.Buffer, value string) {
	for _, r := range value {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		default:
			buf.WriteRune(r)
		}
	}
}
