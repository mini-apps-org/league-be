package model

type CursorReponse[T any] struct {
	Items      []T
	Total      int64
	NextCursor *string
}
