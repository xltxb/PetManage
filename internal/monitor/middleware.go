package monitor

import (
	"net/http"
	"time"

	"github.com/xltxb/PetManage/internal/middleware"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware returns an HTTP middleware that logs API requests asynchronously.
// Must be placed AFTER OpenAPIAuth so developer_id is available in context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rec, r)

		durationMs := int(time.Since(start).Milliseconds())
		endpoint := r.URL.Path
		method := r.Method

		var devID *int64
		if claims := middleware.OpenDevClaimsFromContext(r.Context()); claims != nil {
			devID = &claims.DeveloperID
		}

		ip := middleware.GetClientIP(r)

		s.LogAsync(LogEntry{
			DeveloperID: devID,
			Endpoint:    endpoint,
			Method:      method,
			StatusCode:  rec.statusCode,
			DurationMs:  durationMs,
			IPAddress:   ip,
		})
	})
}
