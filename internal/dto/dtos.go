package dto

import "time"

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=6,max=100"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type CreateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=2000"`
}

type TaskCommentResponse struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

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

// TaskFilterQuery — query-параметры для GET /api/v1/tasks
type TaskFilterQuery struct {
	TeamID     int64   `form:"team_id" validate:"required,gt=0"`
	Status     *string `form:"status,omitempty" validate:"omitempty,oneof=todo in_progress done"`
	AssigneeID *int64  `form:"assignee_id,omitempty" validate:"omitempty,gt=0"`
	Limit      int     `form:"limit,default=20" validate:"gte=1,lte=100"`
	Offset     int     `form:"offset,default=0" validate:"gte=0"`
}

type CreateTaskRequest struct {
	TeamID      int64  `json:"team_id" validate:"required,gt=0"`
	Title       string `json:"title" validate:"required,min=1,max=255"`
	Description string `json:"description,omitempty" validate:"omitempty,max=5000"`
	AssigneeID  *int64 `json:"assignee_id,omitempty" validate:"omitempty,gt=0"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=5000"`
	Status      *string `json:"status,omitempty" validate:"omitempty,oneof=todo in_progress done"`
	AssigneeID  *int64  `json:"assignee_id,omitempty" validate:"omitempty,gt=0"`
	Version     int     `json:"version" validate:"required,gt=0"` // Обязательно для оптимистической блокировки
}

type TaskResponse struct {
	ID          int64      `json:"id"`
	TeamID      int64      `json:"team_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"`
	CreatedBy   int64      `json:"created_by"`
	AssigneeID  *int64     `json:"assignee_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	Version     int        `json:"version"`
}

type TaskHistoryResponse struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	ChangedBy int64     `json:"changed_by"`
	Changes   any       `json:"changes"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTeamRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type InviteMemberRequest struct {
	UserID int64  `json:"user_id" validate:"required,gt=0"`
	Role   string `json:"role" validate:"required,oneof=admin member"`
}

type TeamResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
