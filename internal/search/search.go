package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/indexing"
	"github.com/panttony/kbase/internal/model"
)

type Params struct {
	Text     string
	Method   string // "" | "bm25" | "tfidf" ("" = bm25)
	Tags     []string
	Category string
	From     time.Time // zero value = open bound
	To       time.Time // zero value = open bound
	Sort     string    // "relevance" (default) | "title" | "created_at" | "updated_at"
	Page     int       // 1-based, default 1
	PageSize int       // default 20, max 100
}

type Result struct {
	Query    string      `json:"query"`
	Method   string      `json:"method"`
	Total    uint64      `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Hits     []model.Hit `json:"hits"`
}

type Service struct {
	idx indexing.Indexer
}

func New(idx indexing.Indexer) *Service {
	return &Service{idx: idx}
}

func (s *Service) Search(ctx context.Context, p Params) (*Result, error) {
	p.normalize()
	if err := p.validate(); err != nil {
		return nil, paramError("%s", err.Error())
	}

	method, err := indexing.NewScoringMethod(p.Method)
	if err != nil {
		return nil, paramError("%s", err.Error())
	}

	window := p.Page * p.PageSize
	res, err := s.idx.Search(ctx, indexing.Query{
		Text:     p.Text,
		Size:     window,
		Tags:     p.Tags,
		Category: p.Category,
		From:     p.From,
		To:       p.To,
	})
	if err != nil {
		return nil, fmt.Errorf("index search: %w", err)
	}

	hits, err := method.Rescore(ctx, s.idx, p.Text, res.Hits)
	if err != nil {
		return nil, fmt.Errorf("re-score with %s: %w", method.Name(), err)
	}

	switch p.Sort {
	case "title":
		sort.SliceStable(hits, func(i, j int) bool {
			return strings.ToLower(hits[i].Title) < strings.ToLower(hits[j].Title)
		})
	case "created_at":
		sort.SliceStable(hits, func(i, j int) bool {
			return hits[i].CreatedAt.After(hits[j].CreatedAt)
		})
	case "updated_at":
		sort.SliceStable(hits, func(i, j int) bool {
			return hits[i].UpdatedAt.After(hits[j].UpdatedAt)
		})
	}

	offset := (p.Page - 1) * p.PageSize
	if offset > len(hits) {
		offset = len(hits)
	}
	end := offset + p.PageSize
	if end > len(hits) {
		end = len(hits)
	}

	out := &Result{
		Query:    p.Text,
		Method:   method.Name(),
		Total:    res.Total,
		Page:     p.Page,
		PageSize: p.PageSize,
		Hits:     make([]model.Hit, 0, end-offset),
	}
	for _, h := range hits[offset:end] {
		id, err := uuid.Parse(h.ID)
		if err != nil {
			continue
		}
		snippet := h.Snippet
		if snippet == "" {
			snippet = h.Title
		}
		out.Hits = append(out.Hits, model.Hit{
			DocumentID: id,
			Title:      h.Title,
			Score:      h.Score,
			Snippet:    snippet,
			UpdatedAt:  h.UpdatedAt,
		})
	}
	return out, nil
}
