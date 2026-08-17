package redis_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	mysqlrepo "github.com/Tantrum4132/task-service/internal/repository/mysql"
	redisrepo "github.com/Tantrum4132/task-service/internal/repository/redis"
)

// setupEnvironment инициализирует Docker-контейнеры MySQL и Redis через testcontainers.
func setupEnvironment(tb testing.TB) (*sql.DB, redisclient.Cmdable, func()) {
	tb.Helper()
	ctx := context.Background()

	// 1. Запуск MySQL контейнера
	mysqlC, err := mysqlcontainer.Run(ctx,
		"mysql:8.0",
		mysqlcontainer.WithDatabase("task_db"),
		mysqlcontainer.WithUsername("test_user"),
		mysqlcontainer.WithPassword("test_pass"),
	)
	require.NoError(tb, err, "failed to start mysql container")

	mysqlHost, err := mysqlC.Host(ctx)
	require.NoError(tb, err)

	mysqlPort, err := mysqlC.MappedPort(ctx, "3306")
	require.NoError(tb, err)

	dsn := fmt.Sprintf("test_user:test_pass@tcp(%s:%s)/task_db?parseTime=true", mysqlHost, mysqlPort.Port())
	db, err := sql.Open("mysql", dsn)
	require.NoError(tb, err)

	require.Eventually(tb, func() bool {
		return db.PingContext(ctx) == nil
	}, 10*time.Second, 500*time.Millisecond, "mysql ping failed")

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(255) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(100) NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.ExecContext(ctx, createTableSQL)
	require.NoError(tb, err, "failed to initialize DB schema")

	// 3. Запуск Redis контейнера
	redisC, err := rediscontainer.Run(ctx, "redis:7-alpine")
	require.NoError(tb, err, "failed to start redis container")

	redisConnStr, err := redisC.ConnectionString(ctx)
	require.NoError(tb, err)

	opts, err := redisclient.ParseURL(redisConnStr)
	require.NoError(tb, err)

	rdb := redisclient.NewClient(opts)
	require.NoError(tb, rdb.Ping(ctx).Err(), "redis ping failed")

	cleanup := func() {
		_ = rdb.Close()
		_ = db.Close()
		_ = mysqlC.Terminate(ctx)
		_ = redisC.Terminate(ctx)
	}

	return db, rdb, cleanup
}

