package notes

import (
	"context"
	"time"

	"github.com/moroz/uuidv7-go"
	"github.com/vanadium23/kompanion/internal/entity"
)

type NotesService struct {
	repo Repo
}

func NewService(repo Repo) *NotesService {
	return &NotesService{repo: repo}
}

func (s *NotesService) Save(ctx context.Context, note entity.ReadingNote) (entity.ReadingNote, error) {
	now := time.Now().UTC()
	if note.ID == "" {
		note.ID = uuidv7.Generate().String()
	}
	note.CreatedAt = now
	note.UpdatedAt = now

	if err := s.repo.Store(ctx, note); err != nil {
		return entity.ReadingNote{}, err
	}
	return note, nil
}

func (s *NotesService) Update(ctx context.Context, note entity.ReadingNote) (entity.ReadingNote, error) {
	note.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, note); err != nil {
		return entity.ReadingNote{}, err
	}
	return s.repo.Get(ctx, note.ID)
}

func (s *NotesService) Get(ctx context.Context, id string) (entity.ReadingNote, error) {
	return s.repo.Get(ctx, id)
}

func (s *NotesService) List(ctx context.Context, limit int) ([]entity.ReadingNote, error) {
	return s.repo.List(ctx, limit)
}

func (s *NotesService) ListByDocument(ctx context.Context, documentID string, limit int) ([]entity.ReadingNote, error) {
	return s.repo.ListByDocument(ctx, documentID, limit)
}
