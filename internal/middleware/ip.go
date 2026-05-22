package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type ipContextKey string

const clientIPKey ipContextKey = "client_ip"

// GetClientIP extracts the client IP from the request, checking common proxy headers.
func GetClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		trimmed := strings.TrimSpace(parts[0])
		if trimmed != "" {
			return trimmed
		}
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ClientIPFromContext retrieves the client IP from the request context.
func ClientIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(clientIPKey).(string); ok {
		return ip
	}
	return ""
}

// WithClientIP returns an http.Handler that extracts the client IP from the request
// and stores it in the context.
func WithClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)
		ctx := context.WithValue(r.Context(), clientIPKey, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
