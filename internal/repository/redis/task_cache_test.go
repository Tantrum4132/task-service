package redis_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	cache "github.com/Tantrum4132/task-service/internal/repository/redis"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	redisContainer "github.com/testcontainers/testcontainers-go/modules/redis"
)

// mockDBEngine имплементирует repository.DBEngine для имитации выполнения операций внутри транзакции
type mockDBEngine struct{}

func (m *mockDBEngine) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}

func (m *mockDBEngine) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (m *mockDBEngine) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

// mockTaskRepository имитирует базовый MySQL репозиторий для изоляции тестов кеша
type mockTaskRepository struct {
	tasks      map[int64]*model.Task
	listCount  int
	nextTaskID int64
}

func newMockTaskRepository() *mockTaskRepository {
	return &mockTaskRepository{
		tasks:      make(map[int64]*model.Task),
		nextTaskID: 1,
	}
}

func (m *mockTaskRepository) CreateTask(ctx context.Context, exec repository.DBEngine, task *model.Task) error {
	task.ID = m.nextTaskID
	m.nextTaskID++
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskRepository) GetTaskByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.Task, error) {
	task, ok := m.tasks[id]
	if !ok {
		return nil, repository.ErrTaskNotFound
	}
	return task, nil
}

func (m *mockTaskRepository) UpdateTask(ctx context.Context, exec repository.DBEngine, task *model.Task) error {
	if _, ok := m.tasks[task.ID]; !ok {
		return repository.ErrTaskNotFound
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskRepository) DeleteTask(ctx context.Context, exec repository.DBEngine, id int64) error {
	if _, ok := m.tasks[id]; !ok {
		return repository.ErrTaskNotFound
	}
	delete(m.tasks, id)
	return nil
}

func (m *mockTaskRepository) ListTasks(ctx context.Context, exec repository.DBEngine, filter repository.TaskFilter) ([]model.Task, error) {
	m.listCount++
	res := make([]model.Task, 0)
	for _, t := range m.tasks {
		if t.TeamID == filter.TeamID {
			if filter.Status != nil && t.Status != *filter.Status {
				continue
			}
			res = append(res, *t)
		}
	}
	return res, nil
}

func setupRedisContainer(ctx context.Context, t *testing.T) (redis.Cmdable, func()) {
	t.Helper()

	container, err := redisContainer.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "failed to start redis container")

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err, "failed to get redis endpoint")

	rdb := redis.NewClient(&redis.Options{
		Addr: endpoint,
	})

	require.NoError(t, rdb.Ping(ctx).Err(), "failed to ping redis")

	cleanup := func() {
		_ = rdb.Close()
		_ = testcontainers.TerminateContainer(container)
	}

	return rdb, cleanup
}

func TestTaskCacheDecorator_Integration(t *testing.T) {
	ctx := context.Background()
	rdb, cleanup := setupRedisContainer(ctx, t)
	defer cleanup()

	t.Run("Cache hit and miss on ListTasks", func(t *testing.T) {
		mockRepo := newMockTaskRepository()
		decorator := cache.NewTaskCacheDecorator(mockRepo, rdb, 5*time.Minute)

		teamID := int64(10)
		filter := repository.TaskFilter{TeamID: teamID, Limit: 10}

		// 1. Первый запрос — Cache Miss, идет обращение к базовому репозиторию
		tasks, err := decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Empty(t, tasks)
		assert.Equal(t, 1, mockRepo.listCount)

		// 2. Второй запрос с теми же фильтрами — Cache Hit, данные берутся из Redis
		tasks, err = decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Empty(t, tasks)
		assert.Equal(t, 1, mockRepo.listCount, "Count should remain 1 due to cache hit")
	})

	t.Run("Invalidate cache on CreateTask", func(t *testing.T) {
		mockRepo := newMockTaskRepository()
		decorator := cache.NewTaskCacheDecorator(mockRepo, rdb, 5*time.Minute)

		teamID := int64(20)
		filter := repository.TaskFilter{TeamID: teamID}

		// Нагреваем кеш
		_, err := decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Equal(t, 1, mockRepo.listCount)

		// Создаем задачу -> кеш команды должен инвалидироваться
		newTask := &model.Task{
			TeamID: teamID,
			Title:  "New Task",
			Status: model.TaskStatusTodo,
		}
		err = decorator.CreateTask(ctx, nil, newTask)
		require.NoError(t, err)

		// Следующий ListTasks должен снова обратиться к MySQL (mockRepo)
		tasks, err := decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, 2, mockRepo.listCount, "ListCount should increment after cache invalidation")
	})

	t.Run("Invalidate cache on UpdateTask and DeleteTask", func(t *testing.T) {
		mockRepo := newMockTaskRepository()
		decorator := cache.NewTaskCacheDecorator(mockRepo, rdb, 5*time.Minute)

		teamID := int64(30)
		filter := repository.TaskFilter{TeamID: teamID}

		task := &model.Task{TeamID: teamID, Title: "Initial", Status: model.TaskStatusTodo}
		require.NoError(t, decorator.CreateTask(ctx, nil, task))

		// Вызов ListTasks для прогрева кеша
		_, err := decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		initialListCount := mockRepo.listCount

		// 1. Обновление задачи -> инвалидация
		task.Title = "Updated Title"
		require.NoError(t, decorator.UpdateTask(ctx, nil, task))

		_, err = decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Equal(t, initialListCount+1, mockRepo.listCount, "Cache must invalidate on UpdateTask")

		// 2. Удаление задачи -> инвалидация
		require.NoError(t, decorator.DeleteTask(ctx, nil, task.ID))

		_, err = decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Equal(t, initialListCount+2, mockRepo.listCount, "Cache must invalidate on DeleteTask")
	})

	t.Run("Invalidate cache even when exec != nil", func(t *testing.T) {
		mockRepo := newMockTaskRepository()
		decorator := cache.NewTaskCacheDecorator(mockRepo, rdb, 5*time.Minute)

		teamID := int64(40)
		filter := repository.TaskFilter{TeamID: teamID}

		// Прогрев кеша
		_, err := decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)

		// Имитация записи внутри активной транзакции (exec != nil)
		fakeTx := &mockDBEngine{}
		task := &model.Task{TeamID: teamID, Title: "Tx Task"}
		require.NoError(t, decorator.CreateTask(ctx, fakeTx, task))

		// Кеш должен инвалидироваться независимо от наличия exec
		_, err = decorator.ListTasks(ctx, nil, filter)
		require.NoError(t, err)
		assert.Equal(t, 2, mockRepo.listCount, "Cache must be invalidated when exec != nil")
	})
}
