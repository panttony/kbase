package model

import (
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID        uuid.UUID
	Title     string
	Category  string
	Tags      []string
	Authors   []string
	Source    string
	Body      string
	PlainText string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int
}

type Meta struct {
	Title     string     `yaml:"title"`
	Tags      []string   `yaml:"tags"`
	Category  string     `yaml:"category"`
	Authors   []string   `yaml:"authors"`
	CreatedAt *time.Time `yaml:"created_at"`
	UpdatedAt *time.Time `yaml:"updated_at"`
}

type Version struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	Number     int
	Body       string
	Meta       Meta
	CreatedAt  time.Time
}

type Stats struct {
	Documents int   `json:"documents"`
	TotalSize int64 `json:"total_size"` 
}

type Hit struct {
	DocumentID uuid.UUID
	Title      string
	Score      float64
	Snippet    string
	UpdatedAt  time.Time
}
