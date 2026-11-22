package model

type CursorResponse[T any] struct {
	Items      []T
	NextCursor *string
	Total      *int64 // rows matching the filters; set on the first page only
}
