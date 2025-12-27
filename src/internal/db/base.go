package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Base struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewBase(pool *pgxpool.Pool, timeout time.Duration) *Base {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Base{
		pool:    pool,
		timeout: timeout,
	}
}

func (b *Base) Q() Queryer {
	return instrumentedQueryer{q: b.pool}
}

func (b *Base) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, b.timeout)
}

func (b *Base) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	ctx, cancel := b.WithTimeout(ctx)
	defer cancel()

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
