package permissions

import (
	"errors"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

var ErrPermissionNotFound = errors.New("permissions: not found")

// permissionErrorMappings is how the service serves each sentinel the repository
// returns.
var permissionErrorMappings = []handler.ErrorMapping{
	{Sentinel: ErrPermissionNotFound, Code: handler.CodeNotFound, Message: "permission not found"},
}
