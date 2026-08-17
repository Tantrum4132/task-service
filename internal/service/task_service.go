package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"

	"go.uber.org/zap"
)

type taskService struct {
	taskRepo         repository.TaskRepository
	teamMemberRepo   repository.TeamMemberRepository
	taskHistoryRepo  repository.TaskHistoryRepository
	taskCommentRepo  repository.TaskCommentRepository
	transactor       Transactor
	cacheInvalidator TaskCacheInvalidator
	logger           *zap.Logger
}

func NewTaskService(
	taskRepo repository.TaskRepository,
	teamMemberRepo repository.TeamMemberRepository,
	taskHistoryRepo repository.TaskHistoryRepository,
	taskCommentRepo repository.TaskCommentRepository,
	transactor Transactor,
	cacheInvalidator TaskCacheInvalidator,
	logger *zap.Logger,
) TaskService {
	return &taskService{
		taskRepo:         taskRepo,
		teamMemberRepo:   teamMemberRepo,
		taskHistoryRepo:  taskHistoryRepo,
		taskCommentRepo:  taskCommentRepo,
		transactor:       transactor,
		cacheInvalidator: cacheInvalidator,
		logger:           logger,
	}
}

func (s *taskService) CreateTask(ctx context.Context, userID int64, req dto.CreateTaskRequest) (*dto.TaskResponse, error) {
	isMember, err := s.teamMemberRepo.IsMember(ctx, nil, req.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check member status: %w", err)
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	if req.AssigneeID != nil {
		assigneeMember, err := s.teamMemberRepo.IsMember(ctx, nil, req.TeamID, *req.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("check assignee status: %w", err)
		}
		if !assigneeMember {
			return nil, ErrInvalidAssignee
		}
	}

	now := time.Now().UTC()
	task := &model.Task{
		TeamID:      req.TeamID,
		Title:       req.Title,
		Description: req.Description,
		Status:      model.TaskStatusTodo,
		CreatedBy:   userID,
		AssigneeID:  req.AssigneeID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
	}

	err = s.transactor.WithinTransaction(ctx, func(exec repository.DBEngine) error {
		if err := s.taskRepo.CreateTask(ctx, exec, task); err != nil {
			return fmt.Errorf("create task repo: %w", err)
		}

		changes := model.TaskHistoryChange{
			"title":  model.TaskHistoryChangeItem{Old: nil, New: task.Title},
			"status": model.TaskHistoryChangeItem{Old: nil, New: task.Status.String()},
		}
		if task.Description != "" {
			changes["description"] = model.TaskHistoryChangeItem{Old: nil, New: task.Description}
		}
		if task.AssigneeID != nil {
			changes["assignee_id"] = model.TaskHistoryChangeItem{Old: nil, New: *task.AssigneeID}
		}

		history := &model.TaskHistory{
			TaskID:    task.ID,
			ChangedBy: userID,
			Changes:   changes,
			CreatedAt: now,
		}

		if err := s.taskHistoryRepo.CreateTaskHistory(ctx, exec, history); err != nil {
			return fmt.Errorf("create task history repo: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.cacheInvalidator != nil {
		if err := s.cacheInvalidator.InvalidateTeamCache(ctx, task.TeamID); err != nil {
			s.logger.Warn("failed to invalidate team cache after task creation",
				zap.Int64("team_id", task.TeamID),
				zap.Error(err),
			)
		}
	}

	return s.toTaskResponse(task), nil
}

func (s *taskService) GetTaskByID(ctx context.Context, userID, taskID int64) (*dto.TaskResponse, error) {
	task, err := s.taskRepo.GetTaskByID(ctx, nil, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}

	isMember, err := s.teamMemberRepo.IsMember(ctx, nil, task.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check member status: %w", err)
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	return s.toTaskResponse(task), nil
}

func (s *taskService) ListTasks(ctx context.Context, userID int64, req dto.TaskFilterQuery) ([]dto.TaskResponse, error) {
	isMember, err := s.teamMemberRepo.IsMember(ctx, nil, req.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check member status: %w", err)
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	filter := model.TaskFilter{
		TeamID:     req.TeamID,
		AssigneeID: req.AssigneeID,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}
	if req.Status != nil {
		st := model.TaskStatus(*req.Status)
		filter.Status = &st
	}

	tasks, err := s.taskRepo.ListTasks(ctx, nil, filter)
	if err != nil {
		return nil, fmt.Errorf("list tasks repo: %w", err)
	}

	responses := make([]dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		responses[i] = *s.toTaskResponse(&task)
	}

	return responses, nil
}

func (s *taskService) UpdateTask(ctx context.Context, userID, taskID int64, req dto.UpdateTaskRequest) (*dto.TaskResponse, error) {
	task, err := s.taskRepo.GetTaskByID(ctx, nil, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}

	if task.Version != req.Version {
		return nil, ErrVersionConflict
	}

	role, err := s.teamMemberRepo.GetMemberRole(ctx, nil, task.TeamID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrMemberNotFound) {
			return nil, ErrPermissionDenied
		}
		return nil, fmt.Errorf("get member role: %w", err)
	}

	isCreator := task.CreatedBy == userID
	isAssignee := task.AssigneeID != nil && *task.AssigneeID == userID
	isOwnerOrAdmin := role == model.TeamRoleOwner || role == model.TeamRoleAdmin

	if !isOwnerOrAdmin && !isCreator && !isAssignee {
		return nil, ErrPermissionDenied
	}

	if isAssignee && !isOwnerOrAdmin && !isCreator {
		if req.Title != nil || req.Description != nil || req.AssigneeID != nil {
			return nil, ErrPermissionDenied
		}
	}

	changes := make(model.TaskHistoryChange)
	now := time.Now().UTC()

	if req.AssigneeID != nil && (task.AssigneeID == nil || *task.AssigneeID != *req.AssigneeID) {
		assigneeMember, err := s.teamMemberRepo.IsMember(ctx, nil, task.TeamID, *req.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("check assignee status: %w", err)
		}
		if !assigneeMember {
			return nil, ErrInvalidAssignee
		}
		var oldVal any
		if task.AssigneeID != nil {
			oldVal = *task.AssigneeID
		}
		changes["assignee_id"] = model.TaskHistoryChangeItem{Old: oldVal, New: *req.AssigneeID}
		task.AssigneeID = req.AssigneeID
	}

	if req.Title != nil && *req.Title != task.Title {
		changes["title"] = model.TaskHistoryChangeItem{Old: task.Title, New: *req.Title}
		task.Title = *req.Title
	}

	if req.Description != nil && *req.Description != task.Description {
		changes["description"] = model.TaskHistoryChangeItem{Old: task.Description, New: *req.Description}
		task.Description = *req.Description
	}

	if req.Status != nil && *req.Status != string(task.Status) {
		newStatus := model.TaskStatus(*req.Status)
		if err := model.ValidateTaskStatus(newStatus); err != nil {
			return nil, err
		}
		changes["status"] = model.TaskHistoryChangeItem{Old: task.Status.String(), New: newStatus.String()}
		task.Status = newStatus

		if newStatus == model.TaskStatusDone {
			task.ClosedAt = &now
		} else {
			task.ClosedAt = nil
		}
	}

	if len(changes) == 0 {
		return s.toTaskResponse(task), nil
	}

	task.Version++
	task.UpdatedAt = now

	err = s.transactor.WithinTransaction(ctx, func(exec repository.DBEngine) error {
		if err := s.taskRepo.UpdateTask(ctx, exec, task); err != nil {
			if errors.Is(err, repository.ErrTaskConflict) {
				return ErrVersionConflict
			}
			return fmt.Errorf("update task repo: %w", err)
		}

		history := &model.TaskHistory{
			TaskID:    task.ID,
			ChangedBy: userID,
			Changes:   changes,
			CreatedAt: now,
		}

		if err := s.taskHistoryRepo.CreateTaskHistory(ctx, exec, history); err != nil {
			return fmt.Errorf("create task history repo: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.cacheInvalidator != nil {
		if err := s.cacheInvalidator.InvalidateTeamCache(ctx, task.TeamID); err != nil {
			s.logger.Warn("failed to invalidate team cache after task update",
				zap.Int64("team_id", task.TeamID),
				zap.Error(err),
			)
		}
	}

	return s.toTaskResponse(task), nil
}

func (s *taskService) checkMember(ctx context.Context, userID, teamID int64) error {
	isMember, err := s.teamMemberRepo.IsMember(ctx, nil, teamID, userID)
	if err != nil {
		return fmt.Errorf("check member status: %w", err)
	}
	if !isMember {
		return ErrPermissionDenied
	}
	return nil
}

func (s *taskService) GetTaskHistory(ctx context.Context, userID, taskID int64, limit, offset int) ([]dto.TaskHistoryResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	task, err := s.taskRepo.GetTaskByID(ctx, nil, taskID)
	if err != nil {
		return nil, ErrTaskNotFound
	}

	if err := s.checkMember(ctx, userID, task.TeamID); err != nil {
		return nil, err
	}

	filter := model.TaskHistoryFilter{TaskID: taskID, Limit: limit, Offset: offset}
	histories, err := s.taskHistoryRepo.GetHistoryByTaskID(ctx, nil, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.TaskHistoryResponse, len(histories))
	for i, h := range histories {
		responses[i] = dto.TaskHistoryResponse{
			ID:        h.ID,
			TaskID:    h.TaskID,
			ChangedBy: h.ChangedBy,
			Changes:   h.Changes,
			CreatedAt: h.CreatedAt,
		}
	}

	return responses, nil
}

func (s *taskService) CreateComment(ctx context.Context, userID, taskID int64, req dto.CreateCommentRequest) (*dto.TaskCommentResponse, error) {
	task, err := s.taskRepo.GetTaskByID(ctx, nil, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}

	isMember, err := s.teamMemberRepo.IsMember(ctx, nil, task.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check member status: %w", err)
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	comment := &model.TaskComment{
		TaskID:    taskID,
		UserID:    userID,
		Content:   req.Content,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.taskCommentRepo.CreateComment(ctx, nil, comment); err != nil {
		return nil, fmt.Errorf("create comment repo: %w", err)
	}

	return &dto.TaskCommentResponse{
		ID:        comment.ID,
		TaskID:    comment.TaskID,
		UserID:    comment.UserID,
		Content:   comment.Content,
		CreatedAt: comment.CreatedAt,
	}, nil
}

func (s *taskService) ListComments(ctx context.Context, userID, taskID int64) ([]dto.TaskCommentResponse, error) {
	task, err := s.taskRepo.GetTaskByID(ctx, nil, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task by id: %w", err)
	}

	isMember, err := s.teamMemberRepo.IsMember(ctx, nil, task.TeamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check member status: %w", err)
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	comments, err := s.taskCommentRepo.ListCommentsByTaskID(ctx, nil, taskID)
	if err != nil {
		return nil, fmt.Errorf("list comments repo: %w", err)
	}

	responses := make([]dto.TaskCommentResponse, len(comments))
	for i, c := range comments {
		responses[i] = dto.TaskCommentResponse{
			ID:        c.ID,
			TaskID:    c.TaskID,
			UserID:    c.UserID,
			Content:   c.Content,
			CreatedAt: c.CreatedAt,
		}
	}

	return responses, nil
}

func (s *taskService) toTaskResponse(t *model.Task) *dto.TaskResponse {
	return &dto.TaskResponse{
		ID:          t.ID,
		TeamID:      t.TeamID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status.String(),
		CreatedBy:   t.CreatedBy,
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		ClosedAt:    t.ClosedAt,
		Version:     t.Version,
	}
}
