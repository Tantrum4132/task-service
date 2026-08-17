package dto

import "time"

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
