package search

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/panttony/kbase/internal/indexing"
	"github.com/panttony/kbase/internal/model"
)

func benchDoc(i int) model.Document {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return model.Document{
		ID:        mustUUID(fmt.Sprintf("doc%d", i)),
		Title:     fmt.Sprintf("Document number %d", i),
		Category:  "bench",
		Tags:      []string{"bench", fmt.Sprintf("tag%d", i%10)},
		Body:      fmt.Sprintf("Benchmark document %d with content about golang and searching and indexing and relevance scoring. document %d body content.", i, i),
		PlainText: fmt.Sprintf("Benchmark document %d with content about golang and searching and indexing and relevance scoring. document %d body content.", i, i),
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
}

func BenchmarkSearchService(b *testing.B) {
	idx, err := indexing.New("")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer idx.Close()
	svc := New(idx)

	const docs = 1000
	for i := 0; i < docs; i++ {
		if err := idx.Add(context.Background(), benchDoc(i)); err != nil {
			b.Fatalf("Add: %v", err)
		}
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := svc.Search(ctx, Params{Text: "golang searching", Method: "tfidf", PageSize: 20})
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if len(res.Hits) != 20 {
			b.Fatalf("hits = %d, want 20", len(res.Hits))
		}
	}
}
