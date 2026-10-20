package indexing

import (
	"context"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"

	_ "github.com/blevesearch/bleve/v2/search/highlight/fragmenter/simple"
	_ "github.com/blevesearch/bleve/v2/search/highlight/highlighter/html"
	"github.com/panttony/kbase/internal/model"
)

type Hit struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Score     float64
	Snippet   string
	TermCounts map[string]float64
}

type Result struct {
	Hits  []Hit
	Total uint64
}

type Query struct {
	Text     string
	Size     int
	Tags     []string
	Category string
	From time.Time
	To   time.Time
}

type StatsProvider interface {
	DocCount() (uint64, error)
	DocFreq(term string) (uint64, error)
}

type Indexer interface {
	StatsProvider
	Add(ctx context.Context, doc model.Document) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, q Query) (*Result, error)
	Close() error
}

type BleveIndexer struct {
	idx bleve.Index
}

func New(path string) (*BleveIndexer, error) {
	m, err := newMapping()
	if err != nil {
		return nil, fmt.Errorf("build mapping: %w", err)
	}
	var (
		idx bleve.Index
	)
	if path == "" {
		idx, err = bleve.NewMemOnly(m)
	} else {
		idx, err = bleve.New(path, m)
	}
	if err != nil {
		return nil, fmt.Errorf("open index at %q: %w", path, err)
	}
	return &BleveIndexer{idx: idx}, nil
}

func (x *BleveIndexer) Close() error {
	return x.idx.Close()
}

func (x *BleveIndexer) Add(ctx context.Context, doc model.Document) error {
	indexed := indexedDocument{
		ID:        doc.ID.String(),
		Title:     doc.Title,
		Text:      doc.PlainText,
		Tags:      doc.Tags,
		Category:  doc.Category,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}
	if err := x.idx.Index(doc.ID.String(), indexed); err != nil {
		return fmt.Errorf("index document %s: %w", doc.ID, err)
	}
	return nil
}

func (x *BleveIndexer) Delete(ctx context.Context, id string) error {
	if err := x.idx.Delete(id); err != nil {
		return fmt.Errorf("delete from index %s: %w", id, err)
	}
	return nil
}

func (x *BleveIndexer) DocCount() (uint64, error) {
	n, err := x.idx.DocCount()
	if err != nil {
		return 0, fmt.Errorf("doc count: %w", err)
	}
	return n, nil
}

func (x *BleveIndexer) Search(ctx context.Context, q Query) (*Result, error) {
	var root query.Query
	if q.Text == "" {
		root = bleve.NewMatchAllQuery()
	} else {
		textMatch := bleve.NewMatchQuery(q.Text)
		textMatch.SetField(fieldText)
		titleMatch := bleve.NewMatchQuery(q.Text)
		titleMatch.SetField(fieldTitle)
		titleMatch.SetBoost(2.0)
		root = bleve.NewDisjunctionQuery(textMatch, titleMatch)
	}

	filters := buildFilters(q)
	if len(filters) > 0 {
		root = bleve.NewConjunctionQuery(append([]query.Query{root}, filters...)...)
	}

	req := bleve.NewSearchRequestOptions(root, q.Size, 0, false)
	req.IncludeLocations = true
	req.Fields = []string{fieldTitle, fieldUpdated, fieldCreated}
	req.Highlight = bleve.NewHighlightWithStyle("html")
	req.Highlight.AddField(fieldText)

	sr, err := x.idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("index search %q: %w", q.Text, err)
	}

	res := &Result{Total: sr.Total}
	for _, h := range sr.Hits {
		res.Hits = append(res.Hits, fromMatch(h))
	}
	return res, nil
}

func buildFilters(q Query) []query.Query {
	var filters []query.Query
	if len(q.Tags) > 0 {
		var tags []query.Query
		for _, tag := range q.Tags {
			tq := bleve.NewTermQuery(tag)
			tq.SetField(fieldTags)
			tags = append(tags, tq)
		}
		filters = append(filters, bleve.NewDisjunctionQuery(tags...))
	}
	if q.Category != "" {
		cq := bleve.NewTermQuery(q.Category)
		cq.SetField(fieldCategory)
		filters = append(filters, cq)
	}
	if !q.From.IsZero() || !q.To.IsZero() {
		dq := bleve.NewDateRangeQuery(q.From, q.To)
		dq.SetField(fieldUpdated)
		filters = append(filters, dq)
	}
	return filters
}

func fromMatch(h *search.DocumentMatch) Hit {
	hit := Hit{
		ID:         h.ID,
		Score:      h.Score,
		TermCounts: make(map[string]float64),
	}
	if v, ok := h.Fields[fieldTitle].(string); ok {
		hit.Title = v
	}
	if v, ok := h.Fields[fieldUpdated].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			hit.UpdatedAt = t
		}
	}
	if v, ok := h.Fields[fieldCreated].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			hit.CreatedAt = t
		}
	}
	for field, terms := range h.Locations {
		weight, scoreable := tfidfFieldWeight[field]
		if !scoreable {
			continue
		}
		for term, locs := range terms {
			hit.TermCounts[term] += weight * float64(len(locs))
		}
	}
	if frags := h.Fragments[fieldText]; len(frags) > 0 {
		hit.Snippet = frags[0]
	}
	return hit
}
