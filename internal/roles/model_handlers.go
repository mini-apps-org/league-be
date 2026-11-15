package roles

import "time"

type CreateRoleRequest struct {
	Code        string  `json:"code" example:"LEAGUE_OPERATOR"`
	Name        string  `json:"name" example:"League Operator"`
	Description *string `json:"description,omitempty" example:"Runs the leagues they are assigned to."`
}

type UpdateRoleRequest struct {
	Name        *string `json:"name,omitempty" example:"Team Captain"`
	Description *string `json:"description,omitempty"`
}

type RoleResponse struct {
	ID          string    `json:"id"`
	Code        RoleCode  `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newRoleResponse(role Role) RoleResponse {
	return RoleResponse{ //nolint:staticcheck // S1016: explicit mapping keeps the db row and JSON response independent
		ID:          role.ID,
		Code:        role.Code,
		Name:        role.Name,
		Description: role.Description,
		Permissions: role.Permissions,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}
