package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func Parse(raw []byte) (*Parsed, error) {
	if len(raw) > MaxDocumentSize {
		return nil, ErrTooLarge
	}
	body, front, err := splitFrontMatter(raw)
	if err != nil {
		return nil, err
	}
	meta, err := parseMeta(front)
	if err != nil {
		return nil, err
	}

	gmd := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
	reader := text.NewReader(body)
	doc := gmd.Parser().Parse(reader)

	p := &Parsed{Meta: meta, Body: string(body)}
	walk(gmd, doc, body, p)
	return p, nil
}

func walk(gmd goldmark.Markdown, doc ast.Node, source []byte, p *Parsed) {
	var plain strings.Builder
	var section *Section
	flush := func(text string) {
		if strings.TrimSpace(text) != "" {
			plain.WriteString(text)
			plain.WriteByte('\n')
		}
	}
	appendText := func(text string) {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return
		}
		if section == nil {
			section = &Section{}
		}
		if section.Content != "" {
			section.Content += " "
		}
		section.Content += trimmed
	}

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		if h, ok := child.(*ast.Heading); ok {
			if section != nil {
				p.Sections = append(p.Sections, *section)
			}
			heading := Heading{Level: h.Level, Text: nodeText(child, source)}
			p.Headings = append(p.Headings, heading)
			section = &Section{Heading: heading}
			flush(heading.Text)
			continue
		}
		text := strings.TrimSpace(nodeText(child, source))
		if text == "" {
			continue
		}
		flush(text)
		appendText(text)
	}
	if section != nil {
		p.Sections = append(p.Sections, *section)
	}
	p.Plain = strings.TrimSpace(plain.String())
}

func nodeText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(source))
		case *ast.String:
			b.WriteString(string(t.Value))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func (p *Parsed) firstH1() string {
	for _, h := range p.Headings {
		if h.Level == 1 {
			return h.Text
		}
	}
	return ""
}
