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

// Константы кодов ошибок MySQL
const (
	mysqlErrDuplicateEntry   = 1062
	mysqlErrForeignKeyFailed = 1452
)

type taskCommentRepository struct {
	db *sql.DB
}

func NewTaskCommentRepository(db *sql.DB) repository.TaskCommentRepository {
	return &taskCommentRepository{
		db: db,
	}
}

func (r *taskCommentRepository) getExec(exec repository.DBEngine) repository.DBEngine {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *taskCommentRepository) CreateComment(ctx context.Context, exec repository.DBEngine, comment *model.TaskComment) error {
	if comment == nil {
		return errors.New("comment cannot be nil")
	}

	if comment.CreatedAt.IsZero() {
		return repository.ErrInvalidCommentPayload
	}

	query := `
		INSERT INTO task_comments (task_id, user_id, content, created_at)
		VALUES (?, ?, ?, ?)
	`
	runner := r.getExec(exec)

	result, err := runner.ExecContext(ctx, query, comment.TaskID, comment.UserID, comment.Content, comment.CreatedAt)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrForeignKeyFailed {
			return repository.ErrForeignKeyViolation
		}
		return fmt.Errorf("failed to insert comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id for comment: %w", err)
	}

	comment.ID = id
	return nil
}

func (r *taskCommentRepository) GetCommentByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.TaskComment, error) {
	query := `
		SELECT id, task_id, user_id, content, created_at
		FROM task_comments
		WHERE id = ?
	`
	runner := r.getExec(exec)

	var comment model.TaskComment
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&comment.ID,
		&comment.TaskID,
		&comment.UserID,
		&comment.Content,
		&comment.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrCommentNotFound
		}
		return nil, fmt.Errorf("failed to get comment by id: %w", err)
	}

	return &comment, nil
}

func (r *taskCommentRepository) ListCommentsByTaskID(ctx context.Context, exec repository.DBEngine, taskID int64) ([]model.TaskComment, error) {
	query := `
		SELECT id, task_id, user_id, content, created_at
		FROM task_comments
		WHERE task_id = ?
		ORDER BY created_at ASC, id ASC
	`
	runner := r.getExec(exec)

	rows, err := runner.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query comments by task id: %w", err)
	}
	defer rows.Close()

	comments := make([]model.TaskComment, 0, 10)
	for rows.Next() {
		var c model.TaskComment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.UserID, &c.Content, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan comment row: %w", err)
		}
		comments = append(comments, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error during comment list iteration: %w", err)
	}

	return comments, nil
}

func (r *taskCommentRepository) DeleteComment(ctx context.Context, exec repository.DBEngine, id int64) error {
	query := `DELETE FROM task_comments WHERE id = ?`
	runner := r.getExec(exec)

	result, err := runner.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected on comment deletion: %w", err)
	}

	if rowsAffected == 0 {
		return repository.ErrCommentNotFound
	}

	return nil
}