func TestUserCacheDecorator_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	db, rdb, cleanup := setupEnvironment(t)
	defer cleanup()

	mysqlRepo := mysqlrepo.NewUserRepository(db)
	decorator := redisrepo.NewUserCacheDecorator(mysqlRepo, rdb, 5*time.Minute)

	t.Run("Create user and verify Cache Miss -> Fill Cache on FindByID", func(t *testing.T) {
		user := &model.User{
			Email:        "john.doe@example.com",
			PasswordHash: "hashed_password_123",
			Name:         "John Doe",
			CreatedAt:    time.Now().Truncate(time.Second).UTC(),
		}

		err := decorator.Create(ctx, nil, user)
		require.NoError(t, err)
		require.Greater(t, user.ID, int64(0))

		cacheKey := fmt.Sprintf("user:id:%d", user.ID)

		// Ключа нет в кеше до вызова FindByID
		_, err = rdb.Get(ctx, cacheKey).Result()
		assert.ErrorIs(t, err, redisclient.Nil)

		// Cache Miss -> Запрос в MySQL -> Прогрев Redis
		foundUser, err := decorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, foundUser.ID)
		assert.Equal(t, user.Email, foundUser.Email)

		// Ключ появился в Redis
		val, err := rdb.Get(ctx, cacheKey).Result()
		require.NoError(t, err)
		assert.Contains(t, val, "john.doe@example.com")

		// Прямой UPDATE в MySQL в обход кеша для проверки Cache Hit
		_, err = db.ExecContext(ctx, "UPDATE users SET name = 'Bypassed Name' WHERE id = ?", user.ID)
		require.NoError(t, err)

		// Должен отдать старое значение из Redis
		cachedUser, err := decorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "John Doe", cachedUser.Name)
	})

	t.Run("Update user invalidates cache", func(t *testing.T) {
		user := &model.User{
			Email:        "update.test@example.com",
			PasswordHash: "pass",
			Name:         "Initial Name",
			CreatedAt:    time.Now().Truncate(time.Second).UTC(),
		}
		err := decorator.Create(ctx, nil, user)
		require.NoError(t, err)

		// Прогреваем кеш
		_, err = decorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)

		cacheKey := fmt.Sprintf("user:id:%d", user.ID)

		// Инвалидация при Update
		user.Name = "Updated Name"
		err = decorator.Update(ctx, nil, user)
		require.NoError(t, err)

		_, err = rdb.Get(ctx, cacheKey).Result()
		assert.ErrorIs(t, err, redisclient.Nil)

		freshUser, err := decorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", freshUser.Name)
	})

	t.Run("Delete user invalidates cache and returns ErrUserNotFound", func(t *testing.T) {
		user := &model.User{
			Email:        "delete.test@example.com",
			PasswordHash: "pass",
			Name:         "To Delete",
			CreatedAt:    time.Now().Truncate(time.Second).UTC(),
		}
		err := decorator.Create(ctx, nil, user)
		require.NoError(t, err)

		_, err = decorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)

		cacheKey := fmt.Sprintf("user:id:%d", user.ID)

		err = decorator.Delete(ctx, nil, user.ID)
		require.NoError(t, err)

		_, err = rdb.Get(ctx, cacheKey).Result()
		assert.ErrorIs(t, err, redisclient.Nil)

		_, err = decorator.FindByID(ctx, nil, user.ID)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)
	})

	t.Run("FindByEmail delegates directly to mysql repository", func(t *testing.T) {
		user := &model.User{
			Email:        "email.test@example.com",
			PasswordHash: "pass",
			Name:         "Email User",
			CreatedAt:    time.Now().Truncate(time.Second).UTC(),
		}
		err := decorator.Create(ctx, nil, user)
		require.NoError(t, err)

		found, err := decorator.FindByEmail(ctx, nil, user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("FindByID returns ErrUserNotFound for invalid IDs", func(t *testing.T) {
		_, errZero := decorator.FindByID(ctx, nil, 0)
		assert.ErrorIs(t, errZero, repository.ErrUserNotFound)

		_, errNegative := decorator.FindByID(ctx, nil, -10)
		assert.ErrorIs(t, errNegative, repository.ErrUserNotFound)
	})

	t.Run("FindByID fallbacks to DB on corrupted JSON in cache", func(t *testing.T) {
		user := &model.User{
			Email:        "corrupted.cache@example.com",
			PasswordHash: "pass",
			Name:         "Corrupted User",
			CreatedAt:    time.Now().Truncate(time.Second).UTC(),
		}
		err := decorator.Create(ctx, nil, user)
		require.NoError(t, err)

		cacheKey := fmt.Sprintf("user:id:%d", user.ID)
		// Записываем поврежденный JSON прямо в Redis
		err = rdb.Set(ctx, cacheKey, "{invalid-json-data}", time.Minute).Err()
		require.NoError(t, err)

		// Декоратор должен обработать ошибку unmarshal и прозрачно взять пользователя из MySQL
		found, err := decorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Name, found.Name)
	})

	t.Run("NewUserCacheDecorator sets default TTL when ttl <= 0", func(t *testing.T) {
		defaultDecorator := redisrepo.NewUserCacheDecorator(mysqlRepo, rdb, 0)
		require.NotNil(t, defaultDecorator)

		user := &model.User{
			Email:        "default.ttl@example.com",
			PasswordHash: "pass",
			Name:         "Default TTL User",
			CreatedAt:    time.Now().Truncate(time.Second).UTC(),
		}
		err := defaultDecorator.Create(ctx, nil, user)
		require.NoError(t, err)

		_, err = defaultDecorator.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)

		cacheKey := fmt.Sprintf("user:id:%d", user.ID)
		ttl, err := rdb.TTL(ctx, cacheKey).Result()
		require.NoError(t, err)

		// Дефолтный TTL равен 15 минутам (DefaultUserCacheTTL)
		assert.Greater(t, ttl, 14*time.Minute)
		assert.LessOrEqual(t, ttl, 15*time.Minute)
	})
}
