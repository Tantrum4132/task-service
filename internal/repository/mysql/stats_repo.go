package mysql

import (
	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"

	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type statsRepository struct {
	db *sql.DB
}

// NewStatsRepository создает экземпляр репозитория статистики.
func NewStatsRepository(db *sql.DB) repository.StatsRepository {
	return &statsRepository{db: db}
}

// GetTeamStats возвращает статистику по команде за один CTE-запрос без N+1.
func (r *statsRepository) GetTeamStats(ctx context.Context, teamID int64) (*model.TeamStats, error) {
	const query = `
WITH team_tasks AS (
    SELECT id, status, created_at, closed_at, assignee_id
    FROM tasks
    WHERE team_id = ?
),
status_counts AS (
    SELECT 
        COALESCE(SUM(CASE WHEN status = 'todo' THEN 1 ELSE 0 END), 0) AS count_todo,
        COALESCE(SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END), 0) AS count_in_progress,
        COALESCE(SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END), 0) AS count_done
    FROM team_tasks
),
avg_closing AS (
    SELECT 
        COALESCE(AVG(TIMESTAMPDIFF(SECOND, created_at, closed_at)), 0) AS avg_seconds
    FROM team_tasks
    WHERE status = 'done' AND closed_at IS NOT NULL
),
comment_counts AS (
    SELECT 
        COUNT(tc.id) AS total_comments
    FROM task_comments tc
    JOIN team_tasks tt ON tc.task_id = tt.id
),
top_assignees_raw AS (
    SELECT 
        u.id AS user_id,
        u.name,
        u.email,
        COUNT(tt.id) AS closed_count
    FROM team_tasks tt
    JOIN users u ON tt.assignee_id = u.id
    WHERE tt.status = 'done' 
      AND tt.closed_at >= NOW() - INTERVAL 30 DAY
    GROUP BY u.id, u.name, u.email
    ORDER BY closed_count DESC
    LIMIT 3
),
top_assignees_json AS (
    SELECT 
        COALESCE(
            JSON_ARRAYAGG(
                JSON_OBJECT(
                    'user_id', user_id,
                    'name', name,
                    'email', email,
                    'closed_tasks', closed_count
                )
            ), 
            JSON_ARRAY()
        ) AS json_data
    FROM top_assignees_raw
)
SELECT 
    sc.count_todo,
    sc.count_in_progress,
    sc.count_done,
    ac.avg_seconds,
    cc.total_comments,
    taj.json_data
FROM status_counts sc
CROSS JOIN avg_closing ac
CROSS JOIN comment_counts cc
CROSS JOIN top_assignees_json taj;
`

	var (
		todoCount       int64
		inProgressCount int64
		doneCount       int64
		avgSeconds      float64
		commentsCount   int64
		rawTopAssignees []byte
	)

	// Так как запрос всегда гарантированно возвращает ровно 1 строку,
	// используем QueryRowContext вместо цикла по rows.Next()
	err := r.db.QueryRowContext(ctx, query, teamID).Scan(
		&todoCount,
		&inProgressCount,
		&doneCount,
		&avgSeconds,
		&commentsCount,
		&rawTopAssignees,
	)
	if err != nil {
		return nil, fmt.Errorf("statsRepository.GetTeamStats scan error: %w", err)
	}

	topAssignees := make([]model.TopAssignee, 0, 3)
	if len(rawTopAssignees) > 0 {
		if err := json.Unmarshal(rawTopAssignees, &topAssignees); err != nil {
			return nil, fmt.Errorf("statsRepository.GetTeamStats unmarshal top assignees error: %w", err)
		}
	}

	stats := &model.TeamStats{
		TeamID: teamID,
		Statuses: model.TaskStatusStats{
			Todo:       todoCount,
			InProgress: inProgressCount,
			Done:       doneCount,
		},
		AvgCloseTimeHours:  avgSeconds / 3600.0,
		TotalCommentsCount: commentsCount,
		TopAssignees:       topAssignees,
	}

	return stats, nil
}
