package model

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
	cursorPrefix = "p1:"
)

// errBadCursor is one shared value: a cursor is server-generated, so a bad
// one is a client bug and there is nothing useful to tell the end user.
var errBadCursor = handler.ErrorWithMessage(handler.CodeValidationFailed,
	"cursor is not a valid pagination cursor", nil)

type CursorParams struct {
	Limit   int
	Cursor  string
	AfterID int64
}

func (p *CursorParams) Normalise() error {
	if p.Limit == 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit < 1 || p.Limit > MaxLimit {
		return handler.ErrorWithMessage(handler.CodeValidationFailed,
			fmt.Sprintf("limit must be between 1 and %d", MaxLimit), nil)
	}

	if p.Cursor != "" {
		afterID, err := DecodeCursor(p.Cursor)
		if err != nil {
			return err
		}
		p.AfterID = afterID
	}

	return nil
}

func EncodeCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(cursorPrefix + strconv.FormatInt(id, 10)))
}

func DecodeCursor(raw string) (int64, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, errBadCursor
	}

	body, ok := strings.CutPrefix(string(decoded), cursorPrefix)
	if !ok {
		return 0, errBadCursor
	}

	id, err := strconv.ParseInt(body, 10, 64)
	if err != nil || id < 1 {
		return 0, errBadCursor
	}
	return id, nil
}

func NewCursorReponse[T any](rows []T, limit int, total int64, idOf func(T) int64) *CursorReponse[T] {
	page := &CursorReponse[T]{Items: rows, Total: total}

	if len(rows) > limit {
		page.Items = rows[:limit]
		next := EncodeCursor(idOf(page.Items[len(page.Items)-1]))
		page.NextCursor = &next
	}

	return page
}
