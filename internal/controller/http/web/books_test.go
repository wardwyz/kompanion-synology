package web

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRemoteFilenameFromContentDisposition(t *testing.T) {
	t.Parallel()

	sourceURL, err := url.Parse("https://example.com/download")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Content-Disposition", "attachment; filename*=UTF-8''My%20Book.epub")

	got := remoteFilename(resp, sourceURL)
	if got != "My Book.epub" {
		t.Fatalf("expected filename from content-disposition, got %q", got)
	}
}

func TestRemoteFilenameFallsBackToURLPath(t *testing.T) {
	t.Parallel()

	sourceURL, err := url.Parse("https://example.com/files/test-book.pdf")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	got := remoteFilename(&http.Response{Header: make(http.Header)}, sourceURL)
	if got != "test-book.pdf" {
		t.Fatalf("expected fallback filename from path, got %q", got)
	}
}

func TestIsSupportedBookFilename(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"a.epub": true,
		"a.pdf":  true,
		"a.fb2":  true,
		"a.mobi": false,
		"a":      false,
	}

	for filename, want := range cases {
		filename, want := filename, want
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			if got := isSupportedBookFilename(filename); got != want {
				t.Fatalf("isSupportedBookFilename(%q) = %v, want %v", filename, got, want)
			}
		})
	}
}
