package permissions

import "time"

type PermissionResponse struct {
	ID          string    `json:"id"`
	Code        Code      `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newPermissionResponse(permission Permission) PermissionResponse {
	return PermissionResponse{ //nolint:staticcheck // S1016: explicit mapping keeps the db row and JSON response independent
		ID:          permission.ID,
		Code:        permission.Code,
		Name:        permission.Name,
		Description: permission.Description,
		CreatedAt:   permission.CreatedAt,
		UpdatedAt:   permission.UpdatedAt,
	}
}
