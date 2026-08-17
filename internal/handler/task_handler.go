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

type TaskHandler struct {
	taskService  service.TaskService
	statsService service.StatsService
	validate     *validator.Validate
	logger       *zap.Logger
}

func NewTaskHandler(
	taskService service.TaskService,
	statsService service.StatsService,
	validate *validator.Validate,
	logger *zap.Logger,
) *TaskHandler {
	return &TaskHandler{
		taskService:  taskService,
		statsService: statsService,
		validate:     validate,
		logger:       logger,
	}
}

func (h *TaskHandler) RegisterRoutes(r *gin.RouterGroup) {
	tasks := r.Group("/tasks")
	{
		tasks.POST("", h.CreateTask)
		tasks.GET("", h.ListTasks)
		tasks.GET("/:id", h.GetTaskByID)
		tasks.PUT("/:id", h.UpdateTask)
		tasks.GET("/:id/history", h.GetTaskHistory)
		tasks.POST("/:id/comments", h.CreateComment)
		tasks.GET("/:id/comments", h.ListComments)
	}

	teams := r.Group("/teams")
	{
		teams.GET("/:team_id/stats", h.GetTeamStats)
	}
}

// CreateTask godoc
//
//	@Summary		Создание новой задачи
//	@Description	Создает задачу в указанной команде с привязкой к исполнителю при необходимости.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		dto.CreateTaskRequest	true	"Параметры создания задачи"
//	@Success		201		{object}	dto.TaskResponse		"Задача успешно создана"
//	@Failure		400		{object}	gin.H					"Невалидный запрос или исполнитель не состоит в команде"
//	@Failure		401		{object}	gin.H					"Пользователь не авторизован"
//	@Failure		403		{object}	gin.H					"Пользователь не состоит в команде задачи"
//	@Failure		422		{object}	gin.H					"Ошибка валидации полей"
//	@Failure		500		{object}	gin.H					"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks [post]
func (h *TaskHandler) CreateTask(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.taskService.CreateTask(c.Request.Context(), userID, req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetTaskByID godoc
//
//	@Summary		Получение задачи по ID
//	@Description	Возвращает подробную информацию о задаче по ее идентификатору.
//	@Tags			tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int64				true	"ID задачи"
//	@Success		200	{object}	dto.TaskResponse	"Информация о задаче"
//	@Failure		400	{object}	gin.H				"Невалидный ID задачи"
//	@Failure		401	{object}	gin.H				"Пользователь не авторизован"
//	@Failure		403	{object}	gin.H				"Нет доступа к задаче"
//	@Failure		404	{object}	gin.H				"Задача не найдена"
//	@Failure		500	{object}	gin.H				"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks/{id} [get]
func (h *TaskHandler) GetTaskByID(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	resp, err := h.taskService.GetTaskByID(c.Request.Context(), userID, taskID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListTasks godoc
//
//	@Summary		Получение списка задач команды
//	@Description	Возвращает список задач указанной команды с поддержкой фильтрации по статусу, исполнителю и пагинации.
//	@Tags			tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			team_id		query		int64				true	"ID команды"
//	@Param			status		query		string				false	"Фильтр по статусу (todo, in_progress, done)"
//	@Param			assignee_id	query		int64				false	"Фильтр по ID исполнителя"
//	@Param			limit		query		int					false	"Лимит записей (по умолчанию 20)"
//	@Param			offset		query		int					false	"Смещение (по умолчанию 0)"
//	@Success		200			{array}		dto.TaskResponse	"Список задач"
//	@Failure		400			{object}	gin.H				"Некорректный team_id"
//	@Failure		401			{object}	gin.H				"Пользователь не авторизован"
//	@Failure		403			{object}	gin.H				"Пользователь не входит в состав команды"
//	@Failure		422			{object}	gin.H				"Ошибка валидации параметров запроса"
//	@Failure		500			{object}	gin.H				"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks [get]
func (h *TaskHandler) ListTasks(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	teamIDStr := c.Query("team_id")
	if teamIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "team_id is required"})
		return
	}

	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team_id"})
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	query := dto.TaskFilterQuery{
		TeamID: teamID,
		Limit:  limit,
		Offset: offset,
	}

	if status := c.Query("status"); status != "" {
		query.Status = &status
	}

	if assigneeStr := c.Query("assignee_id"); assigneeStr != "" {
		if assigneeID, err := strconv.ParseInt(assigneeStr, 10, 64); err == nil {
			query.AssigneeID = &assigneeID
		}
	}

	if err := h.validate.StructCtx(c.Request.Context(), query); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	tasks, err := h.taskService.ListTasks(c.Request.Context(), userID, query)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// UpdateTask godoc
//
//	@Summary		Обновление задачи
//	@Description	Обновляет данные задачи с учетом прав роли пользователя и оптимистичной блокировки по полю version.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int64					true	"ID задачи"
//	@Param			request	body		dto.UpdateTaskRequest	true	"Данные для обновления задачи"
//	@Success		200		{object}	dto.TaskResponse		"Задача успешно обновлена"
//	@Failure		400		{object}	gin.H					"Невалидный ID задачи или неверный исполнитель"
//	@Failure		401		{object}	gin.H					"Пользователь не авторизован"
//	@Failure		403		{object}	gin.H					"Недостаточно прав для редактирования"
//	@Failure		404		{object}	gin.H					"Задача не найдена"
//	@Failure		409		{object}	gin.H					"Конфликт версий (оптимистичная блокировка)"
//	@Failure		422		{object}	gin.H					"Ошибка валидации полей"
//	@Failure		500		{object}	gin.H					"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks/{id} [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.taskService.UpdateTask(c.Request.Context(), userID, taskID, req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetTaskHistory godoc
//
//	@Summary		История изменений задачи
//	@Description	Возвращает хронологический журнал изменений полей задачи.
//	@Tags			tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int64						true	"ID задачи"
//	@Param			limit	query		int							false	"Лимит записей (по умолчанию 20)"
//	@Param			offset	query		int							false	"Смещение (по умолчанию 0)"
//	@Success		200		{array}		dto.TaskHistoryResponse		"История изменений задачи"
//	@Failure		400		{object}	gin.H						"Невалидный ID задачи"
//	@Failure		401		{object}	gin.H						"Пользователь не авторизован"
//	@Failure		403		{object}	gin.H						"Нет доступа к просмотру истории"
//	@Failure		404		{object}	gin.H						"Задача не найдена"
//	@Failure		500		{object}	gin.H						"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks/{id}/history [get]
func (h *TaskHandler) GetTaskHistory(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	limit := 20
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	history, err := h.taskService.GetTaskHistory(c.Request.Context(), userID, taskID, limit, offset)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, history)
}

// CreateComment godoc
//
//	@Summary		Добавление комментария к задаче
//	@Description	Создает новый комментарий к существующей задаче от имени текущего пользователя.
//	@Tags			tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int64					true	"ID задачи"
//	@Param			request	body		dto.CreateCommentRequest	true	"Текст комментария"
//	@Success		201		{object}	dto.TaskCommentResponse	"Комментарий успешно создан"
//	@Failure		400		{object}	gin.H					"Невалидный ID задачи или пустое тело"
//	@Failure		401		{object}	gin.H					"Пользователь не авторизован"
//	@Failure		403		{object}	gin.H					"Пользователь не состоит в команде задачи"
//	@Failure		404		{object}	gin.H					"Задача не найдена"
//	@Failure		422		{object}	gin.H					"Ошибка валидации комментария"
//	@Failure		500		{object}	gin.H					"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks/{id}/comments [post]
func (h *TaskHandler) CreateComment(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.taskService.CreateComment(c.Request.Context(), userID, taskID, req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, comment)
}

// ListComments godoc
//
//	@Summary		Получение списка комментариев задачи
//	@Description	Возвращает все комментарии, оставленные к задаче.
//	@Tags			tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int64						true	"ID задачи"
//	@Success		200	{array}		dto.TaskCommentResponse		"Список комментариев"
//	@Failure		400	{object}	gin.H						"Невалидный ID задачи"
//	@Failure		401	{object}	gin.H						"Пользователь не авторизован"
//	@Failure		403	{object}	gin.H						"Пользователь не состоит в команде задачи"
//	@Failure		404	{object}	gin.H						"Задача не найдена"
//	@Failure		500	{object}	gin.H						"Внутренняя ошибка сервера"
//	@Router			/api/v1/tasks/{id}/comments [get]
func (h *TaskHandler) ListComments(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	comments, err := h.taskService.ListComments(c.Request.Context(), userID, taskID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, comments)
}

// GetTeamStats godoc
//
//	@Summary		Получение аналитического отчета по команде
//	@Description	Возвращает сводные метрики по задачам, комментариям и ТОП-3 исполнителям. Доступно только ролям owner и admin.
//	@Tags			teams
//	@Produce		json
//	@Security		BearerAuth
//	@Param			team_id	path		int64					true	"ID команды"
//	@Success		200		{object}	dto.TaskStatsResponse	"Аналитический отчет"
//	@Failure		400		{object}	gin.H					"Невалидный ID команды"
//	@Failure		401		{object}	gin.H					"Пользователь не авторизован"
//	@Failure		403		{object}	gin.H					"Недостаточно прав (не owner или admin)"
//	@Failure		404		{object}	gin.H					"Команда не найдена"
//	@Failure		500		{object}	gin.H					"Внутренняя ошибка сервера"
//	@Router			/api/v1/teams/{team_id}/stats [get]
func (h *TaskHandler) GetTeamStats(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	teamID, err := strconv.ParseInt(c.Param("team_id"), 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	stats, err := h.statsService.GetTeamStats(c.Request.Context(), userID, teamID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *TaskHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrTaskNotFound),
		errors.Is(err, service.ErrTeamNotFound),
		errors.Is(err, service.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidAssignee):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		h.logger.Error("internal server error during request processing", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
