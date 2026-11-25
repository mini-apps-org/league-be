package permissions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"

	"github.com/nguyentanvinh/league-be/internal/platform/db"
)

const permissionListLimit = 200

const permissionColumns = "id, code, name, description, created_at, updated_at"

type Repository interface {
	GetPermissionByID(ctx context.Context, id string) (*Permission, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
	GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type repository struct {
	pgPool *pgxpool.Pool
	logger *slog.Logger
}

func NewRepo(pool *pgxpool.Pool, logger *slog.Logger) Repository {
	return &repository{pgPool: pool, logger: logger.With(slog.String("component", "permissions.repository"))}
}

// conn is the transaction carried by ctx, if a service opened one, else the pool.
func (r *repository) conn(ctx context.Context) db.Querier {
	return db.ConnFromContext(ctx, r.pgPool)
}

func (r *repository) GetPermissionByID(ctx context.Context, id string) (*Permission, error) {
	const query = `SELECT ` + permissionColumns + ` FROM permissions WHERE id = $1`
	args := []any{
		id,
	}

	permission, err := repo.QueryOne[Permission](ctx, r.conn(ctx), query, args...)
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

	permissions, err := repo.QueryAll[Permission](ctx, r.conn(ctx), query, args...)
	if err != nil {
		return nil, fmt.Errorf("permissions: all: %w", err)
	}
	return permissions, nil
}

func (r *repository) GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error) {
	const query = `
SELECT DISTINCT p.code
FROM users_roles ur
JOIN roles ro             ON ro.id = ur.role_id AND NOT ro.is_deleted
JOIN roles_permissions rp ON rp.role_id = ur.role_id
JOIN permissions p        ON p.id = rp.permission_id
WHERE ur.user_id = $1
ORDER BY p.code`
	args := []any{
		userID,
	}

	codes, err := repo.QueryStrings(ctx, r.conn(ctx), query, args...)
	if err != nil {
		return nil, fmt.Errorf("permissions: user codes %s: %w", userID, err)
	}
	return codes, nil
}
