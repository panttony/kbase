package search

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/panttony/kbase/internal/indexing"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

type ParamError struct{ msg string }

func (e *ParamError) Error() string { return e.msg }

func paramError(format string, args ...any) error {
	return &ParamError{msg: fmt.Sprintf(format, args...)}
}

func (p *Params) normalize() {
	if p.Page == 0 {
		p.Page = defaultPage
	}
	if p.PageSize == 0 {
		p.PageSize = defaultPageSize
	}
	p.Method = strings.ToLower(strings.TrimSpace(p.Method))
	if p.Method == "" {
		p.Method = "bm25"
	}
	p.Sort = strings.ToLower(strings.TrimSpace(p.Sort))
	if p.Sort == "" {
		p.Sort = "relevance"
	}
}

func (p Params) validate() error {
	if p.Page < 1 {
		return errors.New("page must be >= 1")
	}
	if p.PageSize < 1 || p.PageSize > maxPageSize {
		return fmt.Errorf("page_size must be between 1 and %d", maxPageSize)
	}
	if _, err := indexing.NewScoringMethod(p.Method); err != nil {
		return err
	}
	switch p.Sort {
	case "relevance", "title", "created_at", "updated_at":
	default:
		return fmt.Errorf("unknown sort %q", p.Sort)
	}
	if !p.From.IsZero() && !p.To.IsZero() && p.From.After(p.To) {
		return errors.New("from must not be after to")
	}
	return nil
}

func (p Params) CacheKey() string {
	q := p
	q.normalize()

	tags := append([]string(nil), q.Tags...)
	sort.Strings(tags)

	var dates []string
	if !q.From.IsZero() {
		dates = append(dates, q.From.UTC().Format(time.RFC3339Nano))
	}
	if !q.To.IsZero() {
		dates = append(dates, q.To.UTC().Format(time.RFC3339Nano))
	}

	canonical := fmt.Sprintf("v1|%s|%s|%s|%s|%s|%s|%d|%d",
		q.Method, q.Text, strings.Join(tags, ","), q.Category,
		strings.Join(dates, ","), q.Sort, q.Page, q.PageSize)
	sum := sha256.Sum256([]byte(canonical))
	return "search:v1:" + hex.EncodeToString(sum[:])
}
