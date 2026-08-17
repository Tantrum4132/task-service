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

func TestTeamRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	teamRepo := repos.NewTeamRepository(db)
	memberRepo := repos.NewTeamMemberRepository(db)

	ownerID := createTestUser(ctx, t, db, "owner@example.com", "Owner User")
	memberID := createTestUser(ctx, t, db, "member@example.com", "Member User")

	t.Run("CreateTeam - Nil Team Error", func(t *testing.T) {
		err := teamRepo.CreateTeam(ctx, nil, nil)
		assert.ErrorContains(t, err, "team cannot be nil")
	})

	t.Run("Create team with owner in transaction - Success", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		now := time.Now().UTC().Truncate(time.Second)
		team := &model.Team{
			Name:      "Alpha Team",
			CreatedBy: ownerID,
			CreatedAt: now, // Добавлена инициализация CreatedAt
		}

		// 1. Создаем команду в транзакции
		err = teamRepo.CreateTeam(ctx, tx, team)
		require.NoError(t, err)
		assert.Greater(t, team.ID, int64(0))

		// 2. Добавляем владельца через memberRepo в той же транзакции
		member := &model.TeamMember{
			TeamID: team.ID,
			UserID: ownerID,
			Role:   model.TeamRoleOwner,
		}
		err = memberRepo.AddMember(ctx, tx, member)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())

		// 3. Проверяем получение созданной команды по ID
		fetchedTeam, err := teamRepo.GetTeamByID(ctx, nil, team.ID)
		require.NoError(t, err)
		assert.Equal(t, team.Name, fetchedTeam.Name)
		assert.Equal(t, team.CreatedBy, fetchedTeam.CreatedBy)

		// 4. Проверяем роль владельца через memberRepo
		role, err := memberRepo.GetMemberRole(ctx, nil, team.ID, ownerID)
		require.NoError(t, err)
		assert.Equal(t, model.TeamRoleOwner, role)
	})

	t.Run("GetUserTeams - Returns teams where user is member", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		team := &model.Team{
			Name:      "Gamma Team",
			CreatedBy: ownerID,
			CreatedAt: now, // Добавлена инициализация CreatedAt
		}
		require.NoError(t, teamRepo.CreateTeam(ctx, nil, team))
		require.NoError(t, memberRepo.AddMember(ctx, nil, &model.TeamMember{
			TeamID: team.ID,
			UserID: memberID,
			Role:   model.TeamRoleMember,
		}))

		teams, err := teamRepo.GetUserTeams(ctx, nil, memberID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(teams), 1)
		assert.Equal(t, team.Name, teams[0].Name)
	})

	t.Run("GetUserTeams - Empty list when user has no teams", func(t *testing.T) {
		noTeamsUserID := createTestUser(ctx, t, db, "noteams@example.com", "No Teams User")
		teams, err := teamRepo.GetUserTeams(ctx, nil, noTeamsUserID)
		require.NoError(t, err)
		assert.Empty(t, teams)
	})

	t.Run("GetNonExistentTeam - Returns ErrTeamNotFound", func(t *testing.T) {
		_, err := teamRepo.GetTeamByID(ctx, nil, 999999)
		assert.ErrorIs(t, err, repository.ErrTeamNotFound)
	})
}
