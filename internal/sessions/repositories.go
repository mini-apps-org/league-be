package sessions

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/db"
)

// leagueSessionColumns reads FROM league_sessions ls. The two derived values are
// correlated subqueries: cheap at a handful of sessions per league, and covered by
// team_matches_division_id_week_no_idx.
const leagueSessionColumns = `
    ls.id, ls.league_id, ls.name, ls.session_type, ls.season_year, ls.starts_on, ls.ends_on,
    ls.status, ls.is_active,
    (SELECT count(DISTINCT tm.week_no)
       FROM team_matches tm JOIN divisions d ON d.id = tm.division_id
      WHERE d.season_id = ls.id) AS week_count,
    EXISTS (SELECT 1
              FROM team_matches tm JOIN divisions d ON d.id = tm.division_id
             WHERE d.season_id = ls.id AND tm.status = 'submitted') AS has_submitted_results,
    ls.created_at, ls.updated_at`

type Repository interface {
	GetDefaultLeagueID(ctx context.Context) (string, error)
	ListSessions(ctx context.Context, leagueID string, params ListLeagueSessionsServiceParams) ([]LeagueSession, error)
	CountSessions(ctx context.Context, leagueID string, params ListLeagueSessionsServiceParams) (int64, error)
	GetSessionByID(ctx context.Context, leagueID, leagueSessionID string) (*LeagueSession, error)
	CreateSession(ctx context.Context, leagueID string, params CreateLeagueSessionServiceParams) (*LeagueSession, error)
	UpdateSession(ctx context.Context, leagueID, leagueSessionID string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error)
	SoftDeleteSession(ctx context.Context, leagueID, leagueSessionID string) error
	PermanentDeleteSession(ctx context.Context, leagueID, leagueSessionID string) error
	BulkSoftDeleteSessions(ctx context.Context, leagueID string, ids []string) (*BulkDeleteLeagueSessionsResult, error)
	BulkPermanentDeleteSessions(ctx context.Context, leagueID string, ids []string) (*BulkDeleteLeagueSessionsResult, error)
	ListSessionWeeks(ctx context.Context, leagueID, leagueSessionID string) ([]LeagueSessionWeek, error)
}

type repository struct {
	pgPool *pgxpool.Pool
	logger *slog.Logger
}

func NewRepo(pool *pgxpool.Pool, logger *slog.Logger) Repository {
	return &repository{pgPool: pool, logger: logger.With(slog.String("component", "sessions.repository"))}
}

// conn is the transaction carried by ctx, if a service opened one, else the pool.
func (r *repository) conn(ctx context.Context) db.Querier {
	return db.ConnFromContext(ctx, r.pgPool)
}
