package service

import (
	"context"
	"errors"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/repository"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrTaskNotFound     = errors.New("task not found")
	ErrUserNotFound     = errors.New("user not found")
	ErrTeamNotFound     = errors.New("team not found")
	ErrInvalidAssignee  = errors.New("assignee must be a member of the same team")
	ErrVersionConflict  = errors.New("version conflict: task has been updated by another user")
)

// Transactor декларирует абстракцию для выполнения нескольких операций в транзакции.
//
//go:generate mockgen -destination=../../mocks/transactor_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/service Transactor
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(exec repository.DBEngine) error) error
}

// TaskCacheInvalidator позволяет сбрасывать кеш списка задач команды.
//
//go:generate mockgen -destination=../../mocks/task_cache_invalidator_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/service TaskCacheInvalidator
type TaskCacheInvalidator interface {
	InvalidateTeamCache(ctx context.Context, teamID int64) error
}

// TaskService предоставляет методы для управления задачами, историей и комментариями.
//
//go:generate mockgen -destination=../../mocks/task_service_mock.go -package=mocks github.com/Tantrum4132/task-service/internal/service TaskService
type TaskService interface {
	CreateTask(ctx context.Context, userID int64, req dto.CreateTaskRequest) (*dto.TaskResponse, error)
	GetTaskByID(ctx context.Context, userID, taskID int64) (*dto.TaskResponse, error)
	ListTasks(ctx context.Context, userID int64, req dto.TaskFilterQuery) ([]dto.TaskResponse, error)
	UpdateTask(ctx context.Context, userID, taskID int64, req dto.UpdateTaskRequest) (*dto.TaskResponse, error)
	GetTaskHistory(ctx context.Context, userID, taskID int64, limit, offset int) ([]dto.TaskHistoryResponse, error)
	CreateComment(ctx context.Context, userID, taskID int64, req dto.CreateCommentRequest) (*dto.TaskCommentResponse, error)
	ListComments(ctx context.Context, userID, taskID int64) ([]dto.TaskCommentResponse, error)
}

// StatsService предоставляет методы для получения отчетов и аналитики.
//
//go:generate mockgen -destination=../../mocks/stats_service.go -package=mocks github.com/Tantrum4132/task-service/internal/service StatsService
type StatsService interface {
	GetTeamStats(ctx context.Context, userID, teamID int64) (*dto.TaskStatsResponse, error)
}
