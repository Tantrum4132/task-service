package dto

import "time"

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
