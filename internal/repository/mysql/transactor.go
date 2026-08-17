package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Tantrum4132/task-service/internal/repository"
	"github.com/Tantrum4132/task-service/internal/service"
)

type transactor struct {
	db *sql.DB
}

// NewTransactor создает новый экземпляр транзактора MySQL.
func NewTransactor(db *sql.DB) service.Transactor {
	return &transactor{
		db: db,
	}
}

// WithinTransaction выполняет функцию fn внутри SQL-транзакции.
func (t *transactor) WithinTransaction(ctx context.Context, fn func(exec repository.DBEngine) error) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("execute in transaction: %w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
