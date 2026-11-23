package roles

import (
	"errors"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// A wrapped sentinel must still map to its status and message: a lost mapping would
// serve 500 for a plain 404 or 409.
func TestMapFeatureErrorUnwrapsRoleSentinels(t *testing.T) {
	tests := []struct {
		sentinel    error
		wantCode    handler.ErrorCode
		wantMessage string
	}{
		{ErrRoleNotFound, handler.CodeNotFound, "role not found"},
		{ErrRoleNotSoftDeleted, handler.CodeConflict, "delete the role before deleting it permanently"},
		{ErrDuplicateRoleCode, handler.CodeConflict, "a role with that code already exists"},
	}

	for _, tt := range tests {
		t.Run(tt.sentinel.Error(), func(t *testing.T) {
			wrapped := errors.Join(errors.New("roles: update role"), tt.sentinel)

			var apiErr *handler.APIError
			if !errors.As(handler.MapFeatureError(wrapped, roleErrorMappings), &apiErr) {
				t.Fatalf("%v was not mapped to an APIError", tt.sentinel)
			}
			if apiErr.Code != tt.wantCode || apiErr.Message != tt.wantMessage {
				t.Errorf("got %d %q, want %d %q", apiErr.Code, apiErr.Message, tt.wantCode, tt.wantMessage)
			}
		})
	}
}
