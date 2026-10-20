package indexing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/model"
)

func newTestIndex(t *testing.T) *BleveIndexer {
	t.Helper()
	idx, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func testDoc(id, title, body string, tags []string) model.Document {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return model.Document{
		ID:        mustUUID(id),
		Title:     title,
		Category:  "test",
		Tags:      tags,
		Body:      body,
		PlainText: body,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
}

func mustUUID(seed string) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("%032x", seed))
}

func addAll(t *testing.T, idx *BleveIndexer, docs ...model.Document) {
	t.Helper()
	for _, d := range docs {
		if err := idx.Add(context.Background(), d); err != nil {
			t.Fatalf("Add(%s): %v", d.Title, err)
		}
	}
}

func searchAll(t *testing.T, idx *BleveIndexer, q string) *Result {
	t.Helper()
	res, err := idx.Search(context.Background(), Query{Text: q, Size: 100})
	if err != nil {
		t.Fatalf("Search(%q): %v", q, err)
	}
	return res
}

func TestIndexAndSearchBM25(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx,
		testDoc("1", "Go Networking", "The net package provides portable network interfaces.", nil),
		testDoc("2", "Redis Caching", "Redis stores query results to speed up repeated searches.", nil),
		testDoc("3", "Bleve Notes", "Bleve is a full-text search engine written in Go.", nil),
	)

	res := searchAll(t, idx, "go")
	if res.Total == 0 {
		t.Fatal("expected at least one hit for 'go'")
	}
	if got, want := res.Hits[0].Title, "Go Networking"; !strings.HasPrefix(got, want) {
		t.Errorf("top hit = %q, want %q", got, want)
	}
	if got := res.Hits[0].Score; got <= 0 {
		t.Errorf("top score = %v, want > 0", got)
	}
}

func TestSearchResultCarriesTermCounts(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx, testDoc("1", "Title", "alpha beta alpha", nil))

	res := searchAll(t, idx, "alpha")
	if len(res.Hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(res.Hits))
	}
	if got, want := res.Hits[0].TermCounts["alpha"], 2.0; got != want {
		t.Errorf("term count alpha = %v, want %v", got, want)
	}
}

func TestTitleTermCountDoubleWeight(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx, testDoc("1", "alpha title", "beta", nil))

	res := searchAll(t, idx, "alpha")
	if len(res.Hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(res.Hits))
	}
	if got, want := res.Hits[0].TermCounts["alpha"], 2.0; got != want {
		t.Errorf("title term count alpha = %v, want %v (2x weight)", got, want)
	}
}

func TestIndexUpdateAndDelete(t *testing.T) {
	idx := newTestIndex(t)
	doc := testDoc("1", "Original Title", "original body", nil)
	addAll(t, idx, doc)

	doc.Title = "Updated Title"
	doc.PlainText = "updated body"
	addAll(t, idx, doc)

	if res := searchAll(t, idx, "original"); res.Total != 0 {
		t.Errorf("old content still indexed: %+v", res.Hits)
	}
	if res := searchAll(t, idx, "updated"); res.Total != 1 {
		t.Errorf("updated content missing: %d hits", res.Total)
	}

	if err := idx.Delete(context.Background(), doc.ID.String()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if res := searchAll(t, idx, "updated"); res.Total != 0 {
		t.Errorf("deleted doc still found: %d hits", res.Total)
	}
}

func TestDocCountAndFreq(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx,
		testDoc("1", "Alpha", "zephyr common", nil),
		testDoc("2", "Beta", "common common", nil),
		testDoc("3", "Gamma", "common", nil),
	)

	n, err := idx.DocCount()
	if err != nil {
		t.Fatalf("DocCount: %v", err)
	}
	if n != 3 {
		t.Errorf("DocCount = %d, want 3", n)
	}

	freq, err := idx.DocFreq("common")
	if err != nil {
		t.Fatalf("DocFreq: %v", err)
	}
	if freq != 3 {
		t.Errorf("df(common) = %d, want 3", freq)
	}
	freq, err = idx.DocFreq("zephyr")
	if err != nil {
		t.Fatalf("DocFreq: %v", err)
	}
	if freq != 1 {
		t.Errorf("df(zephyr) = %d, want 1", freq)
	}
}

