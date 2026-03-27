package v1

import "testing"

func TestExtractDocumentID(t *testing.T) {
	tests := []struct {
		name string
		in   joplinNotePayload
		want string
	}{
		{
			name: "explicit document id",
			in:   joplinNotePayload{DocumentID: "abc"},
			want: "",
		},
		{
			name: "from body marker",
			in:   joplinNotePayload{Body: "hello\nKOReader_partial_md5: 123456\n"},
			want: "123456",
		},
		{
			name: "from source url",
			in:   joplinNotePayload{SourceURL: "https://example.local?a=1&koreader_partial_md5=xyz789&b=2"},
			want: "xyz789",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractDocumentID(tc.in)
			if got != tc.want {
				t.Fatalf("extractDocumentID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeJoplinTitle(t *testing.T) {
	tests := []struct {
		name string
		in   joplinNotePayload
		want string
	}{
		{
			name: "keep provided title",
			in:   joplinNotePayload{Title: "  Keep Me  "},
			want: "Keep Me",
		},
		{
			name: "fallback to explicit document id",
			in:   joplinNotePayload{DocumentID: "doc-1"},
			want: "KOReader Note doc-1",
		},
		{
			name: "fallback to extracted document id",
			in:   joplinNotePayload{Body: "KOReader_partial_md5: doc-2"},
			want: "KOReader Note doc-2",
		},
		{
			name: "generic fallback",
			in:   joplinNotePayload{},
			want: "KOReader Note",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeJoplinTitle(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeJoplinTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}
