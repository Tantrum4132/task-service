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

var (
	ErrAccessDenied      = errors.New("access denied: insufficient permissions")
	ErrUserAlreadyInTeam = errors.New("user is already a member of this team")
	ErrCannotInviteOwner = errors.New("cannot grant owner role via invitation")
)

type TeamService interface {
	CreateTeam(ctx context.Context, userID int64, req dto.CreateTeamRequest) (*dto.TeamResponse, error)
	GetUserTeams(ctx context.Context, userID int64) ([]dto.TeamResponse, error)
	InviteMember(ctx context.Context, actorID, teamID int64, req dto.InviteMemberRequest) error
}

type teamService struct {
	teamRepo       repository.TeamRepository
	teamMemberRepo repository.TeamMemberRepository
	userRepo       repository.UserRepository
	transactor     Transactor
	db             repository.DBEngine
	logger         *zap.Logger
}

func NewTeamService(
	teamRepo repository.TeamRepository,
	teamMemberRepo repository.TeamMemberRepository,
	userRepo repository.UserRepository,
	transactor Transactor,
	db repository.DBEngine,
	logger *zap.Logger,
) TeamService {
	return &teamService{
		teamRepo:       teamRepo,
		teamMemberRepo: teamMemberRepo,
		userRepo:       userRepo,
		transactor:     transactor,
		db:             db,
		logger:         logger,
	}
}

func (s *teamService) CreateTeam(ctx context.Context, userID int64, req dto.CreateTeamRequest) (*dto.TeamResponse, error) {
	var team *model.Team

	err := s.transactor.WithinTransaction(ctx, func(exec repository.DBEngine) error {
		t := &model.Team{
			Name:      req.Name,
			CreatedBy: userID,
			CreatedAt: time.Now().UTC(),
		}

		if err := s.teamRepo.CreateTeam(ctx, exec, t); err != nil {
			return fmt.Errorf("create team: %w", err)
		}

		member := &model.TeamMember{
			TeamID: t.ID,
			UserID: userID,
			Role:   model.TeamRoleOwner,
		}

		if err := s.teamMemberRepo.AddMember(ctx, exec, member); err != nil {
			return fmt.Errorf("add owner as team member: %w", err)
		}

		team = t
		return nil
	})

	if err != nil {
		s.logger.Error("failed to execute create team transaction", zap.Error(err), zap.Int64("user_id", userID))
		return nil, err
	}

	return &dto.TeamResponse{
		ID:        team.ID,
		Name:      team.Name,
		CreatedBy: team.CreatedBy,
		CreatedAt: team.CreatedAt,
	}, nil
}

func (s *teamService) GetUserTeams(ctx context.Context, userID int64) ([]dto.TeamResponse, error) {
	teams, err := s.teamRepo.GetUserTeams(ctx, s.db, userID)
	if err != nil {
		s.logger.Error("failed to get user teams", zap.Error(err), zap.Int64("user_id", userID))
		return nil, fmt.Errorf("get user teams: %w", err)
	}

	result := make([]dto.TeamResponse, len(teams))
	for i, t := range teams {
		result[i] = dto.TeamResponse{
			ID:        t.ID,
			Name:      t.Name,
			CreatedBy: t.CreatedBy,
			CreatedAt: t.CreatedAt,
		}
	}

	return result, nil
}

func (s *teamService) InviteMember(ctx context.Context, actorID, teamID int64, req dto.InviteMemberRequest) error {
	role := model.TeamRole(req.Role)
	if role == model.TeamRoleOwner {
		return ErrCannotInviteOwner
	}

	return s.transactor.WithinTransaction(ctx, func(exec repository.DBEngine) error {
		actorRole, err := s.teamMemberRepo.GetMemberRole(ctx, exec, teamID, actorID)
		if err != nil {
			return err
		}
		if actorRole != model.TeamRoleOwner && actorRole != model.TeamRoleAdmin {
			return ErrAccessDenied
		}

		member := &model.TeamMember{
			TeamID: teamID,
			UserID: req.UserID,
			Role:   role,
		}
		return s.teamMemberRepo.AddMember(ctx, exec, member)
	})
}
