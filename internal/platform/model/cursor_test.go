package model

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

func errorCode(err error) handler.ErrorCode {
	var apiErr *handler.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return 0
}

// A cursor's LastValue belongs to the column it was sorted by. Reused under another sort,
// "Rack Attack" would be compared against created_at — a cast error, or a page that
// silently starts in the wrong place.
func TestCursorParamsNormalise(t *testing.T) {
	byName := SortParams{Field: "name"}
	byNameDesc := SortParams{Field: "name", Desc: true}
	cursorByName := EncodeCursor(Cursor{Sort: byName.String(), LastValue: "Rack Attack", LastID: "id-1"})

	tests := []struct {
		name      string
		params    CursorParams
		sort      SortParams
		wantCode  handler.ErrorCode
		wantLimit int
		wantAfter bool
	}{
		{"first page gets the default limit", CursorParams{}, byName, 0, DefaultLimit, false},
		{"cursor for the same sort", CursorParams{Cursor: cursorByName}, byName, 0, DefaultLimit, true},
		{"cursor for another direction", CursorParams{Cursor: cursorByName}, byNameDesc, handler.CodeValidationFailed, 0, false},
		{"not base64", CursorParams{Cursor: "%%%"}, byName, handler.CodeValidationFailed, 0, false},
		{"wrong prefix", CursorParams{Cursor: base64.RawURLEncoding.EncodeToString([]byte(`p1:{"s":"name"}`))}, byName, handler.CodeValidationFailed, 0, false},
		{"no id", CursorParams{Cursor: EncodeCursor(Cursor{Sort: "name", LastValue: "x"})}, byName, handler.CodeValidationFailed, 0, false},
		{"limit over max", CursorParams{Limit: MaxLimit + 1}, byName, handler.CodeValidationFailed, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.params
			err := params.Normalise(tt.sort)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %d, want %d (err: %v)", got, tt.wantCode, err)
			}
			if tt.wantCode != 0 {
				return
			}
			if params.Limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", params.Limit, tt.wantLimit)
			}
			if (params.After != nil) != tt.wantAfter {
				t.Errorf("after = %+v, want set = %v", params.After, tt.wantAfter)
			}
		})
	}
}

// The repository selects limit+1. A page that forgot to trim would serve 11 rows and a
// cursor pointing past one the client never saw.
func TestNewCursorResponse(t *testing.T) {
	cursorOf := func(id string) Cursor { return Cursor{Sort: "name", LastValue: id, LastID: id} }

	t.Run("lookahead row present", func(t *testing.T) {
		page := NewCursorResponse([]string{"a", "b", "c"}, 2, cursorOf)
		if len(page.Items) != 2 || page.NextCursor == nil {
			t.Fatalf("items = %v, next = %v; want 2 items and a next cursor", page.Items, page.NextCursor)
		}
		next, err := DecodeCursor(*page.NextCursor)
		if err != nil || next.LastID != "b" {
			t.Errorf("next cursor = %+v (err %v), want after b", next, err)
		}
	})

	t.Run("last page", func(t *testing.T) {
		page := NewCursorResponse([]string{"a", "b"}, 2, cursorOf)
		if len(page.Items) != 2 || page.NextCursor != nil {
			t.Errorf("items = %v, next = %v; want 2 items and no next cursor", page.Items, page.NextCursor)
		}
	})
}

// "limit=0" or "limit=abc" must be a 400. Falling back to the default would hide a client
// bug, and a value past MaxLimit would let one request scan an unbounded page.
func TestParseLimit(t *testing.T) {
	tests := []struct {
		raw      string
		want     int
		wantCode handler.ErrorCode
	}{
		{"", 0, 0}, // Normalise applies DefaultLimit
		{"5", 5, 0},
		{" 25 ", 25, 0},
		{"200", 200, 0},
		{"0", 0, handler.CodeValidationFailed},
		{"-1", 0, handler.CodeValidationFailed},
		{"201", 0, handler.CodeValidationFailed},
		{"abc", 0, handler.CodeValidationFailed},
		{"2.5", 0, handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseLimit(tt.raw)
			if code := errorCode(err); code != tt.wantCode {
				t.Fatalf("error code = %d, want %d (err: %v)", code, tt.wantCode, err)
			}
			if got != tt.want {
				t.Errorf("ParseLimit(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
