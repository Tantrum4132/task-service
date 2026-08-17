package mysql_test

import (
	"context"
	"testing"
	"time"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	repos "github.com/Tantrum4132/task-service/internal/repository/mysql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskCommentRepository_CreateComment(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTaskCommentRepository(db)

	userID := createTestUser(ctx, t, db, "commenter@test.com", "Commenter")
	teamID := createTestTeam(ctx, t, db, userID, "Team A")
	taskID := createTestTask(ctx, t, db, teamID, userID, "Task 1")

	now := time.Now().UTC().Truncate(time.Second)

	t.Run("success create comment", func(t *testing.T) {
		comment := &model.TaskComment{
			TaskID:    taskID,
			UserID:    userID,
			Content:   "This is a test comment",
			CreatedAt: now,
		}

		err := repo.CreateComment(ctx, nil, comment)
		require.NoError(t, err)
		assert.Greater(t, comment.ID, int64(0))

		// Проверяем прямое чтение из базы
		var dbContent string
		var dbCreatedAt time.Time
		err = db.QueryRowContext(ctx, "SELECT content, created_at FROM task_comments WHERE id = ?", comment.ID).
			Scan(&dbContent, &dbCreatedAt)
		require.NoError(t, err)
		assert.Equal(t, "This is a test comment", dbContent)
		assert.WithinDuration(t, now, dbCreatedAt.UTC(), time.Second)
	})

	t.Run("fail zero created_at", func(t *testing.T) {
		comment := &model.TaskComment{
			TaskID:  taskID,
			UserID:  userID,
			Content: "Invalid time comment",
		}

		err := repo.CreateComment(ctx, nil, comment)
		require.ErrorIs(t, err, repository.ErrInvalidCommentPayload)
	})

	t.Run("fail foreign key constraint (non-existing task)", func(t *testing.T) {
		comment := &model.TaskComment{
			TaskID:    99999, // Несуществующая задача
			UserID:    userID,
			Content:   "Orphan comment",
			CreatedAt: now,
		}

		err := repo.CreateComment(ctx, nil, comment)
		require.ErrorIs(t, err, repository.ErrForeignKeyViolation)
	})

	t.Run("success create within transaction", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		comment := &model.TaskComment{
			TaskID:    taskID,
			UserID:    userID,
			Content:   "Transactional comment",
			CreatedAt: now,
		}

		err = repo.CreateComment(ctx, tx, comment)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())

		// Убеждаемся, что закоммиченная запись на месте
		fetched, err := repo.GetCommentByID(ctx, nil, comment.ID)
		require.NoError(t, err)
		assert.Equal(t, "Transactional comment", fetched.Content)
	})
}

func TestTaskCommentRepository_GetCommentByID(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTaskCommentRepository(db)

	userID := createTestUser(ctx, t, db, "user@test.com", "User")
	teamID := createTestTeam(ctx, t, db, userID, "Team A")
	taskID := createTestTask(ctx, t, db, teamID, userID, "Task 1")
	now := time.Now().UTC().Truncate(time.Second)

	comment := &model.TaskComment{
		TaskID:    taskID,
		UserID:    userID,
		Content:   "Comment to fetch",
		CreatedAt: now,
	}
	err := repo.CreateComment(ctx, nil, comment)
	require.NoError(t, err)

	t.Run("success get comment", func(t *testing.T) {
		res, err := repo.GetCommentByID(ctx, nil, comment.ID)
		require.NoError(t, err)
		assert.Equal(t, comment.ID, res.ID)
		assert.Equal(t, taskID, res.TaskID)
		assert.Equal(t, userID, res.UserID)
		assert.Equal(t, "Comment to fetch", res.Content)
		assert.WithinDuration(t, now, res.CreatedAt.UTC(), time.Second)
	})

	t.Run("fail comment not found", func(t *testing.T) {
		res, err := repo.GetCommentByID(ctx, nil, 99999)
		require.ErrorIs(t, err, repository.ErrCommentNotFound)
		assert.Nil(t, res)
	})
}

func TestTaskCommentRepository_ListCommentsByTaskID(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTaskCommentRepository(db)

	userID := createTestUser(ctx, t, db, "user@test.com", "User")
	teamID := createTestTeam(ctx, t, db, userID, "Team A")
	task1ID := createTestTask(ctx, t, db, teamID, userID, "Task 1")
	task2ID := createTestTask(ctx, t, db, teamID, userID, "Task 2")

	now := time.Now().UTC().Truncate(time.Second)

	// Создаем несколько комментариев с разным временем для скрупулезной проверки ORDER BY created_at ASC
	c1 := &model.TaskComment{TaskID: task1ID, UserID: userID, Content: "First", CreatedAt: now.Add(-2 * time.Hour)}
	c2 := &model.TaskComment{TaskID: task1ID, UserID: userID, Content: "Second", CreatedAt: now.Add(-1 * time.Hour)}
	c3 := &model.TaskComment{TaskID: task2ID, UserID: userID, Content: "Other Task Comment", CreatedAt: now}

	require.NoError(t, repo.CreateComment(ctx, nil, c1))
	require.NoError(t, repo.CreateComment(ctx, nil, c2))
	require.NoError(t, repo.CreateComment(ctx, nil, c3))

	t.Run("list comments for task 1 ordered ascending", func(t *testing.T) {
		comments, err := repo.ListCommentsByTaskID(ctx, nil, task1ID)
		require.NoError(t, err)
		require.Len(t, comments, 2)

		assert.Equal(t, c1.ID, comments[0].ID)
		assert.Equal(t, "First", comments[0].Content)

		assert.Equal(t, c2.ID, comments[1].ID)
		assert.Equal(t, "Second", comments[1].Content)
	})

	t.Run("returns empty slice (not nil) when task has no comments", func(t *testing.T) {
		emptyTaskID := createTestTask(ctx, t, db, teamID, userID, "Empty Task")
		comments, err := repo.ListCommentsByTaskID(ctx, nil, emptyTaskID)
		require.NoError(t, err)
		assert.NotNil(t, comments, "slice should be non-nil for safe JSON serialization")
		assert.Empty(t, comments)
	})
}

func TestTaskCommentRepository_DeleteComment(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTaskCommentRepository(db)

	userID := createTestUser(ctx, t, db, "user@test.com", "User")
	teamID := createTestTeam(ctx, t, db, userID, "Team A")
	taskID := createTestTask(ctx, t, db, teamID, userID, "Task 1")

	comment := &model.TaskComment{
		TaskID:    taskID,
		UserID:    userID,
		Content:   "To be deleted",
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.CreateComment(ctx, nil, comment))

	t.Run("success delete comment", func(t *testing.T) {
		err := repo.DeleteComment(ctx, nil, comment.ID)
		require.NoError(t, err)

		// Убеждаемся, что больше не существует
		_, err = repo.GetCommentByID(ctx, nil, comment.ID)
		require.ErrorIs(t, err, repository.ErrCommentNotFound)
	})

	t.Run("fail delete non-existing comment", func(t *testing.T) {
		err := repo.DeleteComment(ctx, nil, 99999)
		require.ErrorIs(t, err, repository.ErrCommentNotFound)
	})
}
