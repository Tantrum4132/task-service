package service_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/service"
	"github.com/Tantrum4132/task-service/mocks"
)

func TestStatsService_GetTeamStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	statsRepoMock := mocks.NewMockStatsRepository(ctrl)
	teamMemberRepoMock := mocks.NewMockTeamMemberRepository(ctrl)

	tests := []struct {
		name        string
		teamID      int64
		userID      int64
		setupMock   func(statsRepo *mocks.MockStatsRepository, memberRepo *mocks.MockTeamMemberRepository)
		expectedErr error
		checkResult func(t *testing.T, resp *dto.TaskStatsResponse, err error)
	}{
		{
			name:   "Success: Owner can get team stats",
			teamID: 1,
			userID: 10,
			setupMock: func(statsRepo *mocks.MockStatsRepository, memberRepo *mocks.MockTeamMemberRepository) {
				memberRepo.EXPECT().
					GetMemberRole(gomock.Any(), gomock.Any(), int64(1), int64(10)).
					Return(model.TeamRoleOwner, nil)

				statsRepo.EXPECT().
					GetTeamStats(gomock.Any(), int64(1)).
					Return(&model.TeamStats{
						Statuses: model.TaskStatusStats{
							Todo:       2,
							InProgress: 3,
							Done:       5,
						},
						TopAssignees: []model.TopAssignee{
							{UserID: 20, Name: "Alice", ClosedTasks: 5},
						},
						AvgCloseTimeHours:  12.5,
						TotalCommentsCount: 8,
					}, nil)
			},
			expectedErr: nil,
			checkResult: func(t *testing.T, resp *dto.TaskStatsResponse, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, int64(1), resp.TeamID)
				assert.Equal(t, 2, resp.TasksByStatus["todo"])
				assert.Equal(t, 3, resp.TasksByStatus["in_progress"])
				assert.Equal(t, 5, resp.TasksByStatus["done"])
				assert.Len(t, resp.TopAssignees, 1)
				assert.Equal(t, "Alice", resp.TopAssignees[0].UserName)
				assert.Equal(t, 12.5, resp.AvgTimeToCloseHours)
				assert.Equal(t, int64(8), resp.TotalComments)
			},
		},
		{
			name:   "Error: Permission denied for regular member",
			teamID: 1,
			userID: 15,
			setupMock: func(statsRepo *mocks.MockStatsRepository, memberRepo *mocks.MockTeamMemberRepository) {
				memberRepo.EXPECT().
					GetMemberRole(gomock.Any(), gomock.Any(), int64(1), int64(15)).
					Return(model.TeamRoleMember, nil)
			},
			expectedErr: service.ErrPermissionDenied,
			checkResult: func(t *testing.T, resp *dto.TaskStatsResponse, err error) {
				assert.Nil(t, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(statsRepoMock, teamMemberRepoMock)

			svc := service.NewStatsService(
				statsRepoMock,
				teamMemberRepoMock,
				zap.NewNop(),
			)

			resp, err := svc.GetTeamStats(context.Background(), tt.userID, tt.teamID)

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
