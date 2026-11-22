package handler

// SuccessResponse is the body of every successful response; T is the shape of Data. Its
// counterpart for failures is ErrorResponse.
type SuccessResponse[T any] struct {
	Data     T         `json:"data"`
	Metadata *Metadata `json:"metadata,omitempty"` // lists only; nil for a single item
}

// Metadata describes a list response. Build it with NewMetadata or
// NewPageMetadata rather than field by field, so Count always matches Data and HasMore
// always matches NextCursor.
//
// It lives here rather than in platform/model, which imports this package.
type Metadata struct {
	Count int64 `json:"count"` // rows in this response: len(Data)
	// Rows matching the filters, across all pages. Sent on the first page only (no cursor):
	// one COUNT per list view rather than one per page.
	Total *int64 `json:"total,omitempty"`
	// Whether another page follows: NextCursor != nil. Always false on a list that is not
	// paged.
	HasMore bool `json:"has_more"`
	// Pass back as ?cursor= for the next page. Absent when HasMore is false.
	NextCursor *string `json:"next_cursor,omitempty"`
}

// NewMetadata is the metadata of a list returned whole, in one response.
func NewMetadata(count int) *Metadata {
	return &Metadata{Count: int64(count)}
}

// NewPageMetadata is the metadata of one page of a paged list. total is nil after the
// first page; nextCursor is nil on the last.
func NewPageMetadata(count int, total *int64, nextCursor *string) *Metadata {
	return &Metadata{
		Count:      int64(count),
		Total:      total,
		HasMore:    nextCursor != nil,
		NextCursor: nextCursor,
	}
}
