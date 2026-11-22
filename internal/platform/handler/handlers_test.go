package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The success body is the frontend's contract. A dropped omitempty would send "metadata":
// null on every single item, or "total": null / "next_cursor": null where the client
// expects the key to be absent; has_more must always agree with next_cursor.
func TestReturnWithJSONSuccessResponse(t *testing.T) {
	total := int64(87)
	next := "c1"

	tests := []struct {
		name string
		body any
		want string
	}{
		{"single item has no metadata",
			SuccessResponse[string]{Data: "team"},
			`{"data":"team"}`},
		{"unpaged list counts its rows",
			SuccessResponse[[]string]{Data: []string{"a", "b"}, Metadata: NewMetadata(2)},
			`{"data":["a","b"],"metadata":{"count":2,"has_more":false}}`},
		{"first page of a paged list",
			SuccessResponse[[]string]{Data: []string{"a", "b"}, Metadata: NewPageMetadata(2, &total, &next)},
			`{"data":["a","b"],"metadata":{"count":2,"total":87,"has_more":true,"next_cursor":"c1"}}`},
		{"middle page has no total",
			SuccessResponse[[]string]{Data: []string{"a", "b"}, Metadata: NewPageMetadata(2, nil, &next)},
			`{"data":["a","b"],"metadata":{"count":2,"has_more":true,"next_cursor":"c1"}}`},
		{"last page: has_more false, no next_cursor",
			SuccessResponse[[]string]{Data: []string{"a", "b"}, Metadata: NewPageMetadata(2, nil, nil)},
			`{"data":["a","b"],"metadata":{"count":2,"has_more":false}}`},
		{"single page: total and no more",
			SuccessResponse[[]string]{Data: []string{}, Metadata: NewPageMetadata(0, new(int64), nil)},
			`{"data":[],"metadata":{"count":0,"total":0,"has_more":false}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			(&Handler{}).ReturnWithJSON(recorder, http.StatusOK, tt.body)

			if got := recorder.Body.String(); got != tt.want+"\n" {
				t.Errorf("body = %s\nwant   %s", got, tt.want)
			}
		})
	}
}
