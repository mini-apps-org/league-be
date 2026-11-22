package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ReturnWithJSON finds a SuccessResponse through responseWithDefaultCount. Give
// withDefaultCount a pointer receiver and the values handlers pass stop matching: every
// single-item response would silently serve "count": null.
func TestReturnWithJSONSuccessResponse(t *testing.T) {
	two := int64(2)
	total := int64(87)
	next := "c1"

	tests := []struct {
		name string
		body any
		want string
	}{
		{"single item gets count 1, no metadata",
			SuccessResponse[string]{Data: "team"},
			`{"session_id":"","count":1,"data":"team"}` + "\n"},
		{"list keeps its count",
			SuccessResponse[[]string]{Count: &two, Data: []string{"a", "b"}},
			`{"session_id":"","count":2,"data":["a","b"]}` + "\n"},
		{"paged list carries next_cursor",
			SuccessResponse[[]string]{Count: &two, Data: []string{"a", "b"}, Metadata: &CursorMetadata{NextCursor: &next}},
			`{"session_id":"","count":2,"data":["a","b"],"metadata":{"next_cursor":"c1"}}` + "\n"},
		{"first page carries the total",
			SuccessResponse[[]string]{Count: &two, Data: []string{"a", "b"}, Metadata: &CursorMetadata{NextCursor: &next, Total: &total}},
			`{"session_id":"","count":2,"data":["a","b"],"metadata":{"next_cursor":"c1","total":87}}` + "\n"},
		{"last page has null next_cursor",
			SuccessResponse[[]string]{Count: &two, Data: []string{"a", "b"}, Metadata: &CursorMetadata{}},
			`{"session_id":"","count":2,"data":["a","b"],"metadata":{"next_cursor":null}}` + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			(&Handler{}).ReturnWithJSON(recorder, http.StatusOK, tt.body)

			if got := recorder.Body.String(); got != tt.want {
				t.Errorf("body = %s\nwant   %s", got, tt.want)
			}
		})
	}
}
