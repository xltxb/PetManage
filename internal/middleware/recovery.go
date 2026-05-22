package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/xltxb/PetManage/pkg/apperrors"
	"go.uber.org/zap"
)

// Recovery catches panics in downstream handlers and returns a 500 error
// without exposing internal details to the client.
func Recovery(lgr *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					lgr.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("stack", string(debug.Stack())),
					)
					appErr := &apperrors.AppError{
						Code:    apperrors.CodeInternalError,
						Message: "internal server error",
					}
					apperrors.WriteError(w, r, appErr)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
