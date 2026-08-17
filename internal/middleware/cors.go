package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
