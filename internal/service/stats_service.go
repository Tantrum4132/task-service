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

type statsService struct {
	statsRepo      repository.StatsRepository
	teamMemberRepo repository.TeamMemberRepository
	logger         *zap.Logger
}

func NewStatsService(
	statsRepo repository.StatsRepository,
	teamMemberRepo repository.TeamMemberRepository,
	logger *zap.Logger,
) StatsService {
	return &statsService{
		statsRepo:      statsRepo,
		teamMemberRepo: teamMemberRepo,
		logger:         logger,
	}
}

func (s *statsService) GetTeamStats(ctx context.Context, userID, teamID int64) (*dto.TaskStatsResponse, error) {
	role, err := s.teamMemberRepo.GetMemberRole(ctx, nil, teamID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrMemberNotFound) {
			return nil, ErrPermissionDenied
		}
		return nil, fmt.Errorf("get member role: %w", err)
	}

	if role != model.TeamRoleOwner && role != model.TeamRoleAdmin {
		return nil, ErrPermissionDenied
	}

	rawStats, err := s.statsRepo.GetTeamStats(ctx, teamID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("get team stats repo: %w", err)
	}

	topAssignees := make([]dto.TopAssignee, len(rawStats.TopAssignees))
	for i, ta := range rawStats.TopAssignees {
		topAssignees[i] = dto.TopAssignee{
			UserID:      ta.UserID,
			UserName:    ta.Name,
			ClosedTasks: int(ta.ClosedTasks),
		}
	}

	resp := &dto.TaskStatsResponse{
		TeamID: teamID,
		TasksByStatus: map[string]int{
			"todo":        int(rawStats.Statuses.Todo),
			"in_progress": int(rawStats.Statuses.InProgress),
			"done":        int(rawStats.Statuses.Done),
		},
		TopAssignees:        topAssignees,
		AvgTimeToCloseHours: rawStats.AvgCloseTimeHours,
		TotalComments:       rawStats.TotalCommentsCount,
		UpdatedAt:           time.Now().UTC(),
	}

	return resp, nil
}
