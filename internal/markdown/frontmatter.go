package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/panttony/kbase/internal/model"
	"gopkg.in/yaml.v3"
)

var bom = []byte{0xEF, 0xBB, 0xBF}

var (
	ErrTooLarge       = errors.New("document exceeds maximum size")
	ErrBadFrontMatter = errors.New("invalid front matter")
)

func splitFrontMatter(raw []byte) (body, front []byte, err error) {
	raw = bytes.TrimPrefix(raw, bom)
	lines := bytes.SplitAfter(raw, []byte("\n"))
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(string(ln)) == "" {
			continue
		}
		if strings.TrimSpace(string(ln)) == "---" {
			start = i
		}
		break
	}
	if start == -1 {
		return raw, nil, nil
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == "---" {
			front = bytes.Join(lines[start+1:i], nil)
			body = bytes.Join(lines[i+1:], nil)
			return body, front, nil
		}
	}
	return raw, nil, nil
}

func parseMeta(front []byte) (model.Meta, error) {
	var meta model.Meta
	if len(bytes.TrimSpace(front)) == 0 {
		return meta, nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(front))
	dec.KnownFields(true)
	if err := dec.Decode(&meta); err != nil {
		return meta, fmt.Errorf("%w: %v", ErrBadFrontMatter, err)
	}
	return meta, nil
}
