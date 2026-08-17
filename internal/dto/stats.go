package dto

import "time"

type StatsQuery struct {
	TeamID int64 `form:"team_id" validate:"required,gt=0"`
}

type TopAssignee struct {
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	ClosedTasks int    `json:"closed_tasks"`
}

type TaskStatsResponse struct {
	TeamID              int64          `json:"team_id"`
	TasksByStatus       map[string]int `json:"tasks_by_status"`
	TopAssignees        []TopAssignee  `json:"top_assignees"`
	AvgTimeToCloseHours float64        `json:"avg_time_to_close_hours"`
	TotalComments       int64          `json:"total_comments"`
	UpdatedAt           time.Time      `json:"updated_at"`
}
