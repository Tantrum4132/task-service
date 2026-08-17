package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"

	"github.com/go-sql-driver/mysql"
)

type taskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) repository.TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) getExec(exec repository.DBEngine) repository.DBEngine {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *taskRepository) CreateTask(ctx context.Context, exec repository.DBEngine, task *model.Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	// Если версия не передана явно (0), инициализируем начальной версией 1
	if task.Version == 0 {
		task.Version = 1
	}

	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return repository.ErrInvalidTaskPayload
	}

	query := `
		INSERT INTO tasks (
			team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query,
		task.TeamID,
		task.Title,
		task.Description,
		task.Status,
		task.CreatedBy,
		task.AssigneeID,
		task.CreatedAt,
		task.UpdatedAt,
		task.ClosedAt,
		task.Version,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrForeignKeyFailed {
			return repository.ErrForeignKeyViolation
		}
		return fmt.Errorf("failed to create task: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	task.ID = id

	return nil
}

func (r *taskRepository) GetTaskByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.Task, error) {
	query := `
		SELECT id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version
		FROM tasks
		WHERE id = ?
	`
	runner := r.getExec(exec)
	var task model.Task

	err := runner.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.TeamID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.CreatedBy,
		&task.AssigneeID,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.ClosedAt,
		&task.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task by id: %w", err)
	}

	return &task, nil
}

func (r *taskRepository) UpdateTask(ctx context.Context, exec repository.DBEngine, task *model.Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	query := `
		UPDATE tasks
		SET title = ?,
		    description = ?,
		    status = ?,
		    assignee_id = ?,
		    updated_at = ?,
		    closed_at = ?,
		    version = version + 1
		WHERE id = ? AND version = ?
	`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query,
		task.Title,
		task.Description,
		task.Status,
		task.AssigneeID,
		task.UpdatedAt,
		task.ClosedAt,
		task.ID,
		task.Version,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrForeignKeyFailed {
			return repository.ErrForeignKeyViolation
		}
		return fmt.Errorf("failed to update task: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		var exists bool
		errCheck := runner.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE id = ?)", task.ID).Scan(&exists)
		if errCheck == nil && !exists {
			return repository.ErrTaskNotFound
		}
		return repository.ErrTaskConflict
	}

	task.Version++

	return nil
}

func (r *taskRepository) DeleteTask(ctx context.Context, exec repository.DBEngine, id int64) error {
	query := `DELETE FROM tasks WHERE id = ?`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return repository.ErrTaskNotFound
	}

	return nil
}

func (r *taskRepository) ListTasks(ctx context.Context, exec repository.DBEngine, filter model.TaskFilter) ([]model.Task, error) {
	if filter.TeamID == 0 {
		return nil, repository.ErrTeamIDRequired
	}

	query := `
		SELECT id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version
		FROM tasks
		WHERE team_id = ?
	`
	args := []any{filter.TeamID}

	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, *filter.Status)
	}

	if filter.AssigneeID != nil {
		query += " AND assignee_id = ?"
		args = append(args, *filter.AssigneeID)
	}

	// Детерминированная сортировка по created_at и id
	query += " ORDER BY created_at DESC, id DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	runner := r.getExec(exec)
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	capacity := 0
	if filter.Limit > 0 {
		capacity = filter.Limit
	}
	tasks := make([]model.Task, 0, capacity)

	for rows.Next() {
		var t model.Task
		err := rows.Scan(
			&t.ID,
			&t.TeamID,
			&t.Title,
			&t.Description,
			&t.Status,
			&t.CreatedBy,
			&t.AssigneeID,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.ClosedAt,
			&t.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task row: %w", err)
		}
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return tasks, nil
}
