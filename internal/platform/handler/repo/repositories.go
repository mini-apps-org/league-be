package repo

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/nguyentanvinh/league-be/internal/platform/db"
)

func QueryAll[T any](ctx context.Context, conn db.Querier, query string, args ...any) ([]T, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []T{}
	}

	return out, nil
}

func QueryOne[T any](ctx context.Context, conn db.Querier, query string, args ...any) (*T, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func QueryStrings(ctx context.Context, conn db.Querier, query string, args ...any) ([]string, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// QueryCount runs a single-value count query, such as SELECT count(*) FROM … WHERE …, and
// returns the count.
func QueryCount(ctx context.Context, conn db.Querier, query string, args ...any) (int64, error) {
	var count int64
	if err := conn.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
