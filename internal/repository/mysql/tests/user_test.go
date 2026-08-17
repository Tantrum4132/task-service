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

func TestUserRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewUserRepository(db)

	t.Run("Create and FindByID/FindByEmail - Success", func(t *testing.T) {
		user := &model.User{
			Email:        "alice@example.com",
			PasswordHash: "hashed_password_123",
			Name:         "Alice",
			CreatedAt:    time.Now().UTC().Truncate(time.Second),
		}

		err := repo.Create(ctx, nil, user)
		require.NoError(t, err)
		assert.Greater(t, user.ID, int64(0))

		// FindByEmail
		foundByMail, err := repo.FindByEmail(ctx, nil, "alice@example.com")
		require.NoError(t, err)
		assert.Equal(t, "Alice", foundByMail.Name)

		// FindByID
		foundByID, err := repo.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, foundByID.ID)
	})

	t.Run("FindByID and FindByEmail - Not Found", func(t *testing.T) {
		_, err := repo.FindByID(ctx, nil, 999999)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)

		_, err = repo.FindByEmail(ctx, nil, "nonexistent@example.com")
		assert.ErrorIs(t, err, repository.ErrUserNotFound)
	})

	t.Run("Create - Duplicate Email Error", func(t *testing.T) {
		user1 := &model.User{Email: "dup@example.com", PasswordHash: "hash", Name: "User 1", CreatedAt: time.Now().UTC()}
		user2 := &model.User{Email: "dup@example.com", PasswordHash: "hash", Name: "User 2", CreatedAt: time.Now().UTC()}

		require.NoError(t, repo.Create(ctx, nil, user1))
		err := repo.Create(ctx, nil, user2)
		assert.ErrorIs(t, err, repository.ErrEmailAlreadyExists)
	})

	t.Run("Delete - Success and Not Found", func(t *testing.T) {
		user := &model.User{Email: "delete_me@example.com", PasswordHash: "hash", Name: "To Delete", CreatedAt: time.Now().UTC()}
		require.NoError(t, repo.Create(ctx, nil, user))

		require.NoError(t, repo.Delete(ctx, nil, user.ID))

		_, err := repo.FindByID(ctx, nil, user.ID)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)

		err = repo.Delete(ctx, nil, user.ID)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)
	})

	t.Run("Transactional behavior", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		user := &model.User{
			Email:        "tx_user@example.com",
			PasswordHash: "hash",
			Name:         "TX User",
			CreatedAt:    time.Now().UTC(),
		}

		// Создаем пользователя внутри транзакции
		require.NoError(t, repo.Create(ctx, tx, user))

		// Внутри транзакции пользователь виден
		foundInTx, err := repo.FindByID(ctx, tx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "TX User", foundInTx.Name)

		// Вне транзакции пользователь еще недоступен
		_, err = repo.FindByID(ctx, nil, user.ID)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)

		require.NoError(t, tx.Commit())

		// После коммита пользователь доступен глобально
		foundAfterCommit, err := repo.FindByID(ctx, nil, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "TX User", foundAfterCommit.Name)
	})
}
