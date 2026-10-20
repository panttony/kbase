package indexing

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func BenchmarkIndexing(b *testing.B) {
	idx, err := New("")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer idx.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.Add(context.Background(), benchDoc(i)); err != nil {
			b.Fatalf("Add: %v", err)
		}
	}
}

func BenchmarkSearchBM25(b *testing.B) {
	idx, err := New("")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer idx.Close()

	const docs = 1000
	for i := 0; i < docs; i++ {
		if err := idx.Add(context.Background(), benchDoc(i)); err != nil {
			b.Fatalf("Add: %v", err)
		}
	}

	ctx := context.Background()
	method, err := NewScoringMethod("bm25")
	if err != nil {
		b.Fatalf("NewScoringMethod: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := idx.Search(ctx, Query{Text: "golang searching", Size: 20})
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if _, err := method.Rescore(ctx, idx, "golang searching", res.Hits); err != nil {
			b.Fatalf("Rescore: %v", err)
		}
	}
}

func BenchmarkSearchTFIDF(b *testing.B) {
	idx, err := New("")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer idx.Close()

	const docs = 1000
	for i := 0; i < docs; i++ {
		if err := idx.Add(context.Background(), benchDoc(i)); err != nil {
			b.Fatalf("Add: %v", err)
		}
	}

	ctx := context.Background()
	method, err := NewScoringMethod("tfidf")
	if err != nil {
		b.Fatalf("NewScoringMethod: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := idx.Search(ctx, Query{Text: "golang searching", Size: 20})
		if err != nil {
			b.Fatalf("Search: %v", err)
		}
		if _, err := method.Rescore(ctx, idx, "golang searching", res.Hits); err != nil {
			b.Fatalf("Rescore: %v", err)
		}
	}
}
