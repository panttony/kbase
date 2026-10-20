package search

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/indexing"
	"github.com/panttony/kbase/internal/model"
)


func newTestIndex(t *testing.T) *indexing.BleveIndexer {
	t.Helper()
	idx, err := indexing.New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func mustUUID(seed string) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("%032x", seed))
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

func addAll(t *testing.T, idx *indexing.BleveIndexer, docs ...model.Document) {
	t.Helper()
	for _, d := range docs {
		if err := idx.Add(context.Background(), d); err != nil {
			t.Fatalf("Add(%s): %v", d.Title, err)
		}
	}
}

func hitIDs(hits []model.Hit) []string {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.DocumentID.String())
	}
	return ids
}

func hitTitles(hits []model.Hit) []string {
	titles := make([]string, 0, len(hits))
	for _, h := range hits {
		titles = append(titles, h.Title)
	}
	return titles
}

func TestValidateDefaults(t *testing.T) {
	var p Params
	p.normalize()
	if p.Page != 1 {
		t.Errorf("Page = %d, want 1", p.Page)
	}
	if p.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", p.PageSize)
	}
	if p.Method != "bm25" {
		t.Errorf("Method = %q, want bm25", p.Method)
	}
	if p.Sort != "relevance" {
		t.Errorf("Sort = %q, want relevance", p.Sort)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name   string
		params Params
		want   string
	}{
		{"page zero", Params{Page: 0}, "page must be >= 1"},
		{"page negative", Params{Page: -1}, "page must be >= 1"},
		{"page size below min", Params{Page: 1, PageSize: 0}, "page_size must be between 1 and 100"},
		{"page size above max", Params{Page: 1, PageSize: 101}, "page_size must be between 1 and 100"},
		{"unknown method", Params{Page: 1, PageSize: 20, Method: "lm"}, "unknown scoring method"},
		{"unknown sort", Params{Page: 1, PageSize: 20, Sort: "foo"}, "unknown sort"},
		{
			"from after to",
			Params{
				Page:     1,
				PageSize: 20,
				Sort:     "relevance",
				From:     time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
				To:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			"from must not be after to",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("validate(%+v) = %v, want error containing %q", tc.params, err, tc.want)
			}
		})
	}
}

func TestPagination(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)
	for i := 0; i < 7; i++ {
		addAll(t, idx, testDoc(fmt.Sprintf("%d", i), fmt.Sprintf("Doc %d", i), "shared body text", nil))
	}

	pageIDs := func(page int) []string {
		t.Helper()
		res, err := svc.Search(context.Background(), Params{Page: page, PageSize: 3})
		if err != nil {
			t.Fatalf("Search(page=%d): %v", page, err)
		}
		if res.Page != page || res.PageSize != 3 {
			t.Errorf("response page meta = %d/%d, want %d/3", res.Page, res.PageSize, page)
		}
		if res.Total != 7 {
			t.Errorf("total = %d, want 7", res.Total)
		}
		return hitIDs(res.Hits)
	}

	seen := make(map[string]int, 7)
	for _, page := range []int{1, 2, 3} {
		for _, id := range pageIDs(page) {
			if _, dup := seen[id]; dup {
				t.Errorf("id %s appears on more than one page", id)
			}
			seen[id]++
		}
	}
	if len(seen) != 7 {
		t.Errorf("distinct ids across pages = %d, want 7", len(seen))
	}
	if got := pageIDs(4); len(got) != 0 {
		t.Errorf("page beyond the end has %d hits, want 0", len(got))
	}
}

func TestPaginationPreservesRelevanceOrder(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)
	addAll(t, idx,
		testDoc("a", "Short", "needle", nil),
		testDoc("b", "Repeated", strings.Repeat("needle ", 19), nil),
		testDoc("c", "Mid", strings.Repeat("needle ", 3), nil),
		testDoc("d", "Few", strings.Repeat("needle ", 2), nil),
	)

	ctx := context.Background()
	full, err := svc.Search(ctx, Params{Text: "needle", PageSize: 100})
	if err != nil {
		t.Fatalf("Search full: %v", err)
	}

	var paged []string
	for page := 1; ; page++ {
		res, err := svc.Search(ctx, Params{Text: "needle", Page: page, PageSize: 2})
		if err != nil {
			t.Fatalf("Search(page=%d): %v", page, err)
		}
		paged = append(paged, hitIDs(res.Hits)...)
		if len(res.Hits) < 2 {
			break
		}
	}

	if want := hitIDs(full.Hits); !slices.Equal(paged, want) {
		t.Errorf("paged order = %v, full order = %v", paged, want)
	}
}

