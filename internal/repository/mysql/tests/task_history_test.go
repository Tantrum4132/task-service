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

func TestTaskHistoryRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTaskHistoryRepository(db)

	// Подготовка базовых данных
	userID := createTestUser(ctx, t, db, "history_user@example.com", "History User")
	teamID := createTestTeam(ctx, t, db, userID, "History Team")
	taskID := createTestTask(ctx, t, db, teamID, userID, "Test Task for History")

	t.Run("CreateTaskHistory - Success with JSON Changes", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		history := &model.TaskHistory{
			TaskID:    taskID,
			ChangedBy: userID,
			Changes: model.TaskHistoryChange{
				"status": model.TaskHistoryChangeItem{Old: "todo", New: "in_progress"},
				"title":  model.TaskHistoryChangeItem{Old: "Test Task for History", New: "Updated Task Title"},
			},
			CreatedAt: now,
		}

		err := repo.CreateTaskHistory(ctx, nil, history)
		require.NoError(t, err)
		assert.Greater(t, history.ID, int64(0))

		// Проверка чтения и валидации замаршаленного/распаршенного JSON
		items, err := repo.GetHistoryByTaskID(ctx, nil, repository.TaskHistoryFilter{TaskID: taskID})
		require.NoError(t, err)
		require.Len(t, items, 1)

		fetched := items[0]
		assert.Equal(t, history.ID, fetched.ID)
		assert.Equal(t, taskID, fetched.TaskID)
		assert.Equal(t, userID, fetched.ChangedBy)
		assert.Equal(t, now, fetched.CreatedAt.UTC())

		// Проверка корректности Scan для типа TaskHistoryChange (JSON)
		assert.Contains(t, fetched.Changes, "status")
		assert.Equal(t, "todo", fetched.Changes["status"].Old)
		assert.Equal(t, "in_progress", fetched.Changes["status"].New)
	})

	t.Run("CreateTaskHistory - Nil History Error", func(t *testing.T) {
		err := repo.CreateTaskHistory(ctx, nil, nil)
		assert.ErrorContains(t, err, "task history cannot be nil")
	})

	t.Run("CreateTaskHistory - Uninitialized CreatedAt Error", func(t *testing.T) {
		history := &model.TaskHistory{
			TaskID:    taskID,
			ChangedBy: userID,
			Changes:   model.TaskHistoryChange{},
			// CreatedAt преднамеренно не инициализирован (IsZero)
		}

		err := repo.CreateTaskHistory(ctx, nil, history)
		assert.ErrorIs(t, err, repository.ErrInvalidTaskHistoryPayload)
	})

	t.Run("CreateTaskHistory - Foreign Key Violation", func(t *testing.T) {
		nonExistentTaskID := int64(999999)
		history := &model.TaskHistory{
			TaskID:    nonExistentTaskID,
			ChangedBy: userID,
			Changes:   model.TaskHistoryChange{"status": model.TaskHistoryChangeItem{Old: "todo", New: "done"}},
			CreatedAt: time.Now().UTC(),
		}

		err := repo.CreateTaskHistory(ctx, nil, history)
		assert.ErrorIs(t, err, repository.ErrForeignKeyViolation)
	})

	t.Run("GetHistoryByTaskID - Missing TaskID Error", func(t *testing.T) {
		_, err := repo.GetHistoryByTaskID(ctx, nil, repository.TaskHistoryFilter{TaskID: 0})
		assert.ErrorIs(t, err, repository.ErrTaskIDRequired)
	})

	t.Run("GetHistoryByTaskID - Pagination and Deterministic Sorting", func(t *testing.T) {
		newTaskID := createTestTask(ctx, t, db, teamID, userID, "Task with Multiple History Logs")
		baseTime := time.Now().UTC().Truncate(time.Second)

		// Создаем 5 записей истории с одинаковым временем для проверки вторичной сортировки по id DESC
		for i := 1; i <= 5; i++ {
			h := &model.TaskHistory{
				TaskID:    newTaskID,
				ChangedBy: userID,
				Changes:   model.TaskHistoryChange{"step": model.TaskHistoryChangeItem{Old: i - 1, New: i}},
				CreatedAt: baseTime,
			}
			require.NoError(t, repo.CreateTaskHistory(ctx, nil, h))
		}

		// Тест 1: Страница 1 (Limit 2, Offset 0)
		page1, err := repo.GetHistoryByTaskID(ctx, nil, repository.TaskHistoryFilter{
			TaskID: newTaskID,
			Limit:  2,
			Offset: 0,
		})
		require.NoError(t, err)
		require.Len(t, page1, 2)
		// Ожидается детерминированная сортировка по created_at DESC, id DESC
		assert.Greater(t, page1[0].ID, page1[1].ID)

		// Тест 2: Страница 2 (Limit 2, Offset 2)
		page2, err := repo.GetHistoryByTaskID(ctx, nil, repository.TaskHistoryFilter{
			TaskID: newTaskID,
			Limit:  2,
			Offset: 2,
		})
		require.NoError(t, err)
		require.Len(t, page2, 2)
		assert.Greater(t, page2[0].ID, page2[1].ID)

		// Элементы разных страниц не должны пересекаться
		assert.Greater(t, page1[1].ID, page2[0].ID)
	})

	t.Run("CreateTaskHistory and GetHistoryByTaskID - Within Transaction", func(t *testing.T) {
		txTaskID := createTestTask(ctx, t, db, teamID, userID, "Transactional Task")

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		history := &model.TaskHistory{
			TaskID:    txTaskID,
			ChangedBy: userID,
			Changes:   model.TaskHistoryChange{"priority": model.TaskHistoryChangeItem{Old: "low", New: "high"}},
			CreatedAt: time.Now().UTC(),
		}

		// Запись истории внутри транзакции
		err = repo.CreateTaskHistory(ctx, tx, history)
		require.NoError(t, err)

		// Чтение внутри транзакции (должно видеть несовершенную запись)
		txItems, err := repo.GetHistoryByTaskID(ctx, tx, repository.TaskHistoryFilter{TaskID: txTaskID})
		require.NoError(t, err)
		require.Len(t, txItems, 1)

		// Чтение вне транзакции (до Commit не должно видеть запись)
		nonTxItems, err := repo.GetHistoryByTaskID(ctx, nil, repository.TaskHistoryFilter{TaskID: txTaskID})
		require.NoError(t, err)
		assert.Empty(t, nonTxItems)

		// Фиксация транзакции
		require.NoError(t, tx.Commit())

		// Чтение вне транзакции после Commit
		postCommitItems, err := repo.GetHistoryByTaskID(ctx, nil, repository.TaskHistoryFilter{TaskID: txTaskID})
		require.NoError(t, err)
		require.Len(t, postCommitItems, 1)
	})
}
