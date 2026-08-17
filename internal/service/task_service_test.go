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

func TestTaskService_UpdateTask_AccessControl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name      string
		actorID   int64
		taskID    int64
		setupMock func(
			taskRepo *mocks.MockTaskRepository,
			memberRepo *mocks.MockTeamMemberRepository,
			historyRepo *mocks.MockTaskHistoryRepository,
			transactor *mocks.MockTransactor,
			cacheInvalidator *mocks.MockTaskCacheInvalidator,
		)
		expectedErr error
	}{
		{
			name:    "Error: Regular member cannot update task they don't own/assigned",
			actorID: 20,
			taskID:  100,
			setupMock: func(
				taskRepo *mocks.MockTaskRepository,
				memberRepo *mocks.MockTeamMemberRepository,
				historyRepo *mocks.MockTaskHistoryRepository,
				transactor *mocks.MockTransactor,
				cacheInvalidator *mocks.MockTaskCacheInvalidator,
			) {
				taskRepo.EXPECT().
					GetTaskByID(gomock.Any(), gomock.Any(), int64(100)).
					Return(&model.Task{
						ID:         100,
						TeamID:     1,
						Version:    1,
						CreatedBy:  30,
						AssigneeID: int64Ptr(40),
						Status:     model.TaskStatusTodo,
					}, nil)

				memberRepo.EXPECT().
					GetMemberRole(gomock.Any(), gomock.Any(), int64(1), int64(20)).
					Return(model.TeamRoleMember, nil)

				// Кэш здесь инвалидироваться не должен, ожиданий нет.
			},
			expectedErr: service.ErrPermissionDenied,
		},
		{
			name:    "Success: Team Owner can update any task",
			actorID: 10,
			taskID:  100,
			setupMock: func(
				taskRepo *mocks.MockTaskRepository,
				memberRepo *mocks.MockTeamMemberRepository,
				historyRepo *mocks.MockTaskHistoryRepository,
				transactor *mocks.MockTransactor,
				cacheInvalidator *mocks.MockTaskCacheInvalidator,
			) {
				taskRepo.EXPECT().
					GetTaskByID(gomock.Any(), gomock.Any(), int64(100)).
					Return(&model.Task{
						ID:         100,
						TeamID:     1,
						Version:    1,
						CreatedBy:  30,
						AssigneeID: int64Ptr(40),
						Status:     model.TaskStatusTodo,
					}, nil)

				memberRepo.EXPECT().
					GetMemberRole(gomock.Any(), gomock.Any(), int64(1), int64(10)).
					Return(model.TeamRoleOwner, nil)

				transactor.EXPECT().
					WithinTransaction(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(exec repository.DBEngine) error) error {
						return fn(nil)
					})

				taskRepo.EXPECT().
					UpdateTask(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				historyRepo.EXPECT().
					CreateTaskHistory(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// Инвалидация кэша происходит только при успешном обновлении
				cacheInvalidator.EXPECT().
					InvalidateTeamCache(gomock.Any(), int64(1)).
					Return(nil)
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskRepoMock := mocks.NewMockTaskRepository(ctrl)
			teamMemberRepoMock := mocks.NewMockTeamMemberRepository(ctrl)
			taskHistoryRepoMock := mocks.NewMockTaskHistoryRepository(ctrl)
			transactorMock := mocks.NewMockTransactor(ctrl)
			cacheInvalidatorMock := mocks.NewMockTaskCacheInvalidator(ctrl)

			// Передаем мок инвалидатора внутрь тест-кейса
			tt.setupMock(
				taskRepoMock,
				teamMemberRepoMock,
				taskHistoryRepoMock,
				transactorMock,
				cacheInvalidatorMock,
			)

			svc := service.NewTaskService(
				taskRepoMock,
				teamMemberRepoMock,
				taskHistoryRepoMock,
				nil, // taskCommentRepo
				transactorMock,
				cacheInvalidatorMock,
				zap.NewNop(),
			)

			req := dto.UpdateTaskRequest{
				Version: 1,
				Title:   stringPtr("Updated Title"),
			}
			_, err := svc.UpdateTask(context.Background(), tt.actorID, tt.taskID, req)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func stringPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64    { return &i }
