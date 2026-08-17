package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

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
