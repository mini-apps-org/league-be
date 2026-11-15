package permissions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

var ErrPermissionNotFound = errors.New("permissions: not found")

const permissionListLimit = 200

const permissionColumns = "id, code, name, description, created_at, updated_at"

type Repository interface {
	GetPermissionByID(ctx context.Context, id string) (*Permission, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
	GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type repository struct {
	pgPool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) Repository {
	return &repository{pgPool: pool}
}

func (r *repository) GetPermissionByID(ctx context.Context, id string) (*Permission, error) {
	const query = `SELECT ` + permissionColumns + ` FROM permissions WHERE id = $1`
	args := []any{
		id,
	}

	permission, err := repo.QueryOne[Permission](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPermissionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("permissions: by id %s: %w", id, err)
	}
	return permission, nil
}

func (r *repository) ListPermissions(ctx context.Context) ([]Permission, error) {
	const query = `SELECT ` + permissionColumns + ` FROM permissions ORDER BY code LIMIT $1`
	args := []any{
		permissionListLimit,
	}

	permissions, err := repo.QueryAll[Permission](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("permissions: all: %w", err)
	}
	return permissions, nil
}

func (r *repository) GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error) {
	const query = `
SELECT DISTINCT p.code
FROM users_roles ur
JOIN roles_permissions rp ON rp.role_id = ur.role_id
JOIN permissions p        ON p.id = rp.permission_id
WHERE ur.user_id = $1
ORDER BY p.code`
	args := []any{
		userID,
	}

	codes, err := repo.QueryStrings(ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("permissions: user codes %s: %w", userID, err)
	}
	return codes, nil
}
