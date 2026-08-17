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

func TestTeamMemberRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := repos.NewTeamMemberRepository(db)

	// Подготовка тестовых данных
	ownerID := createTestUser(ctx, t, db, "owner@example.com", "Owner User")
	user1ID := createTestUser(ctx, t, db, "user1@example.com", "User 1")
	user2ID := createTestUser(ctx, t, db, "user2@example.com", "User 2")
	teamID := createTestTeam(ctx, t, db, ownerID, "Core Team")

	t.Run("AddMember - Nil Member Error", func(t *testing.T) {
		err := repo.AddMember(ctx, nil, nil)
		assert.ErrorContains(t, err, "team member cannot be nil")
	})

	t.Run("AddMember - Success", func(t *testing.T) {
		member := &model.TeamMember{
			TeamID: teamID,
			UserID: ownerID,
			Role:   model.TeamRoleOwner,
		}

		err := repo.AddMember(ctx, nil, member)
		require.NoError(t, err)

		isMember, err := repo.IsMember(ctx, nil, teamID, ownerID)
		require.NoError(t, err)
		assert.True(t, isMember)
	})

	t.Run("AddMember - Duplicate Member Error", func(t *testing.T) {
		member := &model.TeamMember{
			TeamID: teamID,
			UserID: ownerID,
			Role:   model.TeamRoleMember,
		}

		err := repo.AddMember(ctx, nil, member)
		assert.ErrorIs(t, err, repository.ErrMemberExists)
	})

	t.Run("AddMember - Nonexistent User/Team (Foreign Key Error)", func(t *testing.T) {
		invalidMember := &model.TeamMember{
			TeamID: teamID,
			UserID: 99999,
			Role:   model.TeamRoleMember,
		}

		err := repo.AddMember(ctx, nil, invalidMember)
		assert.ErrorIs(t, err, repository.ErrForeignKeyViolation)
	})

	t.Run("GetMember and GetMemberRole - Success and Not Found", func(t *testing.T) {
		member, err := repo.GetMember(ctx, nil, teamID, ownerID)
		require.NoError(t, err)
		assert.Equal(t, teamID, member.TeamID)
		assert.Equal(t, ownerID, member.UserID)
		assert.Equal(t, model.TeamRoleOwner, member.Role)

		role, err := repo.GetMemberRole(ctx, nil, teamID, ownerID)
		require.NoError(t, err)
		assert.Equal(t, model.TeamRoleOwner, role)

		// Not found checks
		_, err = repo.GetMember(ctx, nil, teamID, 99999)
		assert.ErrorIs(t, err, repository.ErrMemberNotFound)

		_, err = repo.GetMemberRole(ctx, nil, teamID, 99999)
		assert.ErrorIs(t, err, repository.ErrMemberNotFound)
	})

	t.Run("UpdateMemberRole - Success and Not Found", func(t *testing.T) {
		err := repo.AddMember(ctx, nil, &model.TeamMember{
			TeamID: teamID,
			UserID: user1ID,
			Role:   model.TeamRoleMember,
		})
		require.NoError(t, err)

		err = repo.UpdateMemberRole(ctx, nil, teamID, user1ID, model.TeamRoleAdmin)
		require.NoError(t, err)

		role, err := repo.GetMemberRole(ctx, nil, teamID, user1ID)
		require.NoError(t, err)
		assert.Equal(t, model.TeamRoleAdmin, role)

		err = repo.UpdateMemberRole(ctx, nil, teamID, user2ID, model.TeamRoleAdmin)
		assert.ErrorIs(t, err, repository.ErrMemberNotFound)
	})

	t.Run("ListTeamMembers - Returns all members sorted", func(t *testing.T) {
		err := repo.AddMember(ctx, nil, &model.TeamMember{
			TeamID: teamID,
			UserID: user2ID,
			Role:   model.TeamRoleMember,
		})
		require.NoError(t, err)

		members, err := repo.ListTeamMembers(ctx, nil, teamID)
		require.NoError(t, err)
		assert.Len(t, members, 3)
	})

	t.Run("Transactional behavior - Add and Read inside TX", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()

		newTeamID := createTestTeam(ctx, t, db, ownerID, "Tx Team")

		err = repo.AddMember(ctx, tx, &model.TeamMember{
			TeamID: newTeamID,
			UserID: user1ID,
			Role:   model.TeamRoleMember,
		})
		require.NoError(t, err)

		isMem, err := repo.IsMember(ctx, tx, newTeamID, user1ID)
		require.NoError(t, err)
		assert.True(t, isMem)

		isMemOutside, err := repo.IsMember(ctx, nil, newTeamID, user1ID)
		require.NoError(t, err)
		assert.False(t, isMemOutside)
	})

	t.Run("RemoveMember - Success and Not Found", func(t *testing.T) {
		err := repo.RemoveMember(ctx, nil, teamID, user2ID)
		require.NoError(t, err)

		isMember, err := repo.IsMember(ctx, nil, teamID, user2ID)
		require.NoError(t, err)
		assert.False(t, isMember)

		err = repo.RemoveMember(ctx, nil, teamID, user2ID)
		assert.ErrorIs(t, err, repository.ErrMemberNotFound)
	})
}
