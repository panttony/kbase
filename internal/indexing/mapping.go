package indexing

import (
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"

	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
)

const analyzerName = "kbase"

const (
	fieldTitle    = "title"
	fieldText     = "text"
	fieldTags     = "tags"
	fieldCategory = "category"
	fieldCreated  = "created_at"
	fieldUpdated  = "updated_at"
)

var scoredFields = []string{fieldTitle, fieldText}

type indexedDocument struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Text      string    `json:"text"`
	Tags      []string  `json:"tags"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newMapping() (*mapping.IndexMappingImpl, error) {
	m := bleve.NewIndexMapping()
	m.DefaultAnalyzer = analyzerName

	title := mapping.NewTextFieldMapping()
	title.Analyzer = analyzerName
	title.IncludeTermVectors = true
	title.IncludeInAll = false

	text := mapping.NewTextFieldMapping()
	text.Analyzer = analyzerName
	text.IncludeTermVectors = true
	text.IncludeInAll = false

	plain := func(f *mapping.FieldMapping) {
		f.IncludeInAll = false
	}
	tags := mapping.NewKeywordFieldMapping()
	category := mapping.NewKeywordFieldMapping()
	created := mapping.NewDateTimeFieldMapping()
	updated := mapping.NewDateTimeFieldMapping()
	plain(tags)
	plain(category)
	plain(created)
	plain(updated)

	doc := mapping.NewDocumentMapping()
	doc.AddFieldMappingsAt(fieldTitle, title)
	doc.AddFieldMappingsAt(fieldText, text)
	doc.AddFieldMappingsAt(fieldTags, tags)
	doc.AddFieldMappingsAt(fieldCategory, category)
	doc.AddFieldMappingsAt(fieldCreated, created)
	doc.AddFieldMappingsAt(fieldUpdated, updated)
	doc.Enabled = true
	doc.Dynamic = false
	m.AddDocumentMapping("_default", doc)

	if err := m.AddCustomAnalyzer(analyzerName, map[string]interface{}{
		"type":          "custom",
		"tokenizer":     "unicode",
		"token_filters": []string{"to_lower"},
	}); err != nil {
		return nil, err
	}
	return m, nil
}

func tokenize(s string) []string {
	stream := unicode.NewUnicodeTokenizer().Tokenize([]byte(s))
	stream = lowercase.NewLowerCaseFilter().Filter(stream)
	terms := make([]string, 0, len(stream))
	for _, tok := range stream {
		if len(tok.Term) > 0 {
			terms = append(terms, string(tok.Term))
		}
	}
	return terms
}
