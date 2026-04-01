package web

import (
	"context"
	"errors"
	"testing"

	"github.com/vanadium23/kompanion/internal/entity"
)

type progressStub struct {
	byDoc map[string]entity.Progress
	errs  map[string]error
}

func (p progressStub) Sync(context.Context, entity.Progress) (entity.Progress, error) {
	return entity.Progress{}, nil
}

func (p progressStub) Fetch(_ context.Context, bookID string) (entity.Progress, error) {
	if err, ok := p.errs[bookID]; ok {
		return entity.Progress{}, err
	}
	if progress, ok := p.byDoc[bookID]; ok {
		return progress, nil
	}
	return entity.Progress{}, nil
}

type loggerStub struct{}

func (loggerStub) Debug(interface{}, ...interface{}) {}
func (loggerStub) Info(string, ...interface{})       {}
func (loggerStub) Warn(string, ...interface{})       {}
func (loggerStub) Error(interface{}, ...interface{}) {}
func (loggerStub) Fatal(interface{}, ...interface{}) {}

func TestFetchBooksWithProgress_KeepOrderAndFallbackOnError(t *testing.T) {
	books := []entity.Book{
		{ID: "b-1", DocumentID: "doc-1"},
		{ID: "b-2", DocumentID: "doc-2"},
	}

	result := fetchBooksWithProgress(context.Background(), books, progressStub{
		byDoc: map[string]entity.Progress{
			"doc-1": {Percentage: 0.37},
		},
		errs: map[string]error{
			"doc-2": errors.New("fetch failed"),
		},
	}, loggerStub{})

	if len(result) != 2 {
		t.Fatalf("unexpected result len: %d", len(result))
	}
	if result[0].ID != "b-1" || result[0].Progress != 37 {
		t.Fatalf("unexpected first result: %+v", result[0])
	}
	if result[1].ID != "b-2" || result[1].Progress != 0 {
		t.Fatalf("unexpected second result: %+v", result[1])
	}
}
