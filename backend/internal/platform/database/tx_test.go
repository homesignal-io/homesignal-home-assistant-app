package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestWithinTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	err := withinTx(
		context.Background(),
		func(context.Context, *sql.TxOptions) (transaction, error) {
			return tx, nil
		},
		nil,
		func(context.Context, transaction) error {
			return nil
		},
	)
	if err != nil {
		t.Fatalf("withinTx returned error: %v", err)
	}
	if tx.commits != 1 {
		t.Fatalf("expected one commit, got %d", tx.commits)
	}
	if tx.rollbacks != 0 {
		t.Fatalf("expected no rollback, got %d", tx.rollbacks)
	}
}

func TestWithinTxRollsBackOnFunctionError(t *testing.T) {
	tx := &fakeTx{}
	expectedErr := errors.New("boom")
	err := withinTx(
		context.Background(),
		func(context.Context, *sql.TxOptions) (transaction, error) {
			return tx, nil
		},
		nil,
		func(context.Context, transaction) error {
			return expectedErr
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected function error, got %v", err)
	}
	if tx.commits != 0 {
		t.Fatalf("expected no commit, got %d", tx.commits)
	}
	if tx.rollbacks != 1 {
		t.Fatalf("expected one rollback, got %d", tx.rollbacks)
	}
}

func TestWithinTxReturnsCommitError(t *testing.T) {
	tx := &fakeTx{commitErr: errors.New("commit failed")}
	err := withinTx(
		context.Background(),
		func(context.Context, *sql.TxOptions) (transaction, error) {
			return tx, nil
		},
		nil,
		func(context.Context, transaction) error {
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected commit error")
	}
	if tx.commits != 1 {
		t.Fatalf("expected one commit attempt, got %d", tx.commits)
	}
}

type fakeTx struct {
	commits     int
	rollbacks   int
	commitErr   error
	rollbackErr error
}

func (t *fakeTx) Commit() error {
	t.commits++
	return t.commitErr
}

func (t *fakeTx) Rollback() error {
	t.rollbacks++
	return t.rollbackErr
}
