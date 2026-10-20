package markdown

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func Normalize(p *Parsed) {
	p.Meta.Title = normalizeTitle(p.Meta.Title)
	p.Meta.Category = normalizeCategory(p.Meta.Category)
	p.Meta.Tags = normalizeTags(p.Meta.Tags)
	p.Meta.Authors = normalizeNames(p.Meta.Authors)
	p.Body = normalizeBody(p.Body)

	for i := range p.Headings {
		p.Headings[i].Text = normalizeTitle(p.Headings[i].Text)
	}
	for i := range p.Sections {
		p.Sections[i].Heading.Text = normalizeTitle(p.Sections[i].Heading.Text)
		p.Sections[i].Content = normalizeTitle(p.Sections[i].Content)
	}
	p.Plain = normalizeTitle(p.Plain)
	p.Title = resolveTitle(p)
}

func Process(raw []byte) (*Parsed, error) {
	p, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	Normalize(p)
	if err := Validate(p); err != nil {
		return nil, err
	}
	return p, nil
}

func resolveTitle(p *Parsed) string {
	switch {
	case p.Meta.Title != "":
		return p.Meta.Title
	case p.firstH1() != "":
		return p.firstH1()
	default:
		return "Untitled"
	}
}

func normalizeTitle(s string) string {
	return strings.Join(strings.Fields(nfkc(s)), " ")
}

func normalizeCategory(s string) string {
	return strings.ToLower(nfkc(strings.TrimSpace(s)))
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = nfkc(t)
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#"))
		t = strings.ToLower(t)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func normalizeNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.Join(strings.Fields(nfkc(n)), " ")
		if n == "" {
			continue
		}
		key := strings.ToLower(n)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	return out
}

func normalizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func nfkc(s string) string {
	if !utf8.ValidString(s) {
		return s
	}
	return norm.NFKC.String(s)
}
