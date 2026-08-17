package handler

import (
	"errors"
	"net/http"

	"github.com/Tantrum4132/task-service/internal/dto"
	"github.com/Tantrum4132/task-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type AuthHandler struct {
	authService service.AuthService
	validate    *validator.Validate
	logger      *zap.Logger
}

func NewAuthHandler(authService service.AuthService, validate *validator.Validate, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validate:    validate,
		logger:      logger,
	}
}

// Register godoc
//
//	@Summary		Регистрация нового пользователя
//	@Description	Создает нового пользователя в системе с хешированием пароля.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.RegisterRequest	true	"Данные для регистрации"
//	@Success		201		{object}	map[string]any		"Пользователь успешно зарегистрирован"
//	@Failure		400		{object}	gin.H				"Невалидное тело запроса или ошибки валидации"
//	@Failure		409		{object}	gin.H				"Пользователь с таким email уже существует"
//	@Failure		500		{object}	gin.H				"Внутренняя ошибка сервера"
//	@Router			/api/v1/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to register user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"name":       user.Name,
		"created_at": user.CreatedAt,
	})
}

// Login godoc
//
//	@Summary		Аутентификация пользователя
//	@Description	Проверяет учетные данные пользователя и возвращает JWT-токен.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.LoginRequest	true	"Учетные данные пользователя"
//	@Success		200		{object}	dto.AuthResponse	"Успешная аутентификация"
//	@Failure		400		{object}	gin.H				"Невалидное тело запроса или ошибки валидации"
//	@Failure		401		{object}	gin.H				"Неверный email или пароль"
//	@Failure		500		{object}	gin.H				"Внутренняя ошибка сервера"
//	@Router			/api/v1/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.validate.StructCtx(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("failed to login user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
