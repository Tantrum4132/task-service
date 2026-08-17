package app

import (
	"github.com/Tantrum4132/task-service/internal/handler"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type HandlersContainer struct {
	Auth *handler.AuthHandler
	Team *handler.TeamHandler
	Task *handler.TaskHandler
}

func NewHandlersContainer(services *ServicesContainer, validate *validator.Validate, logger *zap.Logger) *HandlersContainer {
	return &HandlersContainer{
		Auth: handler.NewAuthHandler(services.Auth, validate, logger),
		Team: handler.NewTeamHandler(services.Team, validate, logger),
		Task: handler.NewTaskHandler(services.Task, services.Stats, validate, logger),
	}
}
