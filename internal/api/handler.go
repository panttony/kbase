package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/indexing"
	"github.com/panttony/kbase/internal/markdown"
	"github.com/panttony/kbase/internal/model"
	"github.com/panttony/kbase/internal/search"
	"github.com/panttony/kbase/internal/storage"
)

type searcher interface {
	Search(ctx context.Context, p search.Params) (*search.Result, error)
}

type Handler struct {
	repo     storage.Repo
	idx      indexing.Indexer
	svc      searcher
	cache    cache
	cacheTTL time.Duration
	log      *slog.Logger
}

func New(repo storage.Repo, idx indexing.Indexer, svc searcher, c cache, cacheTTL time.Duration, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{repo: repo, idx: idx, svc: svc, cache: c, cacheTTL: cacheTTL, log: log}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/documents", h.handleCreateDocument)
	mux.HandleFunc("GET /api/v1/documents/{id}", h.handleGetDocument)
	mux.HandleFunc("PUT /api/v1/documents/{id}", h.handleUpdateDocument)
	mux.HandleFunc("DELETE /api/v1/documents/{id}", h.handleDeleteDocument)
	mux.HandleFunc("GET /api/v1/documents/{id}/versions", h.handleListVersions)
	mux.HandleFunc("GET /api/v1/documents/{id}/versions/{number}", h.handleGetVersion)
	mux.HandleFunc("GET /api/v1/search", h.handleSearch)
	mux.HandleFunc("GET /api/v1/stats", h.handleStats)
	mux.HandleFunc("GET /api/v1/openapi.yaml", h.handleSpec)
	mux.HandleFunc("GET /docs/", h.handleDocs)

	return h.withRecover(h.withLog(mux))
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := r.PathValue(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, invalidRequest("invalid document id"))
		return uuid.Nil, false
	}
	return id, true
}

func process(raw string) (*markdown.Parsed, error) {
	p, err := markdown.Process([]byte(raw))
	if err != nil {
		return nil, &appError{status: http.StatusBadRequest, code: codeBadRequest, message: err.Error()}
	}
	return p, nil
}

func fromParsed(p *markdown.Parsed) model.Document {
	return model.Document{
		Title:     p.Title,
		Category:  p.Meta.Category,
		Tags:      p.Meta.Tags,
		Authors:   p.Meta.Authors,
		Body:      p.Body,
		PlainText: p.Plain,
	}
}
