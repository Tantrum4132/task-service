package mysql_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	repos "github.com/Tantrum4132/task-service/internal/repository/mysql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
)

// setupTaskTestDB поднимает MySQL контейнер и создаёт схему с таблицами users, teams, tasks
func setupTaskTestDB(ctx context.Context, t *testing.T) (*sql.DB, func()) {
	t.Helper()

	container, err := mysqlcontainer.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysqlcontainer.WithDatabase("test_db"),
		mysqlcontainer.WithUsername("test_user"),
		mysqlcontainer.WithPassword("test_password"),
	)
	require.NoError(t, err, "failed to start mysql container")

	connStr, err := container.ConnectionString(ctx, "parseTime=true&multiStatements=true")
	require.NoError(t, err, "failed to get connection string")

	db, err := sql.Open("mysql", connStr)
	require.NoError(t, err, "failed to open db connection")
	require.NoError(t, db.PingContext(ctx), "failed to ping db")

	schemaSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(255) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(100) NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS teams (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		created_by BIGINT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		team_id BIGINT NOT NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT NOT NULL,
		status ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
		created_by BIGINT NOT NULL,
		assignee_id BIGINT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		closed_at DATETIME NULL,
		version INT NOT NULL DEFAULT 1,
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
		FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL
	);
	`
	_, err = db.ExecContext(ctx, schemaSQL)
	require.NoError(t, err, "failed to apply schema migrations")

	cleanup := func() {
		_ = db.Close()
		_ = container.Terminate(ctx)
	}

	return db, cleanup
}

func createTestUser2(ctx context.Context, t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx, "INSERT INTO users (email, password_hash, name) VALUES (?, 'hash', 'Test User')", email)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func TestTaskRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, cleanup := setupTaskTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTaskRepository(db)

	// Подготовка базовых связанных сущностей
	userID := createTestUser2(ctx, t, db, "creator@example.com")
	assigneeID := createTestUser2(ctx, t, db, "assignee@example.com")
	teamID := createTestTeam(ctx, t, db, userID, "Dev Team")

	t.Run("CreateTask - Success", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		task := &model.Task{
			TeamID:      teamID,
			Title:       "Setup CI/CD",
			Description: "Configure GitHub Actions",
			Status:      model.TaskStatusTodo,
			CreatedBy:   userID,
			AssigneeID:  &assigneeID,
			CreatedAt:   now,
			UpdatedAt:   now,
			Version:     1,
		}

		err := repo.CreateTask(ctx, nil, task)
		require.NoError(t, err)
		assert.Greater(t, task.ID, int64(0))

		// Проверяем получение созданной задачи из БД
		fetched, err := repo.GetTaskByID(ctx, nil, task.ID)
		require.NoError(t, err)
		assert.Equal(t, task.Title, fetched.Title)
		assert.Equal(t, task.Description, fetched.Description)
		assert.Equal(t, model.TaskStatusTodo, fetched.Status)
		assert.Equal(t, int64(assigneeID), *fetched.AssigneeID)
		assert.Equal(t, 1, fetched.Version)
		assert.Nil(t, fetched.ClosedAt)
	})

	t.Run("CreateTask - Uninitialized Payload Error", func(t *testing.T) {
		invalidTask := &model.Task{
			TeamID: teamID,
			Title:  "Bad Task",
		}

		err := repo.CreateTask(ctx, nil, invalidTask)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid task payload")
	})

	t.Run("CreateTask - Foreign Key Error", func(t *testing.T) {
		now := time.Now().UTC()
		task := &model.Task{
			TeamID:    999999, // Несуществующая команда
			Title:     "Orphan Task",
			Status:    model.TaskStatusTodo,
			CreatedBy: userID,
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}

		err := repo.CreateTask(ctx, nil, task)
		assert.ErrorIs(t, err, repository.ErrForeignKeyViolation)
	})

	t.Run("UpdateTask - Success with Optimistic Locking", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		task := &model.Task{
			TeamID:    teamID,
			Title:     "Initial Title",
			Status:    model.TaskStatusInProgress,
			CreatedBy: userID,
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}
		require.NoError(t, repo.CreateTask(ctx, nil, task))

		// Обновляем задачу (перевод в Done)
		closedAt := time.Now().UTC().Truncate(time.Second)
		task.Title = "Updated Title"
		task.Status = model.TaskStatusDone
		task.ClosedAt = &closedAt
		task.UpdatedAt = closedAt

		err := repo.UpdateTask(ctx, nil, task)
		require.NoError(t, err)
		assert.Equal(t, 2, task.Version) // Версия локального объекта должна инкрементироваться

		// Проверяем актуальное состояние в БД
		fetched, err := repo.GetTaskByID(ctx, nil, task.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", fetched.Title)
		assert.Equal(t, model.TaskStatusDone, fetched.Status)
		assert.Equal(t, 2, fetched.Version)
		assert.NotNil(t, fetched.ClosedAt)
	})

	t.Run("UpdateTask - Version Conflict Error", func(t *testing.T) {
		now := time.Now().UTC()
		task := &model.Task{
			TeamID:    teamID,
			Title:     "Conflict Test Task",
			Status:    model.TaskStatusTodo,
			CreatedBy: userID,
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}
		require.NoError(t, repo.CreateTask(ctx, nil, task))

		// Имитируем устаревшую версию структуры
		staleTask := *task
		staleTask.Version = 1 // Текущая версия в БД уже 1, но симулируем гонку отправкой некорректной версии после стороннего апдейта

		// Первый апдейт проходит успешно
		task.Title = "First Edit"
		task.UpdatedAt = time.Now().UTC()
		require.NoError(t, repo.UpdateTask(ctx, nil, task)) // В БД версия стала 2

		// Вторая попытка с устаревшей версией 1 должна вернуть ошибку конфликта
		staleTask.Title = "Concurrent Edit"
		staleTask.UpdatedAt = time.Now().UTC()
		err := repo.UpdateTask(ctx, nil, &staleTask)
		assert.ErrorIs(t, err, repository.ErrTaskConflict)
	})

	t.Run("ListTasks - Filtering and Pagination", func(t *testing.T) {
		newTeamID := createTestTeam(ctx, t, db, userID, "Filter Team")
		now := time.Now().UTC()

		// Создаем 3 задачи
		t1 := &model.Task{TeamID: newTeamID, Title: "Task 1", Status: model.TaskStatusTodo, CreatedBy: userID, AssigneeID: &assigneeID, CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now, Version: 1}
		t2 := &model.Task{TeamID: newTeamID, Title: "Task 2", Status: model.TaskStatusInProgress, CreatedBy: userID, AssigneeID: &assigneeID, CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now, Version: 1}
		t3 := &model.Task{TeamID: newTeamID, Title: "Task 3", Status: model.TaskStatusTodo, CreatedBy: userID, CreatedAt: now.Add(-1 * time.Minute), UpdatedAt: now, Version: 1}

		require.NoError(t, repo.CreateTask(ctx, nil, t1))
		require.NoError(t, repo.CreateTask(ctx, nil, t2))
		require.NoError(t, repo.CreateTask(ctx, nil, t3))

		// 1. Фильтр по статусу 'todo'
		todoStatus := model.TaskStatusTodo
		tasks, err := repo.ListTasks(ctx, nil, model.TaskFilter{
			TeamID: newTeamID,
			Status: &todoStatus,
		})
		require.NoError(t, err)
		assert.Len(t, tasks, 2)

		// 2. Фильтр по AssigneeID
		tasks, err = repo.ListTasks(ctx, nil, model.TaskFilter{
			TeamID:     newTeamID,
			AssigneeID: &assigneeID,
		})
		require.NoError(t, err)
		assert.Len(t, tasks, 2)

		// 3. Пагинация (Limit 1, Offset 0)
		tasks, err = repo.ListTasks(ctx, nil, model.TaskFilter{
			TeamID: newTeamID,
			Limit:  1,
			Offset: 0,
		})
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, "Task 3", tasks[0].Title) // ORDER BY created_at DESC
	})

	t.Run("DeleteTask - Success and Not Found", func(t *testing.T) {
		now := time.Now().UTC()
		task := &model.Task{
			TeamID:    teamID,
			Title:     "To be deleted",
			Status:    model.TaskStatusTodo,
			CreatedBy: userID,
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}
		require.NoError(t, repo.CreateTask(ctx, nil, task))

		// Удаляем существующую задачу
		err := repo.DeleteTask(ctx, nil, task.ID)
		require.NoError(t, err)

		// Повторный поиск должен вернуть ErrTaskNotFound
		_, err = repo.GetTaskByID(ctx, nil, task.ID)
		assert.ErrorIs(t, err, repository.ErrTaskNotFound)

		// Повторное удаление возвращает ErrTaskNotFound
		err = repo.DeleteTask(ctx, nil, task.ID)
		assert.ErrorIs(t, err, repository.ErrTaskNotFound)
	})
}
