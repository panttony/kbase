package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/indexing"
	"github.com/panttony/kbase/internal/search"
)

func newTestServer(t *testing.T, withCache bool) (*httptest.Server, *fakeRepo, *fakeCache) {
	t.Helper()
	idx, err := indexing.New("")
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	t.Cleanup(func() { idx.Close() })

	repo := newFakeRepo()
	var c *fakeCache
	if withCache {
		c = newFakeCache("")
	}
	svc := search.New(idx)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := New(repo, idx, svc, c, time.Minute, log)

	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	return srv, repo, c
}

func do(t *testing.T, method, url, body string) (int, http.Header, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header, b
}

func mustDecodeError(t *testing.T, statusCode int, b []byte) errBody {
	t.Helper()
	var e errBody
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("error response is not valid JSON (%q): %v", b, err)
	}
	if e.Error.Code == "" || e.Error.Message == "" {
		t.Fatalf("unified error envelope missing fields: %s", b)
	}
	return e
}

func testContent(title string) string {
	return "---\ntitle: " + title + "\ntags: [go, backend]\ncategory: dev\n---\n#" + title + "\nBody about the subject.\n"
}

func createDoc(t *testing.T, srv *httptest.Server, content string) uuid.UUID {
	t.Helper()
	code, _, b := do(t, "POST", srv.URL+"/api/v1/documents", docRequestBody(content))
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", code, b)
	}
	var created documentResponse
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	return created.ID
}

func docRequestBody(content string) string {
	payload, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		panic(err)
	}
	return string(payload)
}

func TestCreateGetUpdateDeleteDocument(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	id := createDoc(t, srv, testContent("API Notes"))

	// Get.
	code, _, b := do(t, "GET", srv.URL+"/api/v1/documents/"+id.String(), "")
	if code != http.StatusOK {
		t.Fatalf("get status = %d, body %s", code, b)
	}
	var got documentResponse
	json.Unmarshal(b, &got)
	if got.Title != "API Notes" || len(got.Tags) != 2 || got.Version != 1 {
		t.Errorf("get = %+v", got)
	}

	// Update 
	code, _, b = do(t, "PUT", srv.URL+"/api/v1/documents/"+id.String(),
		docRequestBody(testContent("API Notes v2")))
	if code != http.StatusOK {
		t.Fatalf("update status = %d, body %s", code, b)
	}
	json.Unmarshal(b, &got)
	if got.Version != 2 || got.Title != "API Notes v2" {
		t.Errorf("updated = %+v", got)
	}

	// Versions list
	code, _, b = do(t, "GET", srv.URL+"/api/v1/documents/"+id.String()+"/versions", "")
	if code != http.StatusOK {
		t.Fatalf("versions status = %d, body %s", code, b)
	}
	var versions []versionResponse
	json.Unmarshal(b, &versions)
	if len(versions) != 2 || versions[0].Number != 1 || versions[1].Number != 2 {
		t.Errorf("versions = %+v", versions)
	}

	// Specific version
	code, _, b = do(t, "GET", srv.URL+"/api/v1/documents/"+id.String()+"/versions/1", "")
	if code != http.StatusOK {
		t.Fatalf("version 1 status = %d, body %s", code, b)
	}
	var v1 versionResponse
	json.Unmarshal(b, &v1)
	if v1.Number != 1 || v1.DocumentID != id {
		t.Errorf("version 1 = %+v", v1)
	}

	// Missing version.
	code, _, b = do(t, "GET", srv.URL+"/api/v1/documents/"+id.String()+"/versions/99", "")
	if code != http.StatusNotFound {
		t.Fatalf("missing version status = %d, want 404; body %s", code, b)
	}
	mustDecodeError(t, code, b)

	// Delete
	code, _, _ = do(t, "DELETE", srv.URL+"/api/v1/documents/"+id.String(), "")
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d", code)
	}

	// Get after delete
	code, _, b = do(t, "GET", srv.URL+"/api/v1/documents/"+id.String(), "")
	if code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404; body %s", code, b)
	}
	mustDecodeError(t, code, b)
}

func TestValidateContentWhenEmpty(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	code, _, b := do(t, "POST", srv.URL+"/api/v1/documents", `{"content": ""}`)
	if code != http.StatusBadRequest {
		t.Fatalf("empty content status = %d, want 400; body %s", code, b)
	}
	e := mustDecodeError(t, code, b)
	if e.Error.Code != "invalid_request" {
		t.Errorf("error code = %q, want invalid_request", e.Error.Code)
	}
}

func TestMalformedJSONIsBadRequest(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	code, _, b := do(t, "POST", srv.URL+"/api/v1/documents", `{"content": `)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", code, b)
	}
	mustDecodeError(t, code, b)
}

func TestInvalidUUIDIsBadRequest(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	code, _, b := do(t, "GET", srv.URL+"/api/v1/documents/not-a-uuid", "")
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", code, b)
	}
	mustDecodeError(t, code, b)
}

