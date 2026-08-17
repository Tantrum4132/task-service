package service_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"
	"github.com/Tantrum4132/task-service/internal/service"
	"github.com/Tantrum4132/task-service/mocks"
)

func TestTeamService_InviteMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name      string
		teamID    int64
		actorID   int64
		request   dto.InviteMemberRequest
		setupMock func(
			memberRepo *mocks.MockTeamMemberRepository,
			transactor *mocks.MockTransactor,
		)
		expectedErr error
	}{
		{
			name:    "Success: Owner can invite member",
			teamID:  1,
			actorID: 10,
			request: dto.InviteMemberRequest{
				UserID: 20,
				Role:   string(model.TeamRoleMember),
			},
			setupMock: func(
				memberRepo *mocks.MockTeamMemberRepository,
				transactor *mocks.MockTransactor,
			) {
				transactor.EXPECT().
					WithinTransaction(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(exec repository.DBEngine) error) error {
						return fn(nil)
					})

				memberRepo.EXPECT().
					GetMemberRole(gomock.Any(), gomock.Any(), int64(1), int64(10)).
					Return(model.TeamRoleOwner, nil)

				memberRepo.EXPECT().
					AddMember(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&model.TeamMember{})).
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:    "Error: Cannot invite owner role",
			teamID:  1,
			actorID: 10,
			request: dto.InviteMemberRequest{
				UserID: 20,
				Role:   string(model.TeamRoleOwner),
			},
			setupMock: func(
				memberRepo *mocks.MockTeamMemberRepository,
				transactor *mocks.MockTransactor,
			) {
				// Транзакция и проверки вызваны не будут, так как проверка роли owner идет до нее
			},
			expectedErr: service.ErrCannotInviteOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teamRepoMock := mocks.NewMockTeamRepository(ctrl)
			teamMemberRepoMock := mocks.NewMockTeamMemberRepository(ctrl)
			userRepoMock := mocks.NewMockUserRepository(ctrl)
			transactorMock := mocks.NewMockTransactor(ctrl)

			tt.setupMock(teamMemberRepoMock, transactorMock)

			svc := service.NewTeamService(
				teamRepoMock,
				teamMemberRepoMock,
				userRepoMock,
				transactorMock,
				nil,
				zap.NewNop(),
			)

			err := svc.InviteMember(context.Background(), tt.actorID, tt.teamID, tt.request)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
