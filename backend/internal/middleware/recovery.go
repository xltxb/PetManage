package middleware

import (
	"github.com/gin-gonic/gin"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/response"
)

// Recovery handles panics and returns a 500 error.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				traceID, _ := c.Get("trace_id")
				Logger.Error("panic recovered",
					"trace_id", traceID,
					"panic", r,
				)
				response.Error(c, apperr.New(5000, "服务器内部错误"))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// ErrorHandler catches apperr.AppError from handlers.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if ae, ok := err.(*apperr.AppError); ok {
				response.Error(c, ae)
				return
			}
			response.Error(c, apperr.Internal(err))
		}
	}
}
