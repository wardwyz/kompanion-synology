package entity

import "time"

// ReadingNote stores a markdown note synced from external readers (for example KOReader via Joplin API).
type ReadingNote struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	DocumentID string    `json:"document_id"`
	Source     string    `json:"source"`
	SourceURL  string    `json:"source_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
