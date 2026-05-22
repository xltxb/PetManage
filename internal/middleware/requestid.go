package middleware

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// RequestID generates a unique request ID for each request.
// It reads X-Request-ID from the incoming header, or generates a new UUIDv4.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuidv4()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := apperrors.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func uuidv4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%016x", b)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
