package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfTokenLen   = 32
)

// csrfSkipPaths lists paths exempt from CSRF validation.
var csrfSkipPaths = []string{
	"/api/v1/auth/login",
	"/api/v1/auth/refresh",
	"/api/v1/auth/change-password",
	"/api/v1/csrf-token",
	"/api/v1/merchant/auth/login",
	"/api/v1/open/",
	"/api/open/v1/",
	"/health",
}

func csrfShouldSkip(path string) bool {
	for _, p := range csrfSkipPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CSRF returns a middleware that provides CSRF protection using the
// double-submit cookie pattern.
func CSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if csrfShouldSkip(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// For state-changing methods, validate the CSRF token.
			if r.Method == http.MethodPost || r.Method == http.MethodPut ||
				r.Method == http.MethodDelete || r.Method == http.MethodPatch {
				cookie, err := r.Cookie(csrfCookieName)
				if err != nil {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeCSRFMissing,
						Message: "CSRF token is missing",
					})
					return
				}

				headerToken := r.Header.Get(csrfHeaderName)
				if headerToken == "" {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeCSRFMissing,
						Message: "CSRF token header is missing",
					})
					return
				}

				if cookie.Value != headerToken {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeCSRFInvalid,
						Message: "CSRF token does not match",
					})
					return
				}
			}

			// Ensure a CSRF token cookie is set for browsers to use.
			if _, err := r.Cookie(csrfCookieName); err != nil {
				token, genErr := GenerateCSRFToken()
				if genErr != nil {
					next.ServeHTTP(w, r)
					return
				}
				http.SetCookie(w, &http.Cookie{
					Name:     csrfCookieName,
					Value:    token,
					Path:     "/",
					SameSite: http.SameSiteStrictMode,
					Secure:   false, // Allow HTTP for dev
					HttpOnly: false, // JS must be able to read it
				})
			}

			next.ServeHTTP(w, r)
		})
	}
}
