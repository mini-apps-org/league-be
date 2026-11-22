package roles

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// roleCodePattern matches the seeded codes: UPPER_SNAKE_CASE, 2-50 characters.
var roleCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,49}$`)

const (
	maxRoleNameLength        = 100
	maxRoleDescriptionLength = 500
)

type Service interface {
	CreateRole(ctx context.Context, params CreateRoleServiceParams) (*Role, error)
	GetRole(ctx context.Context, id string) (*Role, error)
	ListRoles(ctx context.Context) ([]Role, error)
	UpdateRole(ctx context.Context, id string, params UpdateRoleServiceParams) (*Role, error)
	DeleteRole(ctx context.Context, id string) error
	PermanentDeleteRole(ctx context.Context, id string) error
}

type service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) Service {
	return &service{repo: repo, logger: logger.With(slog.String("component", "roles.service"))}
}

func (s *service) CreateRole(ctx context.Context, params CreateRoleServiceParams) (*Role, error) {
	params.Code = RoleCode(strings.TrimSpace(string(params.Code)))
	params.Name = strings.TrimSpace(params.Name)

	if !roleCodePattern.MatchString(string(params.Code)) {
		return nil, newValidationError("code must be 2-50 characters of A-Z, 0-9 and _, starting with a letter")
	}
	if err := validateRoleFields(&params.Name, params.Description); err != nil {
		return nil, err
	}

	role, err := s.repo.CreateRole(ctx, params)
	return role, asAPIError(err)
}

func (s *service) GetRole(ctx context.Context, id string) (*Role, error) {
	role, err := s.repo.GetRoleByID(ctx, id)
	return role, asAPIError(err)
}

func (s *service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *service) UpdateRole(ctx context.Context, id string, params UpdateRoleServiceParams) (*Role, error) {
	if params.Name == nil && params.Description == nil {
		return nil, newValidationError("supply at least one of name or description")
	}
	if params.Name != nil {
		trimmed := strings.TrimSpace(*params.Name)
		params.Name = &trimmed
	}
	if err := validateRoleFields(params.Name, params.Description); err != nil {
		return nil, err
	}

	role, err := s.repo.UpdateRole(ctx, id, params)
	return role, asAPIError(err)
}

func (s *service) DeleteRole(ctx context.Context, id string) error {
	role, err := s.repo.GetRoleByID(ctx, id)
	if err != nil {
		return asAPIError(err)
	}
	if role.Code == CodeSuperAdmin {
		return superAdminProtectedError()
	}

	return asAPIError(s.repo.SoftDeleteRole(ctx, id))
}

// PermanentDeleteRole needs no SUPER_ADMIN check: DeleteRole refuses to soft-delete it, and
// only a soft-deleted role can be removed.
func (s *service) PermanentDeleteRole(ctx context.Context, id string) error {
	return asAPIError(s.repo.PermanentDeleteRole(ctx, id))
}

// validateRoleFields checks the supplied (non-nil) fields. Lengths count runes, not
// bytes, so Vietnamese names are not cut short.
func validateRoleFields(name, description *string) error {
	if name != nil {
		if *name == "" {
			return newValidationError("name is required")
		}
		if utf8.RuneCountInString(*name) > maxRoleNameLength {
			return newValidationError(fmt.Sprintf("name must be at most %d characters", maxRoleNameLength))
		}
	}
	if description != nil && utf8.RuneCountInString(*description) > maxRoleDescriptionLength {
		return newValidationError(fmt.Sprintf("description must be at most %d characters", maxRoleDescriptionLength))
	}
	return nil
}

func newValidationError(message string) error {
	return handler.ErrorWithMessage(handler.CodeValidationFailed, message, nil)
}

func superAdminProtectedError() error {
	return handler.ErrorWithMessage(handler.CodeConflict,
		"SUPER_ADMIN grants every permission and is managed by migration, not over HTTP", nil)
}

func asAPIError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrRoleNotFound):
		return handler.Error(handler.CodeNotFound, err)
	case errors.Is(err, ErrRoleNotSoftDeleted):
		return handler.ErrorWithMessage(handler.CodeConflict, "delete the role before deleting it permanently", err)
	case errors.Is(err, ErrDuplicateRoleCode):
		return handler.ErrorWithMessage(handler.CodeConflict, "a role with that code already exists", err)
	default:
		return err
	}
}
