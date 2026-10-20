package api

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/model"
	"github.com/panttony/kbase/internal/storage"
)

type fakeRepo struct {
	mu       sync.Mutex
	docs     map[uuid.UUID]model.Document
	versions map[uuid.UUID][]model.Version
	nextVer  map[uuid.UUID]int
	deleted  map[uuid.UUID]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		docs:     map[uuid.UUID]model.Document{},
		versions: map[uuid.UUID][]model.Version{},
		nextVer:  map[uuid.UUID]int{},
		deleted:  map[uuid.UUID]bool{},
	}
}

func (f *fakeRepo) Create(ctx context.Context, doc model.Document) (model.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	doc.Version = 1
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now().UTC()
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = doc.CreatedAt
	}
	f.docs[doc.ID] = doc
	f.versions[doc.ID] = append(f.versions[doc.ID], model.Version{
		ID: uuid.New(), DocumentID: doc.ID, Number: 1, Body: doc.Body,
		Meta:      model.Meta{Title: doc.Title, Tags: doc.Tags, Category: doc.Category, Authors: doc.Authors},
		CreatedAt: doc.CreatedAt,
	})
	f.nextVer[doc.ID] = 2
	return f.docs[doc.ID], nil
}

func (f *fakeRepo) Get(ctx context.Context, id uuid.UUID) (model.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleted[id] {
		return model.Document{}, storage.ErrNotFound{Kind: "document", ID: id}
	}
	doc, ok := f.docs[id]
	if !ok {
		return model.Document{}, storage.ErrNotFound{Kind: "document", ID: id}
	}
	return doc, nil
}

func (f *fakeRepo) Update(ctx context.Context, doc model.Document) (model.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.docs[doc.ID]
	if !ok || f.deleted[doc.ID] {
		return model.Document{}, storage.ErrNotFound{Kind: "document", ID: doc.ID}
	}
	ver := cur.Version + 1
	doc.ID = cur.ID
	doc.CreatedAt = cur.CreatedAt
	doc.UpdatedAt = time.Now().UTC()
	doc.Version = ver
	f.docs[doc.ID] = doc
	f.versions[doc.ID] = append(f.versions[doc.ID], model.Version{
		ID: uuid.New(), DocumentID: doc.ID, Number: ver, Body: doc.Body,
		Meta:      model.Meta{Title: doc.Title, Tags: doc.Tags, Category: doc.Category, Authors: doc.Authors},
		CreatedAt: doc.UpdatedAt,
	})
	return f.docs[doc.ID], nil
}

func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.docs[id]; !ok || f.deleted[id] {
		return storage.ErrNotFound{Kind: "document", ID: id}
	}
	f.deleted[id] = true
	return nil
}

func (f *fakeRepo) ListVersions(ctx context.Context, id uuid.UUID) ([]model.Version, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.docs[id]; !ok || f.deleted[id] {
		return nil, storage.ErrNotFound{Kind: "document", ID: id}
	}
	return append([]model.Version(nil), f.versions[id]...), nil
}

func (f *fakeRepo) GetVersion(ctx context.Context, documentID uuid.UUID, number int) (model.Version, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.docs[documentID]; !ok || f.deleted[documentID] {
		return model.Version{}, storage.ErrNotFound{Kind: "document", ID: documentID}
	}
	for _, v := range f.versions[documentID] {
		if v.Number == number {
			return v, nil
		}
	}
	return model.Version{}, storage.ErrNotFound{Kind: "version", ID: documentID.String() + "/" + itoa(number)}
}

func (f *fakeRepo) Stats(ctx context.Context) (model.Stats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := model.Stats{}
	for id, d := range f.docs {
		if f.deleted[id] {
			continue
		}
		s.Documents++
		s.TotalSize += int64(len(d.Body))
	}
	return s, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

type memEntry struct {
	value string
	ttl   time.Time
}

type fakeCache struct {
	mu     sync.Mutex
	prefix string
	items  map[string]memEntry
}

func newFakeCache(prefix string) *fakeCache {
	return &fakeCache{prefix: prefix, items: map[string]memEntry{}}
}

func (c *fakeCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok || time.Now().After(e.ttl) {
		return "", nil
	}
	return e.value, nil
}

func (c *fakeCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = memEntry{value: value, ttl: time.Now().Add(ttl)}
	return nil
}

func (c *fakeCache) InvalidateAll(ctx context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(prefix) == 0 || hasPrefix(k, prefix) {
			delete(c.items, k)
		}
	}
	return nil
}

func hasPrefix(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }
