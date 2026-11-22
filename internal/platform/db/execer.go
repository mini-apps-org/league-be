package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Execer runs a statement that returns no rows. The pool and a transaction both
// implement it, so a repository helper can run the same statement on its own or as one
// step of a transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// A pgx upgrade that changed either Exec signature fails here, not in every helper.
var (
	_ Execer = (*pgxpool.Pool)(nil)
	_ Execer = pgx.Tx(nil)
)
