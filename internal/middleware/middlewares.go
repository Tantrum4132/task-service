package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/Tantrum4132/task-service/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// UserIDContextKey — ключ для сохранения ID пользователя в контексте Gin
	UserIDContextKey    = "userID"
	RequestIDHeader     = "X-Request-ID"
	RequestIDContextKey = "requestID"
)

// Auth проверяет валидность JWT-токена из заголовка Authorization.
func Auth(jwtManager *util.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := jwtManager.ParseJWT(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Записываем userID в контекст Gin
		c.Set(UserIDContextKey, claims.UserID)
		c.Next()
	}
}

// CORS возвращает middleware для обработки Cross-Origin Resource Sharing.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	originsMap := make(map[string]bool, len(allowedOrigins))
	allowAll := false

	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
		originsMap[o] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
				c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
				c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
			} else if originsMap[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
				c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.New().String()
		}

		c.Set(RequestIDContextKey, reqID)
		c.Header(RequestIDHeader, reqID)
		c.Next()
	}
}

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		cost := time.Since(start)
		reqID, _ := c.Get(RequestIDContextKey)

		logger.Info("HTTP Request",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Duration("latency", cost),
			zap.Any("request_id", reqID),
		)
	}
}

// RateLimiter ограничивает количество запросов с одного IP
func RateLimiter(rdb redis.Cmdable, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := "rate_limit:" + c.FullPath() + ":" + ip

		ctx := c.Request.Context()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next() // В случае сбоя Redis не блокируем пользователей (fail-open)
			return
		}

		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered in HTTP handler",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
