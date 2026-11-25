package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

// Querier is what a repository runs its SQL on. The pool and a transaction both
// implement it; Begin on a transaction opens a savepoint, so pgx.BeginFunc nests.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

var (
	_ Querier = (*pgxpool.Pool)(nil)
	_ Querier = pgx.Tx(nil)
)

// Transactor lets a service run several repository calls as one unit: every call
// made with txCtx joins the transaction, and fn returning an error rolls it all back.
type Transactor interface {
	RunInTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

type transactor struct {
	pgPool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) Transactor {
	return &transactor{pgPool: pool}
}

// RunInTransaction commits when fn returns nil and rolls back otherwise, including
// on panic. Called with a txCtx it opens a savepoint inside the outer transaction.
func (t *transactor) RunInTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	err := pgx.BeginFunc(ctx, ConnFromContext(ctx, t.pgPool), func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, config.ContextKeyTransaction, tx))
	})
	if err != nil {
		return fmt.Errorf("db: run in transaction: %w", err)
	}
	return nil
}

// ConnFromContext returns the transaction carried by ctx, or pool when there is none.
func ConnFromContext(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(config.ContextKeyTransaction).(pgx.Tx); ok {
		return tx
	}
	return pool
}
