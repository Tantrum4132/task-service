package app

import (
	"basis/internal/repository/mysql"
	"basis/internal/service"
	"basis/internal/util"
)

type ServicesContainer struct {
	Auth  service.AuthService
	Team  service.TeamService
	Task  service.TaskService
	Stats service.StatsService
}

func NewServicesContainer(c *Container) *ServicesContainer {
	// 1. Вспомогательные утилиты и транзактор
	jwtManager := util.NewJWTManager(c.Config.JWT.Secret)
	transactor := mysql.NewTransactor(c.DB)

	// 2. Определение инвалидатора кеша задач из репозитория
	var cacheInvalidator service.TaskCacheInvalidator
	if c.Config != nil && c.Config.Redis.Enabled && c.Redis != nil {
		if c.Config.CacheServices.Task.Enabled {
			// Репозиторий Task уже обернут в redis.TaskCacheDecorator внутри NewRepositories
			if invalidator, ok := c.Repositories.Task.(service.TaskCacheInvalidator); ok {
				cacheInvalidator = invalidator
			}
		}
	}

	// 3. Инициализация сервисов
	authSvc := service.NewAuthService(
		c.Repositories.User,
		c.DB,
		jwtManager,
		c.Logger,
		c.Config.JWT.Lifespan,
	)

	teamSvc := service.NewTeamService(
		c.Repositories.Team,
		c.Repositories.TeamMember,
		c.Repositories.User,
		transactor,
		c.DB,
		c.Logger,
	)

	taskSvc := service.NewTaskService(
		c.Repositories.Task,
		c.Repositories.TeamMember,
		c.Repositories.TaskHistory,
		c.Repositories.TaskComment,
		transactor,
		cacheInvalidator,
		c.Logger,
	)

	statsSvc := service.NewStatsService(
		c.Repositories.Stats,
		c.Repositories.TeamMember,
		c.Logger,
	)

	return &ServicesContainer{
		Auth:  authSvc,
		Team:  teamSvc,
		Task:  taskSvc,
		Stats: statsSvc,
	}
}
