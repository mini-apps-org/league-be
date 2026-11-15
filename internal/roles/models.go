// Package roles owns role CRUD.
package roles

import "time"

type RoleCode string

// Seeded by the migration. Add codes; never rename them.
const (
	CodeSuperAdmin RoleCode = "SUPER_ADMIN"
	CodeCaptain    RoleCode = "CAPTAIN"
	CodePlayer     RoleCode = "PLAYER"
)

type Role struct {
	ID          string    `db:"id"`
	Code        RoleCode  `db:"code"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	Permissions []string  `db:"permissions"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type CreateRoleServiceParams struct {
	Code        RoleCode
	Name        string
	Description *string
}

// UpdateRoleServiceParams is a partial update: nil leaves a field unchanged,
// "" clears the description.
type UpdateRoleServiceParams struct {
	Name        *string
	Description *string
}
