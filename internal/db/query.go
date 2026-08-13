package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Querier is satisfied by both *sql.DB and *sql.Tx — every helper below takes one, so callers
// can pass either a pool-level connection or an in-flight transaction without duplicating code.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ScanFunc reads exactly one row. Hand-written per row type next to its queries — no reflection,
// no struct-tag magic.
type ScanFunc[T any] func(*sql.Rows) (T, error)

// Query runs a SELECT and scans every row with scan. Always parameterized — args are bound as
// `?` placeholders by the driver, never string-concatenated into query.
func Query[T any](ctx context.Context, q Querier, query string, scan ScanFunc[T], args ...any) ([]T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration: %w", err)
	}
	return out, nil
}

// QueryOne runs a SELECT and returns the first row, or nil if there were none.
func QueryOne[T any](ctx context.Context, q Querier, query string, scan ScanFunc[T], args ...any) (*T, error) {
	rows, err := Query(ctx, q, query, scan, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

type ExecResult struct {
	RowsAffected int64
}

// Execute runs an INSERT/UPDATE/DELETE.
func Execute(ctx context.Context, q Querier, query string, args ...any) (ExecResult, error) {
	res, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return ExecResult{}, fmt.Errorf("exec: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return ExecResult{}, fmt.Errorf("rows affected: %w", err)
	}
	return ExecResult{RowsAffected: affected}, nil
}

// WithTransaction runs fn inside a single transaction: commits on success, rolls back on error
// or panic.
func WithTransaction[T any](ctx context.Context, pool *sql.DB, fn func(tx *sql.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return zero, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	result, err := fn(tx)
	if err != nil {
		_ = tx.Rollback()
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, fmt.Errorf("commit tx: %w", err)
	}
	return result, nil
}
