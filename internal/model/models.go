package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type TaskComment struct {
	ID        int64     `db:"id"`
	TaskID    int64     `db:"task_id"`
	UserID    int64     `db:"user_id"`
	Content   string    `db:"content"`
	CreatedAt time.Time `db:"created_at"`
}

// TaskHistoryChangeItem представляет собой пару старого и нового значения
type TaskHistoryChangeItem struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// TaskHistoryChange представляет словарь изменённых полей
type TaskHistoryChange map[string]TaskHistoryChangeItem

// Value реализует интерфейс driver.Valuer для сохранения структуры в поле типа JSON в MySQL
func (c TaskHistoryChange) Value() (driver.Value, error) {
	if c == nil {
		return "{}", nil
	}
	return json.Marshal(c)
}

// Scan реализует интерфейс sql.Scanner для корректного считывания JSON из MySQL.
// Поддерживает []byte, string и NULL (nil) значения.
func (c *TaskHistoryChange) Scan(value any) error {
	if value == nil {
		*c = make(TaskHistoryChange)
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type for TaskHistoryChange: %T", value)
	}

	if len(data) == 0 {
		*c = make(TaskHistoryChange)
		return nil
	}

	return json.Unmarshal(data, c)
}

// TaskHistory описывает доменную сущность истории изменений задачи
type TaskHistory struct {
	ID        int64             `db:"id"`
	TaskID    int64             `db:"task_id"`
	ChangedBy int64             `db:"changed_by"`
	Changes   TaskHistoryChange `db:"changes"`
	CreatedAt time.Time         `db:"created_at"`
}

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusTodo, TaskStatusInProgress, TaskStatusDone:
		return true
	default:
		return false
	}
}

func (s TaskStatus) String() string {
	return string(s)
}

func ValidateTaskStatus(status TaskStatus) error {
	if !status.Valid() {
		return errors.New("invalid task status: must be one of 'todo', 'in_progress', 'done'")
	}
	return nil
}

type Task struct {
	ID          int64      `db:"id"`
	TeamID      int64      `db:"team_id"`
	Title       string     `db:"title"`
	Description string     `db:"description"`
	Status      TaskStatus `db:"status"`
	CreatedBy   int64      `db:"created_by"`
	AssigneeID  *int64     `db:"assignee_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	ClosedAt    *time.Time `db:"closed_at"`
	Version     int        `db:"version"`
}

type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
)

func (r TeamRole) Valid() bool {
	switch r {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleMember:
		return true
	default:
		return false
	}
}

func (r TeamRole) String() string {
	return string(r)
}

func ValidateTeamRole(role TeamRole) error {
	if !role.Valid() {
		return fmt.Errorf("invalid team role '%s': must be one of 'owner', 'admin', 'member'", role)
	}
	return nil
}

type TeamMember struct {
	TeamID int64    `db:"team_id"`
	UserID int64    `db:"user_id"`
	Role   TeamRole `db:"role"`
}

type Team struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedBy int64     `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
}

type User struct {
	ID           int64     `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Name         string    `db:"name"`
	CreatedAt    time.Time `db:"created_at"`
}
