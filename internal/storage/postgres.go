package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/panttony/kbase/internal/model"
)

type RepoPostgres struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, pool *pgxpool.Pool) (*RepoPostgres, error) {
	if err := migrate(ctx, pool); err != nil {
		return nil, err
	}
	return &RepoPostgres{pool: pool}, nil
}

func (r *RepoPostgres) Pool() *pgxpool.Pool { return r.pool }

func (r *RepoPostgres) Create(ctx context.Context, doc model.Document) (model.Document, error) {
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	if doc.Version < 1 {
		doc.Version = 1
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = nowUTC()
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}

	metaJSON, err := json.Marshal(docMeta(doc))
	if err != nil {
		return model.Document{}, fmt.Errorf("marshal meta: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Document{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO documents (id, title, category, body, source, created_at, updated_at, current_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		doc.ID, doc.Title, doc.Category, doc.Body, doc.Source,
		doc.CreatedAt, doc.UpdatedAt, int64(doc.Version)); err != nil {
		return model.Document{}, fmt.Errorf("insert document: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO versions (id, document_id, number, body, meta_snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), doc.ID, int64(doc.Version), doc.Body, metaJSON, doc.UpdatedAt); err != nil {
		return model.Document{}, fmt.Errorf("insert version: %w", err)
	}

	if err := r.replaceTags(ctx, tx, doc.ID, doc.Tags); err != nil {
		return model.Document{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Document{}, fmt.Errorf("commit: %w", err)
	}

	stored, err := r.Get(ctx, doc.ID)
	if err != nil {
		return model.Document{}, err
	}
	return stored, nil
}

func (r *RepoPostgres) Get(ctx context.Context, id uuid.UUID) (model.Document, error) {
	doc := model.Document{}
	row := r.pool.QueryRow(ctx, `
		SELECT d.id, d.title, d.category, d.body, d.source,
		       d.created_at, d.updated_at, d.current_version
		FROM documents d
		WHERE d.id = $1 AND d.deleted_at IS NULL`, id)

	if err := row.Scan(&doc.ID, &doc.Title, &doc.Category, &doc.Body, &doc.Source,
		&doc.CreatedAt, &doc.UpdatedAt, &doc.Version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Document{}, ErrNotFound{Kind: "document", ID: id}
		}
		return model.Document{}, fmt.Errorf("get document: %w", err)
	}

	tags, err := r.tagsFor(ctx, id)
	if err != nil {
		return model.Document{}, err
	}
	doc.Tags = tags
	return doc, nil
}

func (r *RepoPostgres) Update(ctx context.Context, doc model.Document) (model.Document, error) {
	current, err := r.Get(ctx, doc.ID)
	if err != nil {
		return model.Document{}, err
	}

	if doc.Source == "" {
		doc.Source = current.Source
	}
	version := current.Version + 1
	doc.ID = current.ID
	doc.CreatedAt = current.CreatedAt
	doc.UpdatedAt = nowUTC()
	doc.Version = version

	metaJSON, err := json.Marshal(docMeta(doc))
	if err != nil {
		return model.Document{}, fmt.Errorf("marshal meta: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Document{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO versions (id, document_id, number, body, meta_snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), doc.ID, int64(version), doc.Body, metaJSON, doc.UpdatedAt); err != nil {
		return model.Document{}, fmt.Errorf("insert version: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE documents
		SET title = $2, category = $3, body = $4, source = $5,
		    updated_at = $6, current_version = $7
		WHERE id = $1 AND deleted_at IS NULL`,
		doc.ID, doc.Title, doc.Category, doc.Body, doc.Source,
		doc.UpdatedAt, int64(version)); err != nil {
		return model.Document{}, fmt.Errorf("update document: %w", err)
	}

	if err := r.replaceTags(ctx, tx, doc.ID, doc.Tags); err != nil {
		return model.Document{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Document{}, fmt.Errorf("commit: %w", err)
	}

	stored, err := r.Get(ctx, doc.ID)
	if err != nil {
		return model.Document{}, err
	}
	return stored, nil
}

func (r *RepoPostgres) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE documents SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound{Kind: "document", ID: id}
	}
	return nil
}

func (r *RepoPostgres) ListVersions(ctx context.Context, id uuid.UUID) ([]model.Version, error) {
	if _, err := r.Get(ctx, id); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, document_id, number, body, meta_snapshot, created_at
		FROM versions WHERE document_id = $1 ORDER BY number ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []model.Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (r *RepoPostgres) GetVersion(ctx context.Context, documentID uuid.UUID, number int) (model.Version, error) {
	if _, err := r.Get(ctx, documentID); err != nil {
		return model.Version{}, err
	}
	row := r.pool.QueryRow(ctx, `
		SELECT id, document_id, number, body, meta_snapshot, created_at
		FROM versions WHERE document_id = $1 AND number = $2`,
		documentID, number)
	v, err := scanVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Version{}, ErrNotFound{Kind: "version", ID: fmt.Sprintf("%s/%d", documentID, number)}
		}
		return model.Version{}, err
	}
	return v, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVersion(row rowScanner) (model.Version, error) {
	var v model.Version
	var metaJSON []byte
	if err := row.Scan(&v.ID, &v.DocumentID, &v.Number, &v.Body, &metaJSON, &v.CreatedAt); err != nil {
		return model.Version{}, fmt.Errorf("scan version: %w", err)
	}
	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &v.Meta); err != nil {
			return model.Version{}, fmt.Errorf("unmarshal meta: %w", err)
		}
	}
	return v, nil
}

func (r *RepoPostgres) replaceTags(ctx context.Context, tx pgx.Tx, documentID uuid.UUID, tags []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM document_tags WHERE document_id = $1`, documentID); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	for _, name := range tags {
		if name == "" {
			continue
		}
		var tagID uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO tags (id, name) VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, uuid.New(), name).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("upsert tag %q: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO document_tags (document_id, tag_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, documentID, tagID); err != nil {
			return fmt.Errorf("link tag %q: %w", name, err)
		}
	}
	return nil
}

func (r *RepoPostgres) tagsFor(ctx context.Context, documentID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.name
		FROM tags t
		JOIN document_tags dt ON dt.tag_id = t.id
		WHERE dt.document_id = $1
		ORDER BY t.name`, documentID)
	if err != nil {
		return nil, fmt.Errorf("tags for document: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

func (r *RepoPostgres) Stats(ctx context.Context) (model.Stats, error) {
	var s model.Stats
	row := r.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(octet_length(body)), 0)
		FROM documents WHERE deleted_at IS NULL`)
	if err := row.Scan(&s.Documents, &s.TotalSize); err != nil {
		return model.Stats{}, fmt.Errorf("stats: %w", err)
	}
	return s, nil
}

func docMeta(doc model.Document) model.Meta {
	return model.Meta{
		Title:    doc.Title,
		Tags:     doc.Tags,
		Category: doc.Category,
		Authors:  doc.Authors,
	}
}

func nowUTC() time.Time { return time.Now().UTC() }