func TestNotFoundDocument(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	code, _, b := do(t, "GET", srv.URL+"/api/v1/documents/"+uuid.New().String(), "")
	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", code, b)
	}
	e := mustDecodeError(t, code, b)
	if e.Error.Code != "not_found" {
		t.Errorf("error code = %q, want not_found", e.Error.Code)
	}
}

func TestSearchHitsWithCache(t *testing.T) {
	srv, _, c := newTestServer(t, true)

	_ = createDoc(t, srv, "---\ntitle: Go Networking\ntags: [go]\ncategory: dev\n---\nInterfaces and goroutines.\n")
	_ = createDoc(t, srv, "---\ntitle: Redis Caching\ntags: [cache, go]\ncategory: dev\n---\nQuery cache with TTL.\n")
	_ = createDoc(t, srv, "---\ntitle: Bleve Indexing\ntags: [go, search]\ncategory: search\n---\nFull text over markdown.\n")

	code, _, b := do(t, "GET", srv.URL+"/api/v1/search?q=interfaces&method=bm25", "")
	if code != http.StatusOK {
		t.Fatalf("search status = %d, body %s", code, b)
	}
	var r1 searchResponse
	if err := json.Unmarshal(b, &r1); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if r1.Total == 0 || len(r1.Hits) == 0 {
		t.Fatalf("expected hits, got %+v", r1)
	}
	if r1.Hits[0].Title != "Go Networking" {
		t.Errorf("top hit = %s, want Go Networking", r1.Hits[0].Title)
	}

	if c == nil {
		t.Fatal("cache is nil in cached server")
	}
	c.mu.Lock()
	cached := len(c.items)
	c.mu.Unlock()
	if cached == 0 {
		t.Error("expected search results in the cache")
	}
}

func TestSearchInvalidParams(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	for name, q := range map[string]string{
		"page zero":    "/api/v1/search?page=0",
		"unknown sort": "/api/v1/search?sort=bogus",
		"bad date":     "/api/v1/search?from=not-a-date",
		"bad method":   "/api/v1/search?method=lm",
	} {
		t.Run(name, func(t *testing.T) {
			code, _, b := do(t, "GET", srv.URL+q, "")
			if code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body %s", code, b)
			}
			mustDecodeError(t, code, b)
		})
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv, _, _ := newTestServer(t, true)

	createDoc(t, srv, testContent("One"))
	createDoc(t, srv, testContent("Two"))

	code, _, b := do(t, "GET", srv.URL+"/api/v1/stats", "")
	if code != http.StatusOK {
		t.Fatalf("stats status = %d, body %s", code, b)
	}
	var s statsResponse
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if s.Documents != 2 {
		t.Errorf("documents = %d, want 2", s.Documents)
	}
	if s.TotalSize <= 0 {
		t.Errorf("total_size = %d, want > 0", s.TotalSize)
	}
}

func TestDocsAndSpecServed(t *testing.T) {
	srv, _, _ := newTestServer(t, false)

	code, hdr, b := do(t, "GET", srv.URL+"/docs/", "")
	if code != http.StatusOK {
		t.Fatalf("docs status = %d, body %s", code, b)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("docs content type = %q, want text/html", ct)
	}
	page := string(b)
	for _, want := range []string{"swagger-ui", specPath} {
		if !strings.Contains(page, want) {
			t.Errorf("docs page misses %q", want)
		}
	}

	code, hdr, b = do(t, "GET", srv.URL+specPath, "")
	if code != http.StatusOK {
		t.Fatalf("spec status = %d, body %s", code, b)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Errorf("spec content type = %q, want application/yaml", ct)
	}
	for _, want := range []string{"openapi: 3.1.0", "info:", "/documents", "/search"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("spec misses %q", want)
		}
	}
}

func TestCacheInvalidatedOnDelete(t *testing.T) {
	srv, _, c := newTestServer(t, true)

	id := createDoc(t, srv, testContent("Cached Doc"))
	do(t, "GET", srv.URL+"/api/v1/search?q=cached", "")
	c.mu.Lock()
	before := len(c.items)
	c.mu.Unlock()
	if before == 0 {
		t.Fatal("expected cache to be populated before delete")
	}

	do(t, "DELETE", srv.URL+"/api/v1/documents/"+id.String(), "")
	c.mu.Lock()
	after := len(c.items)
	c.mu.Unlock()
	if after != 0 {
		t.Errorf("cache not invalidated on delete: %d entries remain", after)
	}

	code, _, b := do(t, "GET", srv.URL+"/api/v1/search?q=cached", "")
	if code != http.StatusOK {
		t.Fatalf("search status = %d, body %s", code, b)
	}
	var r searchResponse
	json.Unmarshal(b, &r)
	if r.Total != 0 {
		t.Errorf("search after delete total = %d, want 0", r.Total)
	}
}
