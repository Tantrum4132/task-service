package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

const UserIDContextKey = "userID"

type TeamHandler struct {
	teamService service.TeamService
	validate    *validator.Validate
	logger      *zap.Logger
}

func NewTeamHandler(teamService service.TeamService, validate *validator.Validate, logger *zap.Logger) *TeamHandler {
	return &TeamHandler{
		teamService: teamService,
		validate:    validate,
		logger:      logger,
	}
}

// CreateTeam godoc
//
//	@Summary		Создание новой команды
//	@Description	Создает новую команду и автоматически делает текущего пользователя ее владельцем (owner).
//	@Tags			teams
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		dto.CreateTeamRequest	true	"Данные для создания команды"
//	@Success		201		{object}	dto.TeamResponse		"Команда успешно создана"
//	@Failure		400		{object}	gin.H					"Невалидное тело запроса или ошибки валидации"
//	@Failure		401		{object}	gin.H					"Пользователь не авторизован"
//	@Failure		500		{object}	gin.H					"Внутренняя ошибка сервера"
//	@Router			/api/v1/teams [post]
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var req dto.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := h.teamService.CreateTeam(c.Request.Context(), userID, req)
	if err != nil {
		h.logger.Error("failed to create team", zap.Error(err), zap.Int64("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, team)
}

// GetUserTeams godoc
//
//	@Summary		Получение списка команд пользователя
//	@Description	Возвращает список всех команд, в которых состоит аутентифицированный пользователь.
//	@Tags			teams
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		dto.TeamResponse	"Список команд пользователя"
//	@Failure		401	{object}	gin.H				"Пользователь не авторизован"
//	@Failure		500	{object}	gin.H				"Внутренняя ошибка сервера"
//	@Router			/api/v1/teams [get]
func (h *TeamHandler) GetUserTeams(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	teams, err := h.teamService.GetUserTeams(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user teams", zap.Error(err), zap.Int64("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// InviteMember godoc
//
//	@Summary		Приглашение нового участника в команду
//	@Description	Добавляет пользователя в команду. Выполнять операцию могут только участники с ролью owner или admin. Выдать роль owner через приглашение нельзя.
//	@Tags			teams
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	int64					true	"ID команды"
//	@Param			request	body	dto.InviteMemberRequest	true	"Данные приглашения (user_id и роль)"
//	@Success		204		"Участник успешно добавлен в команду"
//	@Failure		400		{object}	gin.H	"Невалидный ID команды, попытка выдать роль owner или пользователь уже в команде"
//	@Failure		401		{object}	gin.H	"Пользователь не авторизован"
//	@Failure		403		{object}	gin.H	"Недостаточно прав (пользователь не является owner или admin)"
//	@Failure		404		{object}	gin.H	"Команда или приглашаемый пользователь не найдены"
//	@Failure		500		{object}	gin.H	"Внутренняя ошибка сервера"
//	@Router			/api/v1/teams/{id}/invite [post]
func (h *TeamHandler) InviteMember(c *gin.Context) {
	actorID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req dto.InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.teamService.InviteMember(c.Request.Context(), actorID, teamID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCannotInviteOwner), errors.Is(err, service.ErrUserAlreadyInTeam):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrUserNotFound), errors.Is(err, service.ErrTeamNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			h.logger.Error("failed to invite member to team",
				zap.Error(err),
				zap.Int64("actor_id", actorID),
				zap.Int64("team_id", teamID),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func getUserIDFromContext(c *gin.Context) (int64, bool) {
	val, exists := c.Get(UserIDContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}

	userID, ok := val.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}

	return userID, true
}
