package metadata

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceOPFMetadata(t *testing.T) {
	input := `<package><metadata><dc:title>Old</dc:title><dc:creator>A</dc:creator><dc:description>D</dc:description><dc:publisher>P0</dc:publisher><dc:identifier>ID0</dc:identifier></metadata></package>`
	got := string(replaceOPFMetadata([]byte(input), Metadata{
		Title:       `新书名`,
		Author:      `新作者`,
		Description: `新简介 & test`,
		Publisher:   `人民文学出版社`,
		ISBN:        `9787100000000`,
	}))

	if !strings.Contains(got, `<dc:title>新书名</dc:title>`) {
		t.Fatalf("title not updated: %s", got)
	}
	if !strings.Contains(got, `<dc:creator>新作者</dc:creator>`) {
		t.Fatalf("author not updated: %s", got)
	}
	if !strings.Contains(got, `<dc:description>新简介 &amp; test</dc:description>`) {
		t.Fatalf("description not updated: %s", got)
	}
	if !strings.Contains(got, `<dc:publisher>人民文学出版社</dc:publisher>`) {
		t.Fatalf("publisher not updated: %s", got)
	}
	if !strings.Contains(got, `<dc:identifier>9787100000000</dc:identifier>`) {
		t.Fatalf("identifier not updated: %s", got)
	}
}

func TestReplaceOPFMetadata_InsertWhenTagMissing(t *testing.T) {
	input := `<package><metadata><dc:title>Old</dc:title></metadata></package>`
	got := string(replaceOPFMetadata([]byte(input), Metadata{
		Publisher: "人民文学出版社",
		ISBN:      "9787100000000",
	}))

	if !strings.Contains(got, `<dc:publisher>人民文学出版社</dc:publisher>`) {
		t.Fatalf("publisher not inserted: %s", got)
	}
	if !strings.Contains(got, `<dc:identifier>9787100000000</dc:identifier>`) {
		t.Fatalf("identifier not inserted: %s", got)
	}
}

func TestRewriteDownloadedMetadataEPUB(t *testing.T) {
	src := buildTestEPUB(t, `<package><metadata><dc:title>Old</dc:title></metadata></package>`)
	defer os.Remove(src.Name())
	defer src.Close()

	rewritten, err := RewriteDownloadedMetadata(src, "epub", Metadata{Title: "豆瓣标题"})
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	defer rewritten.Close()

	info, err := rewritten.Stat()
	if err != nil {
		t.Fatalf("stat rewritten failed: %v", err)
	}
	zr, err := zip.NewReader(rewritten, info.Size())
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}

	var opfBody string
	for _, f := range zr.File {
		if f.Name != "OEBPS/content.opf" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open opf failed: %v", err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		opfBody = string(data)
	}

	if !strings.Contains(opfBody, `<dc:title>豆瓣标题</dc:title>`) {
		t.Fatalf("expected title rewritten, got: %s", opfBody)
	}
}

func TestRewriteDownloadedMetadataEPUBWithOnlyPublisherAndISBN(t *testing.T) {
	src := buildTestEPUB(t, `<package><metadata><dc:title>Old</dc:title></metadata></package>`)
	defer os.Remove(src.Name())
	defer src.Close()

	rewritten, err := RewriteDownloadedMetadata(src, "epub", Metadata{
		Publisher: "人民文学出版社",
		ISBN:      "9787100000000",
	})
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	defer rewritten.Close()

	if rewritten == src {
		t.Fatal("expected a rewritten file when only publisher/isbn are provided")
	}

	info, err := rewritten.Stat()
	if err != nil {
		t.Fatalf("stat rewritten failed: %v", err)
	}
	zr, err := zip.NewReader(rewritten, info.Size())
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}

	var opfBody string
	for _, f := range zr.File {
		if f.Name != "OEBPS/content.opf" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open opf failed: %v", err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		opfBody = string(data)
	}

	if !strings.Contains(opfBody, `<dc:publisher>人民文学出版社</dc:publisher>`) {
		t.Fatalf("expected publisher rewritten, got: %s", opfBody)
	}
	if !strings.Contains(opfBody, `<dc:identifier>9787100000000</dc:identifier>`) {
		t.Fatalf("expected identifier rewritten, got: %s", opfBody)
	}
}

func buildTestEPUB(t *testing.T, opfContent string) *os.File {
	t.Helper()

	tmp, err := os.CreateTemp("", "rewrite-test-*.epub")
	if err != nil {
		t.Fatalf("create temp failed: %v", err)
	}

	zw := zip.NewWriter(tmp)
	writeZipFile := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s failed: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s failed: %v", name, err)
		}
	}

	writeZipFile("META-INF/container.xml", `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`)
	writeZipFile(filepath.ToSlash("OEBPS/content.opf"), opfContent)

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp file failed: %v", err)
	}

	reopened, err := os.Open(tmp.Name())
	if err != nil {
		t.Fatalf("reopen temp file failed: %v", err)
	}
	return reopened
}
