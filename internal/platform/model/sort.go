package model

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// SortParams is one sort field and its direction, from ?sort=field or ?sort=-field.
type SortParams struct {
	Field string
	Desc  bool
}

// ParseSort reads raw against the fields a list allows; an empty raw sorts by
// defaultField ascending. Field is only ever compared, never written into SQL: the
// repository maps it to a column.
func ParseSort(raw string, allowedFields []string, defaultField string) (SortParams, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SortParams{Field: defaultField}, nil
	}

	field, desc := strings.CutPrefix(raw, "-")
	if !slices.Contains(allowedFields, field) {
		return SortParams{}, handler.ErrorWithMessage(handler.CodeValidationFailed,
			fmt.Sprintf("sort must be one of %s, with a leading - for descending", strings.Join(allowedFields, ", ")), nil)
	}
	return SortParams{Field: field, Desc: desc}, nil
}

// String is the sort as a client writes it: "name" or "-name".
func (p SortParams) String() string {
	if p.Desc {
		return "-" + p.Field
	}
	return p.Field
}

// KeysetOperator compares (sort column, id) with a cursor's position: rows after it in
// this sort's direction.
func (p SortParams) KeysetOperator() string {
	if p.Desc {
		return "<"
	}
	return ">"
}

// Direction is the SQL keyword for the sort's direction.
func (p SortParams) Direction() string {
	if p.Desc {
		return "DESC"
	}
	return "ASC"
}
