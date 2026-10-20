package indexing

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

type ScoringMethod interface {
	Name() string
	Rescore(ctx context.Context, sp StatsProvider, query string, hits []Hit) ([]Hit, error)
}

func NewScoringMethod(name string) (ScoringMethod, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "bm25":
		return &BM25Method{}, nil
	case "tfidf":
		return &TFIDFMethod{}, nil
	default:
		return nil, fmt.Errorf("unknown scoring method %q", name)
	}
}

type BM25Method struct{}

func (m *BM25Method) Name() string { return "bm25" }

func (m *BM25Method) Rescore(_ context.Context, _ StatsProvider, _ string, hits []Hit) ([]Hit, error) {
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	return hits, nil
}

var tfidfFieldWeight = map[string]float64{fieldTitle: 2.0, fieldText: 1.0}


//	score(d, q) = Σ_{t ∈ q ∩ d} (1 + ln tf(t, d)) · idf(t)

type TFIDFMethod struct{}

func (m *TFIDFMethod) Name() string { return "tfidf" }

func (m *TFIDFMethod) Rescore(ctx context.Context, sp StatsProvider, query string, hits []Hit) ([]Hit, error) {
	if len(hits) == 0 {
		return hits, nil
	}
	total, err := sp.DocCount()
	if err != nil {
		return nil, fmt.Errorf("tfidf: doc count: %w", err)
	}

	terms := tokenize(query)
	idf := make(map[string]float64, len(terms))
	for _, term := range terms {
		if _, seen := idf[term]; seen {
			continue
		}
		freq, err := sp.DocFreq(term)
		if err != nil {
			return nil, fmt.Errorf("tfidf: doc freq %q: %w", term, err)
		}
		idf[term] = math.Log(1 + float64(total)/float64(max(1, freq)))
	}

	for i := range hits {
		var score float64
		for _, term := range terms {
			f, ok := hits[i].TermCounts[term]
			if !ok || f <= 0 {
				continue
			}
			score += (1 + math.Log(f)) * idf[term]
		}
		hits[i].Score = score
	}

	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	return hits, nil
}
