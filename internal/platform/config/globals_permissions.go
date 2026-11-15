package config

import (
	"fmt"
	"net/http"
	"strings"
)

type PermissionFeature string

const (
	PermissionFeatureRoles       PermissionFeature = "ROLE"
	PermissionFeaturePermissions PermissionFeature = "PERMISSION"
)

type PermissionAction string

const (
	PermissionActionRead   PermissionAction = "read"
	PermissionActionCreate PermissionAction = "create"
	PermissionActionUpdate PermissionAction = "update"
	PermissionActionDelete PermissionAction = "delete"
)

var HTTPMethodToPermissionAction = map[string]PermissionAction{
	http.MethodGet:    PermissionActionRead,
	http.MethodPost:   PermissionActionCreate,
	http.MethodPut:    PermissionActionUpdate,
	http.MethodPatch:  PermissionActionUpdate,
	http.MethodDelete: PermissionActionDelete,
}

func GetPermissionByFeatureAndAction(feature PermissionFeature, action PermissionAction) string {
	return fmt.Sprintf(`%s.%s`, strings.ToLower(string(feature)), string(action))
}
