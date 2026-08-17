package mysql_test

import (
	"context"
	"testing"
	"time"

	"github.com/Tantrum4132/task-service/internal/repository/mysql"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsRepository_GetTeamStats(t *testing.T) {
	ctx := context.Background()

	// Инициализация MySQL контейнера и тестовой схемы из setup_test.go
	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := mysql.NewStatsRepository(db)

	t.Run("successfully retrieves full team statistics", func(t *testing.T) {
		// 1. Создаем тестовых пользователей
		ownerID := createTestUser(ctx, t, db, "owner@example.com", "Owner User")
		user1ID := createTestUser(ctx, t, db, "dev1@example.com", "Alice Developer")
		user2ID := createTestUser(ctx, t, db, "dev2@example.com", "Bob Developer")

		// 2. Создаем команду
		teamID := createTestTeam(ctx, t, db, ownerID, "Backend Team")

		// 3. Подготавливаем задачи с разным статусом
		// Задача 1: todo
		_, err := db.ExecContext(ctx,
			"INSERT INTO tasks (team_id, title, status, created_by) VALUES (?, 'Task 1', 'todo', ?)",
			teamID, ownerID,
		)
		require.NoError(t, err)

		// Задача 2: in_progress
		_, err = db.ExecContext(ctx,
			"INSERT INTO tasks (team_id, title, status, created_by, assignee_id) VALUES (?, 'Task 2', 'in_progress', ?, ?)",
			teamID, ownerID, user1ID,
		)
		require.NoError(t, err)

		// Задача 3: done (закрыта 2 часа назад человеком Alice)
		now := time.Now().UTC()
		closedAtTask3 := now.Add(-2 * time.Hour)
		createdAtTask3 := closedAtTask3.Add(-4 * time.Hour) // время выполнения 4 часа
		res3, err := db.ExecContext(ctx,
			"INSERT INTO tasks (team_id, title, status, created_by, assignee_id, created_at, closed_at) VALUES (?, 'Task 3', 'done', ?, ?, ?, ?)",
			teamID, ownerID, user1ID, createdAtTask3, closedAtTask3,
		)
		require.NoError(t, err)
		task3ID, err := res3.LastInsertId()
		require.NoError(t, err)

		// Задача 4: done (закрыта 1 день назад человеком Bob)
		closedAtTask4 := now.Add(-24 * time.Hour)
		createdAtTask4 := closedAtTask4.Add(-2 * time.Hour) // время выполнения 2 часа
		res4, err := db.ExecContext(ctx,
			"INSERT INTO tasks (team_id, title, status, created_by, assignee_id, created_at, closed_at) VALUES (?, 'Task 4', 'done', ?, ?, ?, ?)",
			teamID, ownerID, user2ID, createdAtTask4, closedAtTask4,
		)
		require.NoError(t, err)
		task4ID, err := res4.LastInsertId()
		require.NoError(t, err)

		// Задача 5: done (закрыта 40 дней назад человеком Alice — НЕ должна попасть в топ за 30 дней)
		closedAtTask5 := now.Add(-40 * 24 * time.Hour)
		createdAtTask5 := closedAtTask5.Add(-1 * time.Hour)
		_, err = db.ExecContext(ctx,
			"INSERT INTO tasks (team_id, title, status, created_by, assignee_id, created_at, closed_at) VALUES (?, 'Task 5', 'done', ?, ?, ?, ?)",
			teamID, ownerID, user1ID, createdAtTask5, closedAtTask5,
		)
		require.NoError(t, err)

		// 4. Добавляем комментарии к задачам этой команды
		_, err = db.ExecContext(ctx,
			"INSERT INTO task_comments (task_id, user_id, content) VALUES (?, ?, 'Comment 1'), (?, ?, 'Comment 2')",
			task3ID, user1ID, task4ID, user2ID,
		)
		require.NoError(t, err)

		// 5. Вызываем тестируемый метод
		stats, err := repo.GetTeamStats(ctx, teamID)
		require.NoError(t, err)
		require.NotNil(t, stats)

		// 6. Проверяем корректность расчетов метрик
		assert.Equal(t, teamID, stats.TeamID)
		assert.Equal(t, int64(1), stats.Statuses.Todo)
		assert.Equal(t, int64(1), stats.Statuses.InProgress)
		assert.Equal(t, int64(3), stats.Statuses.Done)

		// Среднее время закрытия: (4 часа + 2 часа + 1 час) / 3 задачи = 2.333... часа
		assert.InEpsilon(t, 2.3333, stats.AvgCloseTimeHours, 0.01)

		// Всего комментариев
		assert.Equal(t, int64(2), stats.TotalCommentsCount)

		// Топ исполнители за 30 дней
		require.Len(t, stats.TopAssignees, 2)
		assigneeEmails := []string{stats.TopAssignees[0].Email, stats.TopAssignees[1].Email}
		assert.Contains(t, assigneeEmails, "dev1@example.com")
		assert.Contains(t, assigneeEmails, "dev2@example.com")
	})

	t.Run("returns zero metrics for empty team without error", func(t *testing.T) {
		ownerID := createTestUser(ctx, t, db, "empty_owner@example.com", "Empty Owner")
		emptyTeamID := createTestTeam(ctx, t, db, ownerID, "Empty Team")

		stats, err := repo.GetTeamStats(ctx, emptyTeamID)
		require.NoError(t, err)
		require.NotNil(t, stats)

		assert.Equal(t, emptyTeamID, stats.TeamID)
		assert.Equal(t, int64(0), stats.Statuses.Todo)
		assert.Equal(t, int64(0), stats.Statuses.InProgress)
		assert.Equal(t, int64(0), stats.Statuses.Done)
		assert.Equal(t, float64(0), stats.AvgCloseTimeHours)
		assert.Equal(t, int64(0), stats.TotalCommentsCount)
		assert.Empty(t, stats.TopAssignees)
	})

	t.Run("ensures data isolation between teams", func(t *testing.T) {
		ownerID := createTestUser(ctx, t, db, "iso_owner@example.com", "Iso Owner")

		team1ID := createTestTeam(ctx, t, db, ownerID, "Team 1")
		team2ID := createTestTeam(ctx, t, db, ownerID, "Team 2")

		// Создаем 3 задачи в Team 1
		for i := 0; i < 3; i++ {
			createTestTask(ctx, t, db, team1ID, ownerID, "Team 1 Task")
		}

		// Создаем 1 задачу в Team 2
		createTestTask(ctx, t, db, team2ID, ownerID, "Team 2 Task")

		// Запрашиваем статистику по Team 2
		statsTeam2, err := repo.GetTeamStats(ctx, team2ID)
		require.NoError(t, err)

		// Проверяем, что в Team 2 посчиталась только 1 ее задача
		assert.Equal(t, int64(1), statsTeam2.Statuses.Todo)
	})
}
