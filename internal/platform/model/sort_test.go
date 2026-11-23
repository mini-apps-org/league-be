package model

import (
	"errors"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// The allowed list is the only thing between ?sort and an ORDER BY. A field that slips
// through would reach the repository's column switch, which falls back to the default
// column — the client would get a silently wrong order instead of a 400.
func TestParseSort(t *testing.T) {
	allowed := []string{"name", "created_at"}

	tests := []struct {
		raw      string
		want     SortParams
		wantCode handler.ErrorCode
	}{
		{"", SortParams{Field: "name"}, 0},
		{"   ", SortParams{Field: "name"}, 0},
		{"created_at", SortParams{Field: "created_at"}, 0},
		{"-created_at", SortParams{Field: "created_at", Desc: true}, 0},
		{" -name ", SortParams{Field: "name", Desc: true}, 0},
		{"--name", SortParams{}, handler.CodeValidationFailed},
		{"+name", SortParams{}, handler.CodeValidationFailed},
		{"Name", SortParams{}, handler.CodeValidationFailed},
		{"name,created_at", SortParams{}, handler.CodeValidationFailed},
		{"name; DROP TABLE teams", SortParams{}, handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseSort(tt.raw, allowed, "name")

			var apiErr *handler.APIError
			var gotCode handler.ErrorCode
			if errors.As(err, &apiErr) {
				gotCode = apiErr.Code
			}
			if gotCode != tt.wantCode {
				t.Fatalf("error code = %d, want %d (err: %v)", gotCode, tt.wantCode, err)
			}
			if got != tt.want {
				t.Errorf("ParseSort(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSortParamsDirection(t *testing.T) {
	if got := (SortParams{}).Direction(); got != "ASC" {
		t.Errorf("ascending = %q, want ASC", got)
	}
	if got := (SortParams{Desc: true}).Direction(); got != "DESC" {
		t.Errorf("descending = %q, want DESC", got)
	}
}
