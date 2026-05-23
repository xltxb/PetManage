package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xltxb/PetManage/internal/openplatform"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

type openAPIContextKey string

const openDevClaimsKey openAPIContextKey = "open_dev_claims"

// OpenDevClaimsFromContext retrieves open platform claims from the request context.
func OpenDevClaimsFromContext(ctx context.Context) *openplatform.OpenPlatformClaims {
	if claims, ok := ctx.Value(openDevClaimsKey).(*openplatform.OpenPlatformClaims); ok {
		return claims
	}
	return nil
}

// OpenAPIAuth returns middleware that validates open platform access tokens
// and verifies request signatures.
func OpenAPIAuth(tokenService *openplatform.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Validate Bearer token.
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

			claims, err := tokenService.ValidateAccessToken(parts[1])
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

			// 2. Verify request signature.
			timestamp := r.Header.Get("X-Timestamp")
			nonce := r.Header.Get("X-Nonce")
			signature := r.Header.Get("X-Signature")

			if timestamp == "" || nonce == "" || signature == "" {
				apperrors.WriteError(w, r, apperrors.NewSignatureMissingError(
					"missing required signature headers: X-Timestamp, X-Nonce, X-Signature",
				))
				return
			}

			// Verify timestamp freshness (±5 minutes).
			var ts int64
			for _, c := range timestamp {
				if c < '0' || c > '9' {
					apperrors.WriteError(w, r, apperrors.NewSignatureInvalidError("invalid timestamp format"))
					return
				}
				ts = ts*10 + int64(c-'0')
			}
			now := time.Now().Unix()
			if abs(now-ts) > 300 {
				apperrors.WriteError(w, r, apperrors.NewSignatureInvalidError("timestamp out of valid range (±5 minutes)"))
				return
			}

			// Fetch developer secret.
			appSecret, err := tokenService.GetDeveloperSecret(r.Context(), claims.DeveloperID)
			if err != nil {
				if appErr, ok := err.(*apperrors.AppError); ok {
					apperrors.WriteError(w, r, appErr)
				} else {
					apperrors.WriteError(w, r, apperrors.NewInternalError("failed to get developer secret", err))
				}
				return
			}

			// Verify signature.
			expectedSig := openplatform.ComputeSignature(appSecret, timestamp, nonce, r.Method, r.URL.Path)
			if !strings.EqualFold(expectedSig, signature) {
				apperrors.WriteError(w, r, apperrors.NewSignatureInvalidError("signature verification failed"))
				return
			}

			ctx := context.WithValue(r.Context(), openDevClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
