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
	var mustChangePassword bool

	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, COALESCE(role_id, 0), COALESCE(must_change_password, false)
		 FROM platform_users
		 WHERE username = $1 AND deleted_at IS NULL AND status = 'active'`,
		req.Username,
	).Scan(&userID, &username, &passwordHash, &roleID, &mustChangePassword)

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

	tokenPair, err := s.jwt.GenerateTokenPair(userID, username, roleID)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "token generation failed",
			Err:     err,
		}
	}
	tokenPair.MustChangePassword = mustChangePassword
	return tokenPair, nil
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

// ChangePasswordRequest is the change password request body.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePasswordResponse is the change password response.
type ChangePasswordResponse struct {
	Message string `json:"message"`
}

// ChangePassword verifies the old password and updates to the new password.
func (s *Service) ChangePassword(ctx context.Context, userID int64, req ChangePasswordRequest) (*ChangePasswordResponse, error) {
	if req.OldPassword == "" || req.NewPassword == "" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "old_password and new_password are required",
		}
	}

	if len(req.NewPassword) < 6 {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "new_password must be at least 6 characters",
		}
	}

	var passwordHash string
	err := s.db.QueryRowContext(ctx,
		`SELECT password_hash FROM platform_users
		 WHERE id = $1 AND deleted_at IS NULL AND status = 'active'`,
		userID,
	).Scan(&passwordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeUnauthorized,
			Message: "user not found or inactive",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "password change failed",
			Err:     err,
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidCredentials,
			Message: "old password is incorrect",
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "password change failed",
			Err:     err,
		}
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE platform_users SET password_hash = $1, must_change_password = false, updated_at = NOW() WHERE id = $2`,
		string(hashedPassword), userID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "password change failed",
			Err:     err,
		}
	}

	return &ChangePasswordResponse{Message: "password changed successfully"}, nil
}
