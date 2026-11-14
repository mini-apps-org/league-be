package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

var (
	ErrNoSession = errors.New("auth: no session")
	ErrNoUser    = errors.New("auth: no user")
)

const userColumns = "id, email, email_verified, name, avatar_url, created_at, last_login_at"

type Repository interface {
	UpsertGoogleIdentity(ctx context.Context, p GoogleIdentityParams) (*User, bool, error)
	GetUserByID(ctx context.Context, id string) (*UserWithAccess, error)

	ReplaceSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string, absoluteCutoff, idleCutoff time.Time) (*Session, error)
	UpdateSessionLastSeenAt(ctx context.Context, id string) error
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID string) error
	DeleteExpiredSessions(ctx context.Context, absoluteCutoff, idleCutoff time.Time) (int64, error)
}

type repository struct {
	pgPool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) Repository {
	return &repository{pgPool: pool}
}

func (r *repository) UpsertGoogleIdentity(ctx context.Context, p GoogleIdentityParams) (*User, bool, error) {
	const query = `
INSERT INTO users (google_sub, email, email_verified, name, avatar_url, hd)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (google_sub) DO UPDATE SET
    email          = $2,
    email_verified = $3,
    name           = $4,
    avatar_url     = $5,
    hd             = $6,
    last_login_at  = now()
RETURNING ` + userColumns + `, (xmax = 0) AS is_new_user`
	args := []any{
		p.Subject,       // $1 google_sub
		p.Email,         // $2 email
		p.EmailVerified, // $3 email_verified
		p.Name,          // $4 name
		p.AvatarURL,     // $5 avatar_url
		p.HD,            // $6 hd
	}

	// Scanned explicitly: is_new_user has no field on User, so RowToStructByName
	// would reject the extra column. Keep this list in userColumns order.
	var u User
	var isNewUser bool
	err := r.pgPool.QueryRow(ctx, query, args...).Scan(
		&u.ID,            // id
		&u.Email,         // email
		&u.EmailVerified, // email_verified
		&u.Name,          // name
		&u.AvatarURL,     // avatar_url
		&u.CreatedAt,     // created_at
		&u.LastLoginAt,   // last_login_at
		&isNewUser,       // (xmax = 0)
	)
	if err != nil {
		return nil, false, fmt.Errorf("auth: upsert google user: %w", err)
	}

	return &u, isNewUser, nil
}

// GetUserByID returns the user with its role and permission codes in one
// statement. The subqueries correlate on users.id and COALESCE to '{}', so a
// user with no role gets empty arrays rather than NULL.
func (r *repository) GetUserByID(ctx context.Context, id string) (*UserWithAccess, error) {
	const query = `
SELECT ` + userColumns + `,
	COALESCE((SELECT array_agg(ro.code ORDER BY ro.code)
	          FROM users_roles ur
	          JOIN roles ro ON ro.id = ur.role_id
	          WHERE ur.user_id = users.id), '{}') AS roles,
	COALESCE((SELECT array_agg(DISTINCT p.code ORDER BY p.code)
	          FROM users_roles ur
	          JOIN roles_permissions rp ON rp.role_id = ur.role_id
	          JOIN permissions p        ON p.id = rp.permission_id
	          WHERE ur.user_id = users.id), '{}') AS permissions
FROM users
WHERE id = $1`
	args := []any{
		id,
	}

	user, err := repo.QueryOne[UserWithAccess](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoUser
	}
	if err != nil {
		return nil, fmt.Errorf("auth: user by id: %w", err)
	}
	return user, nil
}

// ReplaceSession enforces one device per user: user_id is unique, so a new
// sign-in overwrites the previous device's row and its sid stops resolving.
// One statement, so two concurrent sign-ins cannot both keep a session.
func (r *repository) ReplaceSession(ctx context.Context, s Session) error {
	const query = `
INSERT INTO users_sessions (id, user_id) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET
    id           = EXCLUDED.id,
    created_at   = now(),
    last_seen_at = now()`
	args := []any{
		s.ID,     // $1 id
		s.UserID, // $2 user_id
	}

	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("auth: replace session: %w", err)
	}
	return nil
}

func (r *repository) GetSession(ctx context.Context, id string, absoluteCutoff, idleCutoff time.Time) (*Session, error) {
	const query = `
SELECT id, user_id, created_at, last_seen_at
FROM users_sessions
WHERE id = $1 AND created_at > $2 AND last_seen_at > $3`
	args := []any{
		id,             // $1 id
		absoluteCutoff, // $2 created_at floor
		idleCutoff,     // $3 last_seen_at floor
	}

	session, err := repo.QueryOne[Session](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get session: %w", err)
	}
	return session, nil
}

func (r *repository) UpdateSessionLastSeenAt(ctx context.Context, id string) error {
	const query = `UPDATE users_sessions SET last_seen_at = now() WHERE id = $1`
	args := []any{
		id,
	}

	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("auth: update session last_seen_at: %w", err)
	}
	return nil
}

func (r *repository) DeleteSession(ctx context.Context, id string) error {
	const query = `DELETE FROM users_sessions WHERE id = $1`
	args := []any{
		id,
	}

	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

func (r *repository) DeleteUserSessions(ctx context.Context, userID string) error {
	const query = `DELETE FROM users_sessions WHERE user_id = $1`
	args := []any{
		userID,
	}

	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("auth: delete user sessions: %w", err)
	}
	return nil
}

func (r *repository) DeleteExpiredSessions(ctx context.Context, absoluteCutoff, idleCutoff time.Time) (int64, error) {
	const query = `DELETE FROM users_sessions WHERE created_at <= $1 OR last_seen_at <= $2`
	args := []any{
		absoluteCutoff, // $1 created_at ceiling
		idleCutoff,     // $2 last_seen_at ceiling
	}

	tag, err := r.pgPool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("auth: delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
