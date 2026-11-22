package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

const (
	DefaultLimit = 10
	MaxLimit     = 200
	cursorPrefix = "c1:"
)

// Cursor marks where the next page starts: just after the row with LastValue and LastID
// in the Sort order it was made for.
type Cursor struct {
	Sort      string `json:"s"` // SortParams.String() of the page that made it
	LastValue string `json:"v"` // the last row's sort value, as text
	LastID    string `json:"i"` // the last row's id, the tie-breaker
}

type CursorParams struct {
	Limit  int
	Cursor string  // raw ?cursor
	After  *Cursor // set by Normalise; nil on the first page
}

// InvalidCursorError is one shared message: a cursor is server-generated, so a bad one is
// a client bug and there is nothing useful to tell the end user.
func InvalidCursorError() error {
	return handler.ErrorWithMessage(handler.CodeValidationFailed, "cursor is not a valid pagination cursor", nil)
}

// ParseLimit reads ?limit. Absent means the default page size; "0" or anything that is not
// a whole number in range is a 400, not a silent fallback to the default.
func ParseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > MaxLimit {
		return 0, limitOutOfRangeError()
	}
	return limit, nil
}

func limitOutOfRangeError() error {
	return handler.ErrorWithMessage(handler.CodeValidationFailed,
		fmt.Sprintf("limit must be a whole number between 1 and %d", MaxLimit), nil)
}

// Normalise applies the default limit, bounds it, and decodes Cursor. A cursor made for a
// different sort is rejected: its LastValue belongs to another column.
func (p *CursorParams) Normalise(sort SortParams) error {
	if p.Limit == 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit < 1 || p.Limit > MaxLimit {
		return limitOutOfRangeError()
	}

	p.After = nil
	if p.Cursor == "" {
		return nil
	}

	after, err := DecodeCursor(p.Cursor)
	if err != nil {
		return err
	}
	if after.Sort != sort.String() {
		return handler.ErrorWithMessage(handler.CodeValidationFailed,
			"cursor was made for a different sort; start again from the first page", nil)
	}
	p.After = after
	return nil
}

func EncodeCursor(cursor Cursor) string {
	body, _ := json.Marshal(cursor) // three strings: cannot fail
	return base64.RawURLEncoding.EncodeToString(append([]byte(cursorPrefix), body...))
}

func DecodeCursor(raw string) (*Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, InvalidCursorError()
	}

	body, ok := strings.CutPrefix(string(decoded), cursorPrefix)
	if !ok {
		return nil, InvalidCursorError()
	}

	var cursor Cursor
	if err := json.Unmarshal([]byte(body), &cursor); err != nil || cursor.LastID == "" {
		return nil, InvalidCursorError()
	}
	return &cursor, nil
}

// NewCursorResponse trims the lookahead row (the repository selects limit+1) and, when
// there was one, points NextCursor just after the last row kept.
func NewCursorResponse[T any](rows []T, limit int, cursorOf func(T) Cursor) *CursorResponse[T] {
	page := &CursorResponse[T]{Items: rows}

	if len(rows) > limit {
		page.Items = rows[:limit]
		next := EncodeCursor(cursorOf(page.Items[len(page.Items)-1]))
		page.NextCursor = &next
	}

	return page
}