func TestSearchFiltersByTagsAndCategory(t *testing.T) {
	idx := newTestIndex(t)
	docs := []struct {
		id, title, body string
		tags            []string
		category        string
	}{
		{"1", "Go Networking", "net package interfaces", []string{"go", "backend"}, "backend"},
		{"2", "Redis Caching", "redis stores queries", []string{"cache", "backend"}, "backend"},
		{"3", "Bleve Notes", "bleve search engine", []string{"go", "search"}, "search"},
	}
	for _, d := range docs {
		doc := testDoc(d.id, d.title, d.body, d.tags)
		doc.Category = d.category
		addAll(t, idx, doc)
	}

	res, err := idx.Search(context.Background(), Query{Text: "", Size: 100, Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("tag filter search: %v", err)
	}
	if got := res.Total; got != 2 {
		t.Errorf("hits for tag go = %d, want 2", got)
	}

	res, err = idx.Search(context.Background(), Query{Text: "", Size: 100, Category: "search"})
	if err != nil {
		t.Fatalf("category filter search: %v", err)
	}
	if got := res.Total; got != 1 {
		t.Errorf("hits for category search = %d, want 1", got)
	}
	if got, want := res.Hits[0].ID, mustUUID("3").String(); got != want {
		t.Errorf("category hit = %s, want %s", got, want)
	}
}

func TestSearchDateRangeFilter(t *testing.T) {
	idx := newTestIndex(t)
	old := testDoc("1", "Old Doc", "ancient notes", nil)
	old.UpdatedAt = time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	recent := testDoc("2", "Recent Doc", "fresh notes", nil)
	recent.UpdatedAt = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	addAll(t, idx, old, recent)

	res, err := idx.Search(context.Background(), Query{
		Text: "", Size: 100,
		From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("date filter search: %v", err)
	}
	if len(res.Hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(res.Hits))
	}
	if got, want := res.Hits[0].ID, mustUUID("2").String(); got != want {
		t.Errorf("date-filtered hit = %s, want %s", got, want)
	}
}

func TestSearchEmptyTextReturnsAll(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx,
		testDoc("1", "Alpha", "body one", nil),
		testDoc("2", "Beta", "body two", nil),
		testDoc("3", "Gamma", "body three", nil),
	)

	res := searchAll(t, idx, "")
	if got := res.Total; got != 3 {
		t.Errorf("hits without text = %d, want 3", got)
	}
}

func TestSearchProducesHighlightedSnippet(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx, testDoc("1", "Postgres Tuning", "The planner picks an efficient plan for the query.", nil))

	res := searchAll(t, idx, "planner")
	if len(res.Hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(res.Hits))
	}
	hit := res.Hits[0]
	if hit.Snippet == "" {
		t.Fatal("expected a snippet, got empty")
	}
	if !strings.Contains(hit.Snippet, "<mark>") {
		t.Errorf("snippet has no highlight: %q", hit.Snippet)
	}
	if !strings.Contains(hit.Snippet, "planner") {
		t.Errorf("snippet misses the matched term: %q", hit.Snippet)
	}
}

func TestTFIDFRanksRareTerm(t *testing.T) {
	idx := newTestIndex(t)
	for i := 0; i < 6; i++ {
		addAll(t, idx, testDoc(itoa(i), "Common Doc", "publication common language", nil))
	}
	addAll(t, idx, testDoc("x", "Rare Doc", "zephyr publication", nil))

	res := searchAll(t, idx, "publication zephyr")
	method, err := NewScoringMethod("tfidf")
	if err != nil {
		t.Fatalf("NewScoringMethod: %v", err)
	}
	rescored, err := method.Rescore(context.Background(), idx, "publication zephyr", res.Hits)
	if err != nil {
		t.Fatalf("Rescore: %v", err)
	}
	if got, want := rescored[0].ID, mustUUID("x").String(); got != want {
		t.Errorf("tfidf top hit = %s, want the zephyr document", got)
	}
	for _, h := range rescored {
		if h.Score < 0 || math.IsNaN(h.Score) {
			t.Errorf("bad score %v for %s", h.Score, h.ID)
		}
	}
}

func TestTFIDFAndBM25DifferOnTermFrequency(t *testing.T) {
	idx := newTestIndex(t)
	addAll(t, idx,
		testDoc("a", "Short", "needle", nil),
		testDoc("b", "Repeated", "needle needle needle needle needle needle needle needle needle needle needle needle needle needle needle needle needle needle needle", nil),
	)

	res := searchAll(t, idx, "needle")
	tfidf, _ := NewScoringMethod("tfidf")
	bm25, _ := NewScoringMethod("bm25")

	tfidfHits, err := tfidf.Rescore(context.Background(), idx, "needle", res.Hits)
	if err != nil {
		t.Fatalf("tfidf rescore: %v", err)
	}
	bm25Hits, err := bm25.Rescore(context.Background(), idx, "needle", res.Hits)
	if err != nil {
		t.Fatalf("bm25 rescore: %v", err)
	}

	if got, want := tfidfHits[0].ID, mustUUID("b").String(); got != want {
		t.Errorf("tfidf top hit = %s, want %s (term density wins)", got, want)
	}
	tfidfSet := idsOf(tfidfHits)
	for _, h := range bm25Hits {
		if _, ok := tfidfSet[h.ID]; !ok {
			t.Errorf("bm25 returned unknown hit %s", h.ID)
		}
	}
}

func TestScoringFactory(t *testing.T) {
	for _, name := range []string{"", "bm25", "BM25", "tfidf"} {
		if _, err := NewScoringMethod(name); err != nil {
			t.Errorf("NewScoringMethod(%q): %v", name, err)
		}
	}
	if _, err := NewScoringMethod("lm"); err == nil {
		t.Error("expected error for unknown method")
	}
}

func idsOf(hits []Hit) map[string]struct{} {
	out := make(map[string]struct{}, len(hits))
	for _, h := range hits {
		out[h.ID] = struct{}{}
	}
	return out
}

func itoa(i int) string {
	return string(rune('0' + i))
}
