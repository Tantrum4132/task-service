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

type teamMemberRepository struct {
	db *sql.DB
}

func NewTeamMemberRepository(db *sql.DB) repository.TeamMemberRepository {
	return &teamMemberRepository{db: db}
}

func (r *teamMemberRepository) getExec(exec repository.DBEngine) repository.DBEngine {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *teamMemberRepository) AddMember(ctx context.Context, exec repository.DBEngine, member *model.TeamMember) error {
	if member == nil {
		return errors.New("team member cannot be nil")
	}

	query := `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES (?, ?, ?)
	`
	runner := r.getExec(exec)

	_, err := runner.ExecContext(ctx, query, member.TeamID, member.UserID, member.Role)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) {
			switch mysqlErr.Number {
			case mysqlErrDuplicateEntry:
				return repository.ErrMemberExists
			case mysqlErrForeignKeyFailed:
				return repository.ErrForeignKeyViolation
			}
		}
		return fmt.Errorf("failed to add team member: %w", err)
	}

	return nil
}

func (r *teamMemberRepository) UpdateMemberRole(ctx context.Context, exec repository.DBEngine, teamID, userID int64, newRole model.TeamRole) error {
	query := `
		UPDATE team_members
		SET role = ?
		WHERE team_id = ? AND user_id = ?
	`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query, newRole, teamID, userID)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return repository.ErrMemberNotFound
	}

	return nil
}

func (r *teamMemberRepository) RemoveMember(ctx context.Context, exec repository.DBEngine, teamID, userID int64) error {
	query := `
		DELETE FROM team_members
		WHERE team_id = ? AND user_id = ?
	`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return repository.ErrMemberNotFound
	}

	return nil
}

func (r *teamMemberRepository) GetMember(ctx context.Context, exec repository.DBEngine, teamID, userID int64) (*model.TeamMember, error) {
	query := `
		SELECT team_id, user_id, role
		FROM team_members
		WHERE team_id = ? AND user_id = ?
	`
	runner := r.getExec(exec)
	var member model.TeamMember

	err := runner.QueryRowContext(ctx, query, teamID, userID).Scan(
		&member.TeamID,
		&member.UserID,
		&member.Role,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to get team member: %w", err)
	}

	return &member, nil
}

func (r *teamMemberRepository) GetMemberRole(ctx context.Context, exec repository.DBEngine, teamID, userID int64) (model.TeamRole, error) {
	query := `
		SELECT role
		FROM team_members
		WHERE team_id = ? AND user_id = ?
	`
	runner := r.getExec(exec)
	var role model.TeamRole

	err := runner.QueryRowContext(ctx, query, teamID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", repository.ErrMemberNotFound
		}
		return "", fmt.Errorf("failed to get member role: %w", err)
	}

	return role, nil
}

func (r *teamMemberRepository) ListTeamMembers(ctx context.Context, exec repository.DBEngine, teamID int64) ([]model.TeamMember, error) {
	query := `
		SELECT team_id, user_id, role
		FROM team_members
		WHERE team_id = ?
		ORDER BY user_id ASC
	`
	runner := r.getExec(exec)

	rows, err := runner.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list team members: %w", err)
	}
	defer rows.Close()

	members := make([]model.TeamMember, 0, 10)
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role); err != nil {
			return nil, fmt.Errorf("failed to scan team member row: %w", err)
		}
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return members, nil
}

func (r *teamMemberRepository) IsMember(ctx context.Context, exec repository.DBEngine, teamID, userID int64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM team_members
			WHERE team_id = ? AND user_id = ?
		)
	`
	runner := r.getExec(exec)
	var exists bool

	err := runner.QueryRowContext(ctx, query, teamID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check team membership: %w", err)
	}

	return exists, nil
}
