package dto

import "time"

type TaskHistoryResponse struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	ChangedBy int64     `json:"changed_by"`
	Changes   any       `json:"changes"`
	CreatedAt time.Time `json:"created_at"`
}
