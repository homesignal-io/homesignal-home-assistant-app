package database

import (
	"context"
	"database/sql"
	"fmt"
)

type TransactionFunc func(ctx context.Context, tx *sql.Tx) error

func WithinTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn TransactionFunc) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}
	return withinTx(
		ctx,
		func(ctx context.Context, opts *sql.TxOptions) (transaction, error) {
			return db.BeginTx(ctx, opts)
		},
		opts,
		func(ctx context.Context, tx transaction) error {
			sqlTx, ok := tx.(*sql.Tx)
			if !ok {
				return fmt.Errorf("unexpected transaction type %T", tx)
			}
			return fn(ctx, sqlTx)
		},
	)
}

type transaction interface {
	Commit() error
	Rollback() error
}

type beginTxFunc func(context.Context, *sql.TxOptions) (transaction, error)
type genericTransactionFunc func(context.Context, transaction) error

func withinTx(ctx context.Context, begin beginTxFunc, opts *sql.TxOptions, fn genericTransactionFunc) error {
	if begin == nil {
		return fmt.Errorf("transaction begin function is required")
	}
	if fn == nil {
		return fmt.Errorf("transaction function is required")
	}

	tx, err := begin(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
