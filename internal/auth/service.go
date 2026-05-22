package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xltxb/PetManage/pkg/apperrors"
	"golang.org/x/crypto/bcrypt"
)

// Service handles authentication operations (login, token refresh).
type Service struct {
	db  *sql.DB
	jwt *JWTManager
}

// NewService creates a new auth Service.
func NewService(db *sql.DB, jwt *JWTManager) *Service {
	return &Service{db: db, jwt: jwt}
}

// LoginRequest is the login request body.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login verifies credentials and returns a token pair.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	var userID int64
	var username string
	var passwordHash string
	var roleID int64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, COALESCE(role_id, 0)
		 FROM platform_users
		 WHERE username = $1 AND deleted_at IS NULL AND status = 'active'`,
		req.Username,
	).Scan(&userID, &username, &passwordHash, &roleID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidCredentials,
			Message: "invalid username or password",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "authentication failed",
			Err:     err,
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidCredentials,
			Message: "invalid username or password",
		}
	}

	// Update last login time.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE platform_users SET last_login_at = NOW() WHERE id = $1`,
		userID,
	)

	return s.jwt.GenerateTokenPair(userID, username, roleID)
}

// RefreshRequest is the refresh token request body.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *Service) RefreshToken(ctx context.Context, req RefreshRequest) (*TokenPair, error) {
	claims, err := s.jwt.ValidateToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeTokenExpired,
				Message: "refresh token has expired, please login again",
			}
		}
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeUnauthorized,
			Message: "invalid refresh token",
		}
	}

	// Verify the user still exists and is active.
	var status string
	err = s.db.QueryRowContext(ctx,
		`SELECT status FROM platform_users
		 WHERE id = $1 AND deleted_at IS NULL`,
		claims.UserID,
	).Scan(&status)

	if errors.Is(err, sql.ErrNoRows) || status != "active" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeUnauthorized,
			Message: "user account is no longer active",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "token refresh failed",
			Err:     err,
		}
	}

	return s.jwt.GenerateTokenPair(claims.UserID, claims.Username, claims.RoleID)
}
