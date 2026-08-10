// Package db содержит общие для всех доменов примитивы работы с БД:
// узкий интерфейс поверх pgx (Querier) и хелпер для транзакций.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier — общий контракт для *pgxpool.Pool и pgx.Tx.
// Репозитории принимают Querier вместо конкретного пула,
// чтобы уметь работать как вне транзакции, так и внутри неё.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// RunInTx открывает транзакцию на pool, передаёт её в fn и коммитит/роллбэкает
// в зависимости от того, вернула fn ошибку или нет.
func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op, если Commit уже прошёл

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	return nil
}

