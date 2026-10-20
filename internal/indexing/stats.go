package indexing

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
)

func (x *BleveIndexer) DocFreq(term string) (uint64, error) {
	text := bleve.NewTermQuery(term)
	text.SetField(fieldText)
	title := bleve.NewTermQuery(term)
	title.SetField(fieldTitle)

	combined := bleve.NewDisjunctionQuery(text, title)
	req := bleve.NewSearchRequestOptions(combined, 0, 0, false)
	sr, err := x.idx.Search(req)
	if err != nil {
		return 0, fmt.Errorf("doc freq %q: %w", term, err)
	}
	return sr.Total, nil
}
