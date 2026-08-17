package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	auth "github.com/Tantrum4132/task-service/internal/util"

	"go.uber.org/zap"
)

var (
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
}

type authService struct {
	userRepo   repository.UserRepository
	db         repository.DBEngine
	jwtManager *auth.JWTManager
	logger     *zap.Logger
	jwtTTL     time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	db repository.DBEngine,
	jwtManager *auth.JWTManager,
	logger *zap.Logger,
	jwtTTL time.Duration,
) AuthService {
	return &authService{
		userRepo:   userRepo,
		db:         db,
		jwtManager: jwtManager,
		logger:     logger,
		jwtTTL:     jwtTTL,
	}
}

// Register осуществляет регистрацию нового пользователя с хешированием пароля.
func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*model.User, error) {
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Name:         req.Name,
		CreatedAt:    time.Now().UTC(),
	}

	// Полагаемся на уникальный constraint в БД, чтобы избежать состояния гонки
	if err := s.userRepo.Create(ctx, s.db, user); err != nil {
		if errors.Is(err, repository.ErrEmailAlreadyExists) {
			return nil, ErrUserAlreadyExists
		}
		s.logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// Login проверяет учетные данные пользователя и генерирует JWT токен.
func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, s.db, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		s.logger.Error("failed to fetch user by email", zap.Error(err), zap.String("email", req.Email))
		return nil, fmt.Errorf("find user: %w", err)
	}

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	token, err := s.jwtManager.GenerateJWT(user.ID, s.jwtTTL)
	if err != nil {
		s.logger.Error("failed to generate JWT token", zap.Error(err), zap.Int64("user_id", user.ID))
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &dto.AuthResponse{
		AccessToken: token,
		ExpiresIn:   int64(s.jwtTTL.Seconds()),
	}, nil
}
