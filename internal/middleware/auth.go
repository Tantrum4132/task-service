package middleware

import (
	"net/http"
	"strings"

	"github.com/Tantrum4132/task-service/internal/util"

	"github.com/gin-gonic/gin"
)

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

		c.Set(UserIDContextKey, claims.UserID)
		c.Next()
	}
}
