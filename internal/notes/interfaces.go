package notes

import (
	"context"

	"github.com/vanadium23/kompanion/internal/entity"
)

type Repo interface {
	Store(ctx context.Context, note entity.ReadingNote) error
	Update(ctx context.Context, note entity.ReadingNote) error
	Get(ctx context.Context, id string) (entity.ReadingNote, error)
	List(ctx context.Context, limit int) ([]entity.ReadingNote, error)
	ListByDocument(ctx context.Context, documentID string, limit int) ([]entity.ReadingNote, error)
}

type Service interface {
	Save(ctx context.Context, note entity.ReadingNote) (entity.ReadingNote, error)
	Update(ctx context.Context, note entity.ReadingNote) (entity.ReadingNote, error)
	Get(ctx context.Context, id string) (entity.ReadingNote, error)
	List(ctx context.Context, limit int) ([]entity.ReadingNote, error)
	ListByDocument(ctx context.Context, documentID string, limit int) ([]entity.ReadingNote, error)
}
