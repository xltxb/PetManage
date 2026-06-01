package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// RateLimiter limits requests per user per minute using Redis.
func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "rate:" + c.ClientIP()
		if uid, exists := c.Get("user_id"); exists {
			key = "rate:u" + formatInt(uid)
		}

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next() // fail open
			return
		}

		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			response.Error(c, apperr.New(429, "请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}

		c.Next()
	}
}

func formatInt(v interface{}) string {
	switch n := v.(type) {
	case int64:
		return itoa64(n)
	case float64:
		return itoa64(int64(n))
	default:
		return "0"
	}
}

func itoa64(i int64) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
