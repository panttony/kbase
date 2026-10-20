package markdown

import (
	"errors"
	"fmt"
	"strings"

	"github.com/panttony/kbase/internal/model"
)

type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func Validate(p *Parsed) error {
	var errs []error
	if strings.TrimSpace(p.Body) == "" {
		errs = append(errs, &FieldError{Field: "body", Message: "document body is empty"})
	}
	if tooLong(p.Title, MaxTitleLength) {
		errs = append(errs, &FieldError{Field: "title", Message: fmt.Sprintf("title exceeds %d characters", MaxTitleLength)})
	}
	if tooLong(p.Meta.Category, MaxCategoryLength) {
		errs = append(errs, &FieldError{Field: "category", Message: fmt.Sprintf("category exceeds %d characters", MaxCategoryLength)})
	}
	if len(p.Meta.Tags) > MaxTags {
		errs = append(errs, &FieldError{Field: "tags", Message: fmt.Sprintf("too many tags: %d, maximum is %d", len(p.Meta.Tags), MaxTags)})
	}
	if len(p.Meta.Authors) > MaxAuthors {
		errs = append(errs, &FieldError{Field: "authors", Message: fmt.Sprintf("too many authors: %d, maximum is %d", len(p.Meta.Authors), MaxAuthors)})
	}
	if err := validateDates(p.Meta); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func validateDates(meta model.Meta) error {
	if meta.CreatedAt != nil && meta.UpdatedAt != nil {
		if meta.UpdatedAt.Before(*meta.CreatedAt) {
			return &FieldError{Field: "updated_at", Message: "updated_at is before created_at"}
		}
	}
	return nil
}

func tooLong(s string, max int) bool {
	return len([]rune(s)) > max
}
