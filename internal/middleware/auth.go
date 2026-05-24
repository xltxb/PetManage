package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xltxb/PetManage/internal/auth"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

type contextKey string

const userClaimsKey contextKey = "user_claims"

// UserClaimsFromContext retrieves JWT claims from the request context.
func UserClaimsFromContext(ctx context.Context) *auth.Claims {
	if claims, ok := ctx.Value(userClaimsKey).(*auth.Claims); ok {
		return claims
	}
	return nil
}

// Auth returns middleware that validates JWT tokens.
func Auth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeUnauthorized,
					Message: "missing authorization header",
				})
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeUnauthorized,
					Message: "invalid authorization format, expected: Bearer <token>",
				})
				return
			}

			claims, err := jwtManager.ValidateAccessToken(parts[1])
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					apperrors.WriteError(w, r, &apperrors.AppError{
						Code:    apperrors.CodeTokenExpired,
						Message: "token has expired",
					})
					return
				}
				apperrors.WriteError(w, r, &apperrors.AppError{
					Code:    apperrors.CodeUnauthorized,
					Message: "invalid or malformed token",
				})
				return
			}

			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
