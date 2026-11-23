package roles

import (
	"errors"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

var (
	ErrRoleNotFound       = errors.New("roles: not found")
	ErrDuplicateRoleCode  = errors.New("roles: duplicate code")
	ErrRoleNotSoftDeleted = errors.New("roles: not soft-deleted")
)

// roleErrorMappings is how the service serves each sentinel the repository returns.
var roleErrorMappings = []handler.ErrorMapping{
	{Sentinel: ErrRoleNotFound, Code: handler.CodeNotFound, Message: "role not found"},
	{Sentinel: ErrRoleNotSoftDeleted, Code: handler.CodeConflict, Message: "delete the role before deleting it permanently"},
	{Sentinel: ErrDuplicateRoleCode, Code: handler.CodeConflict, Message: "a role with that code already exists"},
}
