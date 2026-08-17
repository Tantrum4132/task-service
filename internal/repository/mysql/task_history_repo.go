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

type taskHistoryRepository struct {
	db *sql.DB
}

func NewTaskHistoryRepository(db *sql.DB) repository.TaskHistoryRepository {
	return &taskHistoryRepository{db: db}
}

func (r *taskHistoryRepository) getExec(exec repository.DBEngine) repository.DBEngine {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *taskHistoryRepository) CreateTaskHistory(ctx context.Context, exec repository.DBEngine, history *model.TaskHistory) error {
	if history == nil {
		return errors.New("task history cannot be nil")
	}

	if history.CreatedAt.IsZero() {
		return repository.ErrInvalidTaskHistoryPayload
	}

	query := `
		INSERT INTO task_history (task_id, changed_by, changes, created_at)
		VALUES (?, ?, ?, ?)
	`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query,
		history.TaskID,
		history.ChangedBy,
		history.Changes,
		history.CreatedAt,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrForeignKeyFailed {
			return repository.ErrForeignKeyViolation
		}
		return fmt.Errorf("failed to insert task history: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get task history insert id: %w", err)
	}
	history.ID = id

	return nil
}

func (r *taskHistoryRepository) GetHistoryByTaskID(ctx context.Context, exec repository.DBEngine, filter model.TaskHistoryFilter) ([]model.TaskHistory, error) {
	if filter.TaskID == 0 {
		return nil, repository.ErrTaskIDRequired
	}

	query := `
		SELECT id, task_id, changed_by, changes, created_at
		FROM task_history
		WHERE task_id = ?
		ORDER BY created_at DESC, id DESC
	`
	args := []any{filter.TaskID}

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
		return nil, fmt.Errorf("failed to query task history: %w", err)
	}
	defer rows.Close()

	capacity := 0
	if filter.Limit > 0 {
		capacity = filter.Limit
	}
	historyList := make([]model.TaskHistory, 0, capacity)

	for rows.Next() {
		var h model.TaskHistory
		err := rows.Scan(
			&h.ID,
			&h.TaskID,
			&h.ChangedBy,
			&h.Changes,
			&h.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task history row: %w", err)
		}
		historyList = append(historyList, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task history rows iteration error: %w", err)
	}

	return historyList, nil
}
