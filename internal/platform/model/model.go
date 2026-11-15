package model

type CursorResponse[T any] struct {
	Items      []T
	Total      int64
	NextCursor *string
}
