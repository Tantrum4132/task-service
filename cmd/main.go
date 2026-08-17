package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Tantrum4132/task-service/internal/app"
	"github.com/Tantrum4132/task-service/internal/middleware"
	"github.com/Tantrum4132/task-service/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/Tantrum4132/task-service/docs"
)

// @title           Task Management System API
// @version         1.0
// @description     REST API сервиса управления задачами и командной аналитики.
// @termsOfService  http://swagger.io/terms/

// @contact.name    API Support
// @contact.email   support@basis.local

// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Введите токен в формате: Bearer <JWT_TOKEN>
func main() {
	configPath := getConfigPath()
	container, err := app.NewContainer(configPath)
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}
	defer container.Close()

	logger := container.Logger

	// 2. Инициализация валидатора, сервисов и обработчиков
	validate := validator.New()
	services := app.NewServicesContainer(container)
	handlers := app.NewHandlersContainer(services, validate, logger)

	// 3. Установка режима работы Gin
	if container.Config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// 4. Подключение глобальных Middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.Logger(logger))

	if len(container.Config.CORS.AllowedOrigins) > 0 {
		router.Use(middleware.CORS(container.Config.CORS.AllowedOrigins))
	}

	// 5. Подключение маршрута для Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 6. Настройка JWT и Auth Middleware
	jwtManager := util.NewJWTManager(container.Config.JWT.Secret)
	authMiddleware := middleware.Auth(jwtManager)

	// 7. Регистрация API маршрутов v1
	v1 := router.Group("/api/v1")
	{
		// 🔓 Публичная группа (Аутентификация и Регистрация) с ограничением запросов (Rate Limiting)
		authGroup := v1.Group("")
		if container.Redis != nil {
			authGroup.Use(middleware.RateLimiter(container.Redis, 10, time.Minute))
		}
		{
			authGroup.POST("/register", handlers.Auth.Register)
			authGroup.POST("/login", handlers.Auth.Login)
		}

		// 🔒 Защищенная группа (Требуется JWT-токен)
		protectedGroup := v1.Group("")
		protectedGroup.Use(authMiddleware)
		{
			// Маршруты управления командами
			teamsGroup := protectedGroup.Group("/teams")
			{
				teamsGroup.POST("", handlers.Team.CreateTeam)
				teamsGroup.GET("", handlers.Team.GetUserTeams)
				teamsGroup.POST("/:id/invite", handlers.Team.InviteMember)
			}

			// Маршруты задач, комментариев, истории и аналитики (/tasks, /teams/:team_id/stats)
			handlers.Task.RegisterRoutes(protectedGroup)
		}
	}

	// 8. Конфигурация HTTP-сервера
	srv := &http.Server{
		Addr:         container.Config.ServerAddr(),
		Handler:      router,
		ReadTimeout:  container.Config.Server.ReadTimeout,
		WriteTimeout: container.Config.Server.WriteTimeout,
	}

	// Запуск HTTP-сервера в отдельной горутине
	go func() {
		logger.Info("starting HTTP server", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTP server failed to start", zap.Error(err))
		}
	}()

	// 9. Graceful Shutdown (Перехват сигналов завершения ОС)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down HTTP server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), container.Config.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	} else {
		logger.Info("server exited cleanly")
	}
}

// getConfigPath возвращает путь к файлу конфигурации из переменной окружения или по умолчанию.
func getConfigPath() string {
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		return envPath
	}
	return "config.yaml"
}
