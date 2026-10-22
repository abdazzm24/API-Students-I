package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager struct {
	pool        *pgxpool.Pool
	maxAttempts int
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{
		pool:        pool,
		maxAttempts: 3,
	}
}

func (m *TxManager) WithTx(ctx context.Context, fn func(DBTX) error) error {
	var last error

	for attempt := 0; attempt < m.maxAttempts; attempt++ {
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		last = runTx(ctx, tx, fn)

		var pgErr *pgconn.PgError
		if !errors.As(last, &pgErr) ||
			(pgErr.Code != "40001" && pgErr.Code != "40P01") ||
			attempt+1 == m.maxAttempts {
			return last
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}

	return last
}

func runTx(ctx context.Context, tx pgx.Tx, fn func(DBTX) error) error {
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

var _ DBTX = (pgx.Tx)(nil)