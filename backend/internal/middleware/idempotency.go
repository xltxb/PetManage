package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Idempotency caches responses for Idempotency-Key requests.
// Replays the cached response if the same key is seen within 24 hours.
func Idempotency(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		hash := hashIdempotencyKey(key + ":" + c.Request.URL.Path)

		ctx := context.Background()
		cached, err := rdb.Get(ctx, "idem:"+hash).Result()
		if err == nil {
			var resp cachedIdemResponse
			if json.Unmarshal([]byte(cached), &resp) == nil {
				c.Header("Content-Type", "application/json")
				c.String(resp.Status, resp.Body)
				c.Abort()
				return
			}
		}

		writer := &responseCapture{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			data, _ := json.Marshal(cachedIdemResponse{
				Status: c.Writer.Status(),
				Body:   writer.body(),
			})
			rdb.Set(ctx, "idem:"+hash, string(data), 24*time.Hour)
		}
	}
}

func hashIdempotencyKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}

type cachedIdemResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

type responseCapture struct {
	gin.ResponseWriter
	buf []byte
}

func (w *responseCapture) Write(data []byte) (int, error) {
	w.buf = append(w.buf, data...)
	return w.ResponseWriter.Write(data)
}

func (w *responseCapture) body() string {
	return string(w.buf)
}
