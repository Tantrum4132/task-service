package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
)

type userRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) repository.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) getExec(exec repository.DBEngine) repository.DBEngine {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *userRepo) Create(ctx context.Context, exec repository.DBEngine, user *model.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	const query = `
		INSERT INTO users (email, password_hash, name, created_at)
		VALUES (?, ?, ?, ?)
	`
	runner := r.getExec(exec)

	result, err := runner.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.CreatedAt,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrDuplicateEntry {
			return repository.ErrEmailAlreadyExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	user.ID = id
	return nil
}

func (r *userRepo) FindByEmail(ctx context.Context, exec repository.DBEngine, email string) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, created_at
		FROM users
		WHERE email = ?
		LIMIT 1
	`
	runner := r.getExec(exec)
	return r.scanUser(runner.QueryRowContext(ctx, query, email))
}

func (r *userRepo) FindByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, created_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`
	runner := r.getExec(exec)
	return r.scanUser(runner.QueryRowContext(ctx, query, id))
}

func (r *userRepo) Update(ctx context.Context, exec repository.DBEngine, user *model.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	const query = `
		UPDATE users
		SET name = ?, email = ?
		WHERE id = ?
	`
	runner := r.getExec(exec)

	result, err := runner.ExecContext(ctx, query, user.Name, user.Email, user.ID)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrDuplicateEntry {
			return repository.ErrEmailAlreadyExists
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return repository.ErrUserNotFound
	}

	return nil
}

func (r *userRepo) Delete(ctx context.Context, exec repository.DBEngine, id int64) error {
	const query = `DELETE FROM users WHERE id = ?`
	runner := r.getExec(exec)

	result, err := runner.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return repository.ErrUserNotFound
	}

	return nil
}

func (r *userRepo) scanUser(row *sql.Row) (*model.User, error) {
	var user model.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	return &user, nil
}
