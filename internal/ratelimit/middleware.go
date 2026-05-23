package ratelimit

import (
	"net/http"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// OpenAPIMiddleware returns an HTTP middleware that applies rate limiting and
// circuit breaking per developer. It must be placed after OpenAPIAuth so that
// developer claims are available in the request context.
func (s *Service) OpenAPIMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := middleware.OpenDevClaimsFromContext(r.Context())
			if claims == nil {
				// No developer claims — shouldn't happen if placed after auth, but be safe.
				next.ServeHTTP(w, r)
				return
			}

			key := claims.DeveloperID

			// 1. Check circuit breaker first.
			cbState, cbAllowed := s.CircuitAllow(key)
			if !cbAllowed && cbState == StateOpen {
				apperrors.WriteError(w, r, apperrors.NewAppError(
					apperrors.CodeServiceUnavailable,
					"circuit breaker is open: service temporarily unavailable due to high error rate",
					nil,
				))
				return
			}

			// 2. Check rate limit.
			if !s.Allow(key) {
				apperrors.WriteError(w, r, apperrors.NewAppError(
					apperrors.CodeRateLimitExceeded,
					"rate limit exceeded: too many requests, please slow down",
					nil,
				))
				return
			}

			// 3. Execute handler with status code tracking for circuit breaker.
			sw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(sw, r)
			s.RecordResult(key, sw.statusCode)
		})
	}
}
