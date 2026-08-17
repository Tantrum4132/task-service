package dto

import "time"

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

type TaskFilterQuery struct {
	TeamID     int64   `form:"team_id" validate:"required,gt=0"`
	Status     *string `form:"status,omitempty" validate:"omitempty,oneof=todo in_progress done"`
	AssigneeID *int64  `form:"assignee_id,omitempty" validate:"omitempty,gt=0"`
	Limit      int     `form:"limit,default=20" validate:"gte=1,lte=100"`
	Offset     int     `form:"offset,default=0" validate:"gte=0"`
}
