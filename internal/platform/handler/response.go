package handler

// SuccessResponse is the body of every successful response; T is the shape of Data. Its
// counterpart for failures is ErrorResponse. Metadata is set only by paged lists and
// omitted otherwise.
type SuccessResponse[T any] struct {
	SessionID string          `json:"session_id"`
	Count     *int64          `json:"count"`
	Data      T               `json:"data"`
	Metadata  *CursorMetadata `json:"metadata,omitempty"`
}

// CursorMetadata is a paged list's Metadata. NextCursor is null on the last page. Total is
// the number of rows matching the filters, sent on the first page only (no cursor): one
// COUNT per list view rather than one per page. It lives here rather than in
// platform/model, which imports this package.
type CursorMetadata struct {
	NextCursor *string `json:"next_cursor"`
	Total      *int64  `json:"total,omitempty"`
}

// responseWithDefaultCount is implemented by every SuccessResponse[T]. A type switch
// cannot name SuccessResponse without its T, so ReturnWithJSON finds it through this
// interface instead.
type responseWithDefaultCount interface {
	withDefaultCount() any
}

// withDefaultCount sets Count to 1 when a single item is returned without one. It has a
// value receiver: handlers pass SuccessResponse values, not pointers.
func (r SuccessResponse[T]) withDefaultCount() any {
	if r.Count == nil {
		one := int64(1)
		r.Count = &one
	}
	return r
}
