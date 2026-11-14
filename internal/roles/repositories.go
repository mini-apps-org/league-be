package roles

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

var (
	ErrRoleNotFound      = errors.New("roles: not found")
	ErrDuplicateRoleCode = errors.New("roles: duplicate code")
)

// pgUniqueViolationCode is Postgres SQLSTATE 23505.
const pgUniqueViolationCode = "23505"

const roleListLimit = 200

const roleColumns = `id, code, name, description, created_at, updated_at,
	COALESCE((SELECT array_agg(p.code ORDER BY p.code)
	          FROM roles_permissions rp
	          JOIN permissions p ON p.id = rp.permission_id
	          WHERE rp.role_id = roles.id), '{}') AS permissions`

type Repository interface {
	CreateRole(ctx context.Context, params CreateRoleServiceParams) (*Role, error)
	GetRoleByID(ctx context.Context, id string) (*Role, error)
	ListRoles(ctx context.Context) ([]Role, error)
	UpdateRole(ctx context.Context, id string, params UpdateRoleServiceParams) (*Role, error)
	DeleteRole(ctx context.Context, id string) error
}

type repository struct {
	pgPool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) Repository {
	return &repository{pgPool: pool}
}

func (r *repository) CreateRole(ctx context.Context, params CreateRoleServiceParams) (*Role, error) {
	const query = `
INSERT INTO roles (code, name, description)
VALUES ($1, $2, $3)
RETURNING ` + roleColumns
	args := []any{
		params.Code,        // $1 code
		params.Name,        // $2 name
		params.Description, // $3 description
	}

	role, err := repo.QueryOne[Role](ctx, r.pgPool, query, args...)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateRoleCode
	}
	if err != nil {
		return nil, fmt.Errorf("roles: create: %w", err)
	}
	return role, nil
}

func (r *repository) GetRoleByID(ctx context.Context, id string) (*Role, error) {
	const query = `SELECT ` + roleColumns + ` FROM roles WHERE id = $1`
	args := []any{
		id,
	}

	role, err := repo.QueryOne[Role](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("roles: by id %s: %w", id, err)
	}
	return role, nil
}

func (r *repository) ListRoles(ctx context.Context) ([]Role, error) {
	const query = `SELECT ` + roleColumns + ` FROM roles ORDER BY code LIMIT $1`
	args := []any{
		roleListLimit,
	}

	roles, err := repo.QueryAll[Role](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("roles: all: %w", err)
	}
	return roles, nil
}

// UpdateRole is one statement: COALESCE leaves a column alone when its argument is
// NULL, so concurrent PATCHes cannot lose an update.
func (r *repository) UpdateRole(ctx context.Context, id string, params UpdateRoleServiceParams) (*Role, error) {
	const query = `
UPDATE roles SET
    name        = COALESCE($2, name),
    description = COALESCE($3, description),
    updated_at  = now()
WHERE id = $1
RETURNING ` + roleColumns
	args := []any{
		id,                 // $1 id
		params.Name,        // $2 name, NULL leaves it
		params.Description, // $3 description, NULL leaves it
	}

	role, err := repo.QueryOne[Role](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("roles: update %s: %w", id, err)
	}
	return role, nil
}

func (r *repository) DeleteRole(ctx context.Context, id string) error {
	const query = `DELETE FROM roles WHERE id = $1`
	args := []any{
		id,
	}

	tag, err := r.pgPool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("roles: delete %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode
}
