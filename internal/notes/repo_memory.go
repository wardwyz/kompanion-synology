package notes

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/vanadium23/kompanion/internal/entity"
)

type MemoryRepo struct {
	mu    sync.RWMutex
	items map[string]entity.ReadingNote
}

func NewMemoryRepo() *MemoryRepo {
	return &MemoryRepo{items: make(map[string]entity.ReadingNote)}
}

func (r *MemoryRepo) Store(_ context.Context, note entity.ReadingNote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[note.ID] = note
	return nil
}

func (r *MemoryRepo) Update(_ context.Context, note entity.ReadingNote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[note.ID]; !ok {
		return ErrNoteNotFound
	}
	r.items[note.ID] = note
	return nil
}

func (r *MemoryRepo) Get(_ context.Context, id string) (entity.ReadingNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	note, ok := r.items[id]
	if !ok {
		return entity.ReadingNote{}, fmt.Errorf("MemoryRepo - Get - note not found")
	}
	return note, nil
}

func (r *MemoryRepo) List(_ context.Context, limit int) ([]entity.ReadingNote, error) {
	if limit <= 0 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]entity.ReadingNote, 0, len(r.items))
	for _, note := range r.items {
		all = append(all, note)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (r *MemoryRepo) ListByDocument(_ context.Context, documentID string, limit int) ([]entity.ReadingNote, error) {
	if limit <= 0 {
		limit = 100
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]entity.ReadingNote, 0)
	for _, note := range r.items {
		if note.DocumentID == documentID {
			all = append(all, note)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
