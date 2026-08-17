package app

import (
	"database/sql"

	"github.com/Tantrum4132/task-service/internal/config"
	"github.com/Tantrum4132/task-service/internal/repository"
	"github.com/Tantrum4132/task-service/internal/repository/mysql"
	"github.com/Tantrum4132/task-service/internal/repository/redis"

	redisclient "github.com/redis/go-redis/v9"
)

type Repositories struct {
	User        repository.UserRepository
	Team        repository.TeamRepository
	TeamMember  repository.TeamMemberRepository
	Task        repository.TaskRepository
	TaskHistory repository.TaskHistoryRepository
	TaskComment repository.TaskCommentRepository
	Stats       repository.StatsRepository
}

func NewRepositories(db *sql.DB, rdb redisclient.Cmdable, cfg *config.Config) *Repositories {
	var (
		userRepo        repository.UserRepository        = mysql.NewUserRepository(db)
		teamRepo        repository.TeamRepository        = mysql.NewTeamRepository(db)
		teamMemberRepo  repository.TeamMemberRepository  = mysql.NewTeamMemberRepository(db)
		taskRepo        repository.TaskRepository        = mysql.NewTaskRepository(db)
		taskHistoryRepo repository.TaskHistoryRepository = mysql.NewTaskHistoryRepository(db)
		taskCommentRepo repository.TaskCommentRepository = mysql.NewTaskCommentRepository(db)
		statsRepo       repository.StatsRepository       = mysql.NewStatsRepository(db)
	)

	if cfg != nil && cfg.Redis.Enabled && rdb != nil {
		if cfg.CacheServices.User.Enabled {
			userRepo = redis.NewUserCacheDecorator(userRepo, rdb, cfg.CacheServices.User.TTL)
		}

		if cfg.CacheServices.Task.Enabled {
			taskRepo = redis.NewTaskCacheDecorator(taskRepo, rdb, cfg.CacheServices.Task.TTL)
		}
	}

	return &Repositories{
		User:        userRepo,
		Team:        teamRepo,
		TeamMember:  teamMemberRepo,
		Task:        taskRepo,
		TaskHistory: taskHistoryRepo,
		TaskComment: taskCommentRepo,
		Stats:       statsRepo,
	}
}
