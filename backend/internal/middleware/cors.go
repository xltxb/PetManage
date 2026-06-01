package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS allows cross-origin requests from the admin frontend and mini-program.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-Store-Id,X-Trace-ID,Idempotency-Key")
		c.Header("Access-Control-Expose-Headers", "X-Trace-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
