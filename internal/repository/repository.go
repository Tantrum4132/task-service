package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Tantrum4132/task-service/internal/model"
)

// Ошибки слоя репозитория
var (
	ErrUserNotFound              = errors.New("user not found")
	ErrEmailAlreadyExists        = errors.New("user with this email already exists")
	ErrTeamNotFound              = errors.New("team not found")
	ErrMemberNotFound            = errors.New("team member not found")
	ErrMemberExists              = errors.New("user is already a member of this team")
	ErrForeignKeyViolation       = errors.New("referenced entity (team or user) does not exist")
	ErrInvalidTaskHistoryPayload = errors.New("invalid task history payload: created_at must be initialized by service")
	ErrTaskIDRequired            = errors.New("task_id is required for getting history")
	ErrTaskNotFound              = errors.New("task not found")
	ErrTaskConflict              = errors.New("task was updated by another process (version conflict)")
	ErrTeamIDRequired            = errors.New("team_id is required for listing tasks")
	ErrInvalidTaskPayload        = errors.New("invalid task payload: created_at, updated_at and version must be initialized by service")
	ErrCommentNotFound           = errors.New("comment not found")
	ErrInvalidCommentPayload     = errors.New("invalid comment payload: created_at must be initialized by service")
)

// DBEngine — абстракция над *sql.DB и *sql.Tx для работы в транзакциях и без
//
//go:generate mockgen -destination=../../mocks/db_engine_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository DBEngine
type DBEngine interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// TaskCommentRepository предоставляет доступ к данным комментариев.
//
//go:generate mockgen -destination=../../mocks/task_comment_repository_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository TaskCommentRepository
type TaskCommentRepository interface {
	CreateComment(ctx context.Context, exec DBEngine, comment *model.TaskComment) error
	GetCommentByID(ctx context.Context, exec DBEngine, id int64) (*model.TaskComment, error)
	ListCommentsByTaskID(ctx context.Context, exec DBEngine, taskID int64) ([]model.TaskComment, error)
	DeleteComment(ctx context.Context, exec DBEngine, id int64) error
}

// TaskHistoryRepository описывает интерфейс взаимодействия с таблицей task_history
//
//go:generate mockgen -destination=../../mocks/task_history_repository_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository TaskHistoryRepository
type TaskHistoryRepository interface {
	CreateTaskHistory(ctx context.Context, exec DBEngine, history *model.TaskHistory) error
	GetHistoryByTaskID(ctx context.Context, exec DBEngine, filter TaskHistoryFilter) ([]model.TaskHistory, error)
}

// TeamMemberRepository — интерфейс работы с таблицей team_members
//
//go:generate mockgen -destination=../../mocks/team_member_repo_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository TeamMemberRepository
type TeamMemberRepository interface {
	AddMember(ctx context.Context, exec DBEngine, member *model.TeamMember) error
	UpdateMemberRole(ctx context.Context, exec DBEngine, teamID, userID int64, newRole model.TeamRole) error
	RemoveMember(ctx context.Context, exec DBEngine, teamID, userID int64) error
	GetMember(ctx context.Context, exec DBEngine, teamID, userID int64) (*model.TeamMember, error)
	GetMemberRole(ctx context.Context, exec DBEngine, teamID, userID int64) (model.TeamRole, error)
	ListTeamMembers(ctx context.Context, exec DBEngine, teamID int64) ([]model.TeamMember, error)
	IsMember(ctx context.Context, exec DBEngine, teamID, userID int64) (bool, error)
}

// TeamRepository — интерфейс работы с сущностью команд
//
//go:generate mockgen -destination=../../mocks/team_repository_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository TeamRepository
type TeamRepository interface {
	CreateTeam(ctx context.Context, exec DBEngine, team *model.Team) error
	GetTeamByID(ctx context.Context, exec DBEngine, id int64) (*model.Team, error)
	GetUserTeams(ctx context.Context, exec DBEngine, userID int64) ([]model.Team, error)
}

// UserRepository — интерфейс для работы с пользователями
//
//go:generate mockgen -destination=../../mocks/user_repository_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository UserRepository
type UserRepository interface {
	Create(ctx context.Context, exec DBEngine, user *model.User) error
	FindByEmail(ctx context.Context, exec DBEngine, email string) (*model.User, error)
	FindByID(ctx context.Context, exec DBEngine, id int64) (*model.User, error)
	Update(ctx context.Context, exec DBEngine, user *model.User) error
	Delete(ctx context.Context, exec DBEngine, id int64) error
}

// TaskRepository — интерфейс работы с таблицей tasks
//
//go:generate mockgen -destination=../../mocks/task_repo_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository TaskRepository
type TaskRepository interface {
	CreateTask(ctx context.Context, exec DBEngine, task *model.Task) error
	GetTaskByID(ctx context.Context, exec DBEngine, id int64) (*model.Task, error)
	UpdateTask(ctx context.Context, exec DBEngine, task *model.Task) error
	DeleteTask(ctx context.Context, exec DBEngine, id int64) error
	ListTasks(ctx context.Context, exec DBEngine, filter TaskFilter) ([]model.Task, error)
}

// StatsRepository декларирует интерфейс для работы с аналитикой.
//
//go:generate mockgen -destination=../../mocks/stat_repository_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/repository StatsRepository
type StatsRepository interface {
	GetTeamStats(ctx context.Context, teamID int64) (*TeamStats, error)
}

// TaskHistoryFilter содержит параметры фильтрации и пагинации истории задач
type TaskHistoryFilter struct {
	TaskID int64
	Limit  int
	Offset int
}

// TaskFilter содержит параметры фильтрации для получения списка задач
type TaskFilter struct {
	TeamID     int64
	Status     *model.TaskStatus
	AssigneeID *int64
	Limit      int
	Offset     int
}

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
