package markdown

import (
	"github.com/panttony/kbase/internal/model"
)

const (
	MaxDocumentSize   = 1 << 20 
	MaxTags           = 32
	MaxAuthors        = 16
	MaxTitleLength    = 256
	MaxCategoryLength = 64
	MaxSectionText    = 1 << 16 
)

type Heading struct {
	Level int
	Text  string
}

type Section struct {
	Heading Heading
	Content string
}

type Parsed struct {
	Meta     model.Meta
	Title    string    
	Body     string   
	Plain    string    
	Headings []Heading 
	Sections []Section 
}
