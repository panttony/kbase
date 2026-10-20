package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/panttony/kbase/internal/search"
)

var (
	errPage     = errors.New("page must be a positive integer")
	errPageSize = errors.New("page_size must be a positive integer")
	errDate     = errors.New("from/to must be RFC3339 timestamps")
)

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	p, err := parseSearchParams(r)
	if err != nil {
		writeError(w, invalidRequest(err.Error()))
		return
	}

	if h.cache != nil {
		key := p.CacheKey()
		if cached, _ := h.cache.Get(r.Context(), key); cached != "" {
			var out searchResponse
			if json.Unmarshal([]byte(cached), &out) == nil {
				writeJSON(w, http.StatusOK, out)
				return
			}
		}
	}

	out, err := searchAndMap(r.Context(), h.svc, p)
	if err != nil {
		writeError(w, mapSearchError(err))
		return
	}

	if h.cache != nil {
		if b, err := json.Marshal(out); err == nil {
			_ = h.cache.Set(r.Context(), p.CacheKey(), string(b), h.cacheTTL)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	s, err := h.repo.Stats(r.Context())
	if err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	writeJSON(w, http.StatusOK, toStatsResponse(s))
}

func searchAndMap(ctx context.Context, svc searcher, p search.Params) (searchResponse, error) {
	out, err := svc.Search(ctx, p)
	if err != nil {
		return searchResponse{}, err
	}
	return toSearchResponse(out), nil
}

func mapSearchError(err error) error {
	var pe *search.ParamError
	if errors.As(err, &pe) {
		return &appError{status: http.StatusBadRequest, code: codeInvalidRequest, message: pe.Error()}
	}
	return err
}

func parseSearchParams(r *http.Request) (search.Params, error) {
	q := r.URL.Query()

	params := search.Params{Text: q.Get("q")}
	if m := q.Get("method"); m != "" {
		params.Method = m
	}
	if c := q.Get("category"); c != "" {
		params.Category = c
	}
	params.Tags = append([]string(nil), q["tags"]...)
	if s := q.Get("sort"); s != "" {
		params.Sort = s
	}

	from, err := parseTime(q.Get("from"))
	if err != nil {
		return params, fmt.Errorf("%w: from", errDate)
	}
	to, err := parseTime(q.Get("to"))
	if err != nil {
		return params, fmt.Errorf("%w: to", errDate)
	}
	params.From = from
	params.To = to

	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return params, errPage
		}
		params.Page = n
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return params, errPageSize
		}
		params.PageSize = n
	}

	return params, nil
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}
