package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/model"
)

type ErrNotFound struct {
	Kind string // "document" or "version"
	ID   any
}

func (e ErrNotFound) Error() string {
	return "not found: " + e.Kind + " " + fmt.Sprint(e.ID)
}

type Repo interface {
	Create(ctx context.Context, doc model.Document) (model.Document, error)

	Get(ctx context.Context, id uuid.UUID) (model.Document, error)

	Update(ctx context.Context, doc model.Document) (model.Document, error)

	Delete(ctx context.Context, id uuid.UUID) error

	ListVersions(ctx context.Context, id uuid.UUID) ([]model.Version, error)

	GetVersion(ctx context.Context, documentID uuid.UUID, number int) (model.Version, error)

	Stats(ctx context.Context) (model.Stats, error)
}

var _ Repo = (*RepoPostgres)(nil)
