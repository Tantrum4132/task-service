package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	"github.com/Tantrum4132/task-service/internal/service"
	"github.com/Tantrum4132/task-service/internal/util"
	"github.com/Tantrum4132/task-service/mocks"
)

func TestAuthService_Login(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userRepoMock := mocks.NewMockUserRepository(ctrl)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.MinCost)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		request     dto.LoginRequest
		setupMock   func(userRepo *mocks.MockUserRepository)
		expectedErr error
		checkResult func(t *testing.T, resp *dto.AuthResponse, err error)
	}{
		{
			name: "Success login",
			request: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "correct_password",
			},
			setupMock: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().
					FindByEmail(gomock.Any(), gomock.Any(), "test@example.com").
					Return(&model.User{
						ID:           1,
						Email:        "test@example.com",
						PasswordHash: string(hashedPassword),
					}, nil)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *dto.AuthResponse, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.AccessToken)
			},
		},
		{
			name: "Error: User not found (returns invalid credentials for security)",
			request: dto.LoginRequest{
				Email:    "unknown@example.com",
				Password: "some_password",
			},
			setupMock: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().
					FindByEmail(gomock.Any(), gomock.Any(), "unknown@example.com").
					Return(nil, repository.ErrUserNotFound)
			},
			expectedErr: service.ErrInvalidCredentials,
			checkResult: func(t *testing.T, resp *dto.AuthResponse, err error) {
				assert.Nil(t, resp)
			},
		},
		{
			name: "Error: Invalid password",
			request: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "wrong_password",
			},
			setupMock: func(userRepo *mocks.MockUserRepository) {
				userRepo.EXPECT().
					FindByEmail(gomock.Any(), gomock.Any(), "test@example.com").
					Return(&model.User{
						ID:           1,
						Email:        "test@example.com",
						PasswordHash: string(hashedPassword),
					}, nil)
			},
			expectedErr: service.ErrInvalidCredentials,
			checkResult: func(t *testing.T, resp *dto.AuthResponse, err error) {
				assert.Nil(t, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(userRepoMock)

			jwtManager := util.NewJWTManager("your-secret-key")

			svc := service.NewAuthService(
				userRepoMock,
				nil,
				jwtManager,
				zap.NewNop(),
				24*time.Hour,
			)

			resp, err := svc.Login(context.Background(), tt.request)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, resp, err)
			}
		})
	}
}
