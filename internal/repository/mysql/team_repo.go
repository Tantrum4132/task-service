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

type teamRepository struct {
	db *sql.DB
}

// NewTeamRepository создает новый экземпляр репозитория команд
func NewTeamRepository(db *sql.DB) repository.TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) getExec(exec repository.DBEngine) repository.DBEngine {
	if exec != nil {
		return exec
	}
	return r.db
}

// CreateTeam создает команду
func (r *teamRepository) CreateTeam(ctx context.Context, exec repository.DBEngine, team *model.Team) error {
	if team == nil {
		return errors.New("team cannot be nil")
	}

	query := `
		INSERT INTO teams (name, created_by, created_at)
		VALUES (?, ?, ?)
	`
	runner := r.getExec(exec)

	res, err := runner.ExecContext(ctx, query, team.Name, team.CreatedBy, team.CreatedAt)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrForeignKeyFailed {
			return repository.ErrForeignKeyViolation
		}
		return fmt.Errorf("failed to insert team: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get inserted team id: %w", err)
	}

	team.ID = id
	return nil
}

// GetTeamByID возвращает команду по ее ID
func (r *teamRepository) GetTeamByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.Team, error) {
	query := `
		SELECT id, name, created_by, created_at
		FROM teams
		WHERE id = ?
	`
	runner := r.getExec(exec)

	var team model.Team
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.CreatedBy,
		&team.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team by id: %w", err)
	}

	return &team, nil
}

// GetUserTeams возвращает список команд, в которых состоит пользователь
func (r *teamRepository) GetUserTeams(ctx context.Context, exec repository.DBEngine, userID int64) ([]model.Team, error) {
	query := `
		SELECT t.id, t.name, t.created_by, t.created_at
		FROM teams t
		INNER JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = ?
		ORDER BY t.created_at DESC
	`
	runner := r.getExec(exec)

	rows, err := runner.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user teams: %w", err)
	}
	defer rows.Close()

	teams := make([]model.Team, 0)
	for rows.Next() {
		var t model.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan team row: %w", err)
		}
		teams = append(teams, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return teams, nil
}
