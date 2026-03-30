package notes

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/vanadium23/kompanion/internal/entity"
	"github.com/vanadium23/kompanion/pkg/postgres"
)

type PostgresRepo struct {
	*postgres.Postgres
}

func NewPostgresRepo(pg *postgres.Postgres) *PostgresRepo {
	return &PostgresRepo{Postgres: pg}
}

func (r *PostgresRepo) Store(ctx context.Context, note entity.ReadingNote) error {
	query := `INSERT INTO joplin_note
		(id, title, body_md, document_id, source, source_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.Pool.Exec(ctx, query, note.ID, note.Title, note.Body, note.DocumentID, note.Source, note.SourceURL, note.CreatedAt, note.UpdatedAt)
	if err != nil {
		return fmt.Errorf("PostgresRepo - Store - r.Pool.Exec: %w", err)
	}
	return nil
}

func (r *PostgresRepo) Update(ctx context.Context, note entity.ReadingNote) error {
	query := `UPDATE joplin_note
		SET title = $1, body_md = $2, document_id = $3, source = $4, source_url = $5, updated_at = $6
		WHERE id = $7`
	res, err := r.Pool.Exec(ctx, query, note.Title, note.Body, note.DocumentID, note.Source, note.SourceURL, note.UpdatedAt, note.ID)
	if err != nil {
		return fmt.Errorf("PostgresRepo - Update - r.Pool.Exec: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func (r *PostgresRepo) Get(ctx context.Context, id string) (entity.ReadingNote, error) {
	query := `SELECT id, title, body_md, document_id, source, source_url, created_at, updated_at
		FROM joplin_note WHERE id = $1`
	var note entity.ReadingNote
	if err := r.Pool.QueryRow(ctx, query, id).Scan(&note.ID, &note.Title, &note.Body, &note.DocumentID, &note.Source, &note.SourceURL, &note.CreatedAt, &note.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.ReadingNote{}, ErrNoteNotFound
		}
		return entity.ReadingNote{}, fmt.Errorf("PostgresRepo - Get - r.Pool.QueryRow: %w", err)
	}
	return note, nil
}

func (r *PostgresRepo) List(ctx context.Context, limit int) ([]entity.ReadingNote, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, title, body_md, document_id, source, source_url, created_at, updated_at
		FROM joplin_note ORDER BY created_at DESC LIMIT $1`
	rows, err := r.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("PostgresRepo - List - r.Pool.Query: %w", err)
	}
	defer rows.Close()

	notes := make([]entity.ReadingNote, 0, limit)
	for rows.Next() {
		var note entity.ReadingNote
		if err := rows.Scan(&note.ID, &note.Title, &note.Body, &note.DocumentID, &note.Source, &note.SourceURL, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PostgresRepo - List - rows.Scan: %w", err)
		}
		notes = append(notes, note)
	}
	return notes, nil
}

func (r *PostgresRepo) ListByDocument(ctx context.Context, documentID string, limit int) ([]entity.ReadingNote, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT id, title, body_md, document_id, source, source_url, created_at, updated_at
		FROM joplin_note WHERE document_id = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := r.Pool.Query(ctx, query, documentID, limit)
	if err != nil {
		return nil, fmt.Errorf("PostgresRepo - ListByDocument - r.Pool.Query: %w", err)
	}
	defer rows.Close()

	notes := make([]entity.ReadingNote, 0, limit)
	for rows.Next() {
		var note entity.ReadingNote
		if err := rows.Scan(&note.ID, &note.Title, &note.Body, &note.DocumentID, &note.Source, &note.SourceURL, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, fmt.Errorf("PostgresRepo - ListByDocument - rows.Scan: %w", err)
		}
		notes = append(notes, note)
	}
	return notes, nil
}

func (r *PostgresRepo) Delete(ctx context.Context, id string) error {
	res, err := r.Pool.Exec(ctx, `DELETE FROM joplin_note WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("PostgresRepo - Delete - r.Pool.Exec: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNoteNotFound
	}
	return nil
}
