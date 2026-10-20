package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/model"
	"github.com/panttony/kbase/internal/search"
)

type documentRequest struct {
	Content string `json:"content"`
}

type documentResponse struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Authors   []string  `json:"authors"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

type metaResponse struct {
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
	Authors  []string `json:"authors"`
}

type versionResponse struct {
	ID         uuid.UUID    `json:"id"`
	DocumentID uuid.UUID    `json:"document_id"`
	Number     int          `json:"number"`
	Body       string       `json:"body"`
	Meta       metaResponse `json:"meta"`
	CreatedAt  time.Time    `json:"created_at"`
}

type hitResponse struct {
	DocumentID uuid.UUID `json:"document_id"`
	Title      string    `json:"title"`
	Score      float64   `json:"score"`
	Snippet    string    `json:"snippet"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type searchResponse struct {
	Query    string        `json:"query"`
	Method   string        `json:"method"`
	Total    uint64        `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Hits     []hitResponse `json:"hits"`
}

type statsResponse struct {
	Documents int   `json:"documents"`
	TotalSize int64 `json:"total_size"`
}

func toDocumentResponse(d model.Document) documentResponse {
	return documentResponse{
		ID:        d.ID,
		Title:     d.Title,
		Category:  d.Category,
		Tags:      d.Tags,
		Authors:   d.Authors,
		Body:      d.Body,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
		Version:   d.Version,
	}
}

func toVersionResponse(v model.Version) versionResponse {
	return versionResponse{
		ID:         v.ID,
		DocumentID: v.DocumentID,
		Number:     v.Number,
		Body:       v.Body,
		Meta: metaResponse{
			Title:    v.Meta.Title,
			Tags:     v.Meta.Tags,
			Category: v.Meta.Category,
			Authors:  v.Meta.Authors,
		},
		CreatedAt: v.CreatedAt,
	}
}

func toSearchResponse(r *search.Result) searchResponse {
	out := searchResponse{
		Query:    r.Query,
		Method:   r.Method,
		Total:    r.Total,
		Page:     r.Page,
		PageSize: r.PageSize,
		Hits:     make([]hitResponse, 0, len(r.Hits)),
	}
	for _, h := range r.Hits {
		out.Hits = append(out.Hits, hitResponse{
			DocumentID: h.DocumentID,
			Title:      h.Title,
			Score:      h.Score,
			Snippet:    h.Snippet,
			UpdatedAt:  h.UpdatedAt,
		})
	}
	return out
}

func toStatsResponse(s model.Stats) statsResponse {
	return statsResponse{Documents: s.Documents, TotalSize: s.TotalSize}
}
