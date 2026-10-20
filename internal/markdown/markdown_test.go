package markdown

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const sampleDoc = `---
title: Go Networking Notes
tags: ["#Go", "networking", "GO ", " cgo "]
category: "Backend"
authors:
  - " Anton Panteleymonov "
  - anton panteleymonov
created_at: 2026-01-10T10:00:00Z
updated_at: 2026-02-01T12:30:00Z
---

# Net Package

The net package provides portable interfaces for network I/O.

## Dial

Dial establishes connections to remote hosts.

## Listen

Listen creates listeners for incoming connections.

- TCP
- UDP
`

func TestParseStructure(t *testing.T) {
	p, err := Process([]byte(sampleDoc))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if got, want := len(p.Headings), 3; got != want {
		t.Errorf("headings = %d, want %d", got, want)
	}
	if got, want := p.Headings[0].Level, 1; got != want {
		t.Errorf("first heading level = %d, want %d", got, want)
	}
	if got, want := p.Headings[0].Text, "Net Package"; got != want {
		t.Errorf("first heading text = %q, want %q", got, want)
	}
	if got, want := len(p.Sections), 3; got != want {
		t.Errorf("sections = %d, want %d", got, want)
	}
	if !strings.Contains(p.Sections[2].Content, "TCP") || !strings.Contains(p.Sections[2].Content, "UDP") {
		t.Errorf("section content missing list items: %q", p.Sections[2].Content)
	}
	if !strings.Contains(p.Plain, "portable interfaces") {
		t.Errorf("plain text missing paragraph: %q", p.Plain)
	}
}

func TestParseMetaNormalization(t *testing.T) {
	p, err := Process([]byte(sampleDoc))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got, want := p.Meta.Tags, []string{"go", "networking", "cgo"}; !equalStrings(got, want) {
		t.Errorf("tags = %v, want %v", got, want)
	}
	if got, want := p.Meta.Category, "backend"; got != want {
		t.Errorf("category = %q, want %q", got, want)
	}
	if got, want := p.Meta.Authors, []string{"Anton Panteleymonov"}; !equalStrings(got, want) {
		t.Errorf("authors = %v, want %v", got, want)
	}
	if got, want := p.Meta.CreatedAt.Format(time.RFC3339), "2026-01-10T10:00:00Z"; got != want {
		t.Errorf("created_at = %q, want %q", got, want)
	}
}

func TestTitleResolution(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"front matter wins", "---\ntitle: From Front\n---\n# From H1\nbody", "From Front"},
		{"h1 fallback", "# From H1\nbody", "From H1"},
		{"untitled", "just a body", "Untitled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Process([]byte(tt.raw))
			if err != nil {
				t.Fatalf("Process: %v", err)
			}
			if p.Title != tt.want {
				t.Errorf("title = %q, want %q", p.Title, tt.want)
			}
		})
	}
}

func TestNormalizeBody(t *testing.T) {
	raw := []byte("# Head\r\n\r\nLine\r\n")
	p, err := Process(raw)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if strings.Contains(p.Body, "\r") {
		t.Errorf("body still contains CR: %q", p.Body)
	}
	if !strings.HasSuffix(p.Body, "\n") {
		t.Errorf("body does not end with newline: %q", p.Body)
	}
}

func TestNormalizeUnicode(t *testing.T) {
	raw := []byte("---\ntitle: \"ﬁle notes\"\n---\n# Title\nbody")
	p, err := Process(raw)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if p.Meta.Title != "file notes" {
		t.Errorf("title = %q, want %q", p.Meta.Title, "file notes")
	}
}

func TestValidationErrors(t *testing.T) {
	empty := []byte("---\ntitle: No Body\n---\n")
	_, err := Process(empty)
	var fe *FieldError
	if !errors.As(err, &fe) || fe.Field != "body" {
		t.Errorf("expected body field error, got %v", err)
	}

	badDates := []byte("---\ncreated_at: 2026-02-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\n---\ntext")
	_, err = Process(badDates)
	if !errors.As(err, &fe) || fe.Field != "updated_at" {
		t.Errorf("expected updated_at field error, got %v", err)
	}

	var manyTags strings.Builder
	manyTags.WriteString("---\ntags:\n")
	for i := 0; i <= MaxTags; i++ {
		manyTags.WriteString("  - tag")
		manyTags.WriteString(strings.Repeat("x", i))
		manyTags.WriteByte('\n')
	}
	manyTags.WriteString("---\ntext")
	if _, err = Process([]byte(manyTags.String())); err == nil {
		t.Error("expected error for too many tags")
	}
}

func TestValidationAggregation(t *testing.T) {
	raw := []byte("---\ncreated_at: 2026-02-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\n---\n")
	_, err := Process(raw)
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "body") || !strings.Contains(msg, "updated_at") {
		t.Errorf("aggregated error missing fields: %v", err)
	}
}

func TestFrontMatterErrors(t *testing.T) {
	unknownField := []byte("---\ntitlex: typo\n---\nbody")
	if _, err := Process(unknownField); !errors.Is(err, ErrBadFrontMatter) {
		t.Errorf("expected ErrBadFrontMatter for unknown field, got %v", err)
	}

	notFrontMatter := []byte("# Thematic break\n\n---\n\nnot front matter")
	p, err := Process(notFrontMatter)
	if err != nil {
		t.Fatalf("thematic break must not fail: %v", err)
	}
	if p.Meta.Title != "" {
		t.Errorf("title should be empty without front matter, got %q", p.Meta.Title)
	}
	if p.Title != "Thematic break" {
		t.Errorf("title = %q, want %q", p.Title, "Thematic break")
	}
}

func TestTooLarge(t *testing.T) {
	raw := make([]byte, MaxDocumentSize+1)
	if _, err := Process(raw); !errors.Is(err, ErrTooLarge) {
		t.Errorf("expected ErrTooLarge, got %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