func TestSortByTitleAndDates(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)

	alpha := testDoc("1", "alpha", "alpha body", nil)
	alpha.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	alpha.UpdatedAt = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	beta := testDoc("2", "Beta", "beta body", nil)
	beta.CreatedAt = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	beta.UpdatedAt = time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	zebra := testDoc("3", "Zebra", "zebra body", nil)
	zebra.CreatedAt = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	zebra.UpdatedAt = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	addAll(t, idx, zebra, alpha, beta)

	ctx := context.Background()

	byTitle, err := svc.Search(ctx, Params{Sort: "title"})
	if err != nil {
		t.Fatalf("Search(title): %v", err)
	}
	if got, want := hitTitles(byTitle.Hits), []string{"alpha", "Beta", "Zebra"}; !slices.Equal(got, want) {
		t.Errorf("title order = %v, want %v", got, want)
	}

	byCreated, err := svc.Search(ctx, Params{Sort: "created_at"})
	if err != nil {
		t.Fatalf("Search(created_at): %v", err)
	}
	if got, want := hitTitles(byCreated.Hits), []string{"Zebra", "Beta", "alpha"}; !slices.Equal(got, want) {
		t.Errorf("created_at order = %v, want %v", got, want)
	}

	byUpdated, err := svc.Search(ctx, Params{Sort: "updated_at"})
	if err != nil {
		t.Fatalf("Search(updated_at): %v", err)
	}
	if got, want := hitTitles(byUpdated.Hits), []string{"Zebra", "Beta", "alpha"}; !slices.Equal(got, want) {
		t.Errorf("updated_at order = %v, want %v", got, want)
	}
}

func TestMethodSelection(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)
	for i := 0; i < 5; i++ {
		addAll(t, idx, testDoc(fmt.Sprintf("%d", i), "Common Doc", "common language", nil))
	}
	dense := testDoc("d", "Dense Doc", strings.Repeat("common ", 50)+strings.Repeat(" padding", 200), nil)
	rare := testDoc("x", "Rare Doc", "zephyr zephyr zephyr"+strings.Repeat(" padding", 200), nil)
	addAll(t, idx, dense, rare)

	ctx := context.Background()
	bm25, err := svc.Search(ctx, Params{Text: "common zephyr", Method: "bm25", PageSize: 100})
	if err != nil {
		t.Fatalf("Search(bm25): %v", err)
	}
	tfidf, err := svc.Search(ctx, Params{Text: "common zephyr", Method: "tfidf", PageSize: 100})
	if err != nil {
		t.Fatalf("Search(tfidf): %v", err)
	}

	if got, want := hitIDs(tfidf.Hits)[0], mustUUID("x").String(); got != want {
		t.Errorf("tfidf top hit = %s, want the rare zephyr document", got)
	}
	if got, want := hitIDs(tfidf.Hits)[1], mustUUID("d").String(); got != want {
		t.Errorf("tfidf second hit = %s, want the dense document", got)
	}
	if top := hitIDs(bm25.Hits)[0]; top == mustUUID("x").String() || top == mustUUID("d").String() {
		t.Errorf("bm25 top hit = %s, want a common document", top)
	}
	if slices.Equal(hitIDs(bm25.Hits), hitIDs(tfidf.Hits)) {
		t.Errorf("bm25 and tfidf must produce different orders")
	}
}

func TestFiltersPassthrough(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)

	goDoc := testDoc("1", "Go Networking", "net package interfaces", []string{"go", "backend"})
	goDoc.Category = "backend"
	redisDoc := testDoc("2", "Redis Caching", "redis stores queries", []string{"cache", "backend"})
	redisDoc.Category = "backend"
	bleveDoc := testDoc("3", "Bleve Notes", "bleve search engine", []string{"go", "search"})
	bleveDoc.Category = "search"
	bleveDoc.UpdatedAt = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	addAll(t, idx, goDoc, redisDoc, bleveDoc)

	ctx := context.Background()

	res, err := svc.Search(ctx, Params{Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("Search(tags): %v", err)
	}
	if res.Total != 2 {
		t.Errorf("hits for tag go = %d, want 2", res.Total)
	}

	res, err = svc.Search(ctx, Params{Category: "search"})
	if err != nil {
		t.Fatalf("Search(category): %v", err)
	}
	if res.Total != 1 {
		t.Errorf("hits for category search = %d, want 1", res.Total)
	}
	if got, want := hitIDs(res.Hits), []string{mustUUID("3").String()}; !slices.Equal(got, want) {
		t.Errorf("category hits = %v, want %v", got, want)
	}

	res, err = svc.Search(ctx, Params{From: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Search(date range): %v", err)
	}
	if res.Total != 1 {
		t.Errorf("hits after 2026-02-01 = %d, want 1", res.Total)
	}
	if got, want := hitIDs(res.Hits), []string{mustUUID("3").String()}; !slices.Equal(got, want) {
		t.Errorf("date-filtered hits = %v, want %v", got, want)
	}
}

func TestEmptyTextBrowse(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)
	addAll(t, idx,
		testDoc("1", "Alpha", "body one", nil),
		testDoc("2", "Beta", "body two", nil),
		testDoc("3", "Gamma", "body three", nil),
	)

	ctx := context.Background()
	res, err := svc.Search(ctx, Params{Tags: []string{"none"}, PageSize: 100})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 0 {
		t.Errorf("total with unmatched tag = %d, want 0", res.Total)
	}

	res, err = svc.Search(ctx, Params{PageSize: 100})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 3 {
		t.Errorf("total without filters = %d, want 3", res.Total)
	}
}

func TestSnippetFallsBackToTitle(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)
	addAll(t, idx,
		testDoc("1", "Planner notes", "", nil),
		testDoc("2", "Other", "the planner article body text", nil),
	)

	res, err := svc.Search(context.Background(), Params{Text: "planner", PageSize: 100})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(res.Hits))
	}
	for _, h := range res.Hits {
		switch h.DocumentID {
		case mustUUID("1"):
			if h.Snippet != h.Title {
				t.Errorf("bodyless match snippet = %q, want fallback to title %q", h.Snippet, h.Title)
			}
		case mustUUID("2"):
			if h.Snippet == "" || !strings.Contains(h.Snippet, "<mark>") {
				t.Errorf("body match snippet = %q, want a highlighted fragment", h.Snippet)
			}
		}
	}
}

func TestSnippetHighlighted(t *testing.T) {
	idx := newTestIndex(t)
	svc := New(idx)
	addAll(t, idx, testDoc("1", "Postgres Tuning", "The planner picks an efficient plan for the query.", nil))

	res, err := svc.Search(context.Background(), Params{Text: "planner"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(res.Hits))
	}
	snippet := res.Hits[0].Snippet
	if snippet == "" {
		t.Fatal("expected a snippet, got empty")
	}
	if !strings.Contains(snippet, "<mark>") {
		t.Errorf("snippet has no highlight: %q", snippet)
	}
	if !strings.Contains(snippet, "planner") {
		t.Errorf("snippet misses the matched term: %q", snippet)
	}
}

func TestCacheKey(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := Params{
		Text:     "golang",
		Method:   "bm25",
		Tags:     []string{"redis", "go"},
		Category: "backend",
		From:     from,
		Sort:     "relevance",
		Page:     1,
		PageSize: 20,
	}

	reordered := base
	reordered.Tags = []string{"go", "redis"}
	if got, want := reordered.CacheKey(), base.CacheKey(); got != want {
		t.Errorf("tag reordering changed the key: %q vs %q", got, want)
	}

	variants := []struct {
		name string
		p    Params
	}{
		{"page", func() Params { p := base; p.Page = 2; return p }()},
		{"method", func() Params { p := base; p.Method = "tfidf"; return p }()},
		{"sort", func() Params { p := base; p.Sort = "title"; return p }()},
		{"text", func() Params { p := base; p.Text = "go"; return p }()},
		{"no dates", func() Params { p := base; p.From = time.Time{}; return p }()},
	}
	seen := make(map[string]string, len(variants))
	for _, v := range variants {
		key := v.p.CacheKey()
		if key == base.CacheKey() {
			t.Errorf("variant %q produced the base key %q", v.name, key)
		}
		if prev, dup := seen[key]; dup {
			t.Errorf("variants %q and %q share the key %q", prev, v.name, key)
		}
		seen[key] = v.name
	}

	for _, v := range variants {
		if got := v.p.CacheKey(); !strings.HasPrefix(got, "search:v1:") {
			t.Errorf("key %q lacks the search:v1: prefix", got)
		}
	}

	const want = "search:v1:7cfe853bfbfbb7a8ad0bf3dcab51e5ecc717c0fceb055ea288df312c4d084274"
	if got := base.CacheKey(); got != want {
		t.Errorf("stable key = %q, want %q", got, want)
	}
}
