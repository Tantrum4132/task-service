package model

// TaskStatusStats содержит агрегированное количество задач по статусам.
type TaskStatusStats struct {
	Todo       int64 `json:"todo"`
	InProgress int64 `json:"in_progress"`
	Done       int64 `json:"done"`
}

// TopAssignee представляет топ-исполнителя по закрытым задачам за последние 30 дней.
type TopAssignee struct {
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	ClosedTasks int64  `json:"closed_tasks"`
}

// TeamStats объединяет все требуемые по ТЗ метрики отчета по команде.
type TeamStats struct {
	TeamID             int64           `json:"team_id"`
	Statuses           TaskStatusStats `json:"statuses"`
	AvgCloseTimeHours  float64         `json:"avg_close_time_hours"`
	TotalCommentsCount int64           `json:"total_comments_count"`
	TopAssignees       []TopAssignee   `json:"top_assignees"`
}
