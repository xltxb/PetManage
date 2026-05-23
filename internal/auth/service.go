package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	var merchantStatus sql.NullString
	var merchantID sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username, u.password_hash, COALESCE(u.role_id, 0), COALESCE(u.must_change_password, false),
			m.status, u.merchant_id
		 FROM platform_users u
		 LEFT JOIN merchants m ON u.merchant_id = m.id AND m.deleted_at IS NULL
		 WHERE u.username = $1 AND u.deleted_at IS NULL AND u.status = 'active'`,
		req.Username,
	).Scan(&userID, &username, &passwordHash, &roleID, &mustChangePassword, &merchantStatus, &merchantID)

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

	// Check merchant status for merchant-level users.
	if merchantStatus.Valid {
		switch merchantStatus.String {
		case "frozen":
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeMerchantFrozen,
				Message: "merchant account has been frozen, please contact platform administrator",
			}
		case "closed":
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeMerchantClosed,
				Message: "merchant account has been permanently closed",
			}
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

	tokenPair, err := s.jwt.GenerateTokenPair(userID, username, roleID, nullableToPtr(merchantID), nil)
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

	// Verify the user still exists and is active, and check merchant status.
	var userStatus string
	var merchantStatus sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT u.status, m.status FROM platform_users u
		 LEFT JOIN merchants m ON u.merchant_id = m.id AND m.deleted_at IS NULL
		 WHERE u.id = $1 AND u.deleted_at IS NULL`,
		claims.UserID,
	).Scan(&userStatus, &merchantStatus)

	if errors.Is(err, sql.ErrNoRows) || userStatus != "active" {
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

	if merchantStatus.Valid {
		switch merchantStatus.String {
		case "frozen":
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeMerchantFrozen,
				Message: "merchant account has been frozen, please contact platform administrator",
			}
		case "closed":
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeMerchantClosed,
				Message: "merchant account has been permanently closed",
			}
		}
	}

	return s.jwt.GenerateTokenPair(claims.UserID, claims.Username, claims.RoleID, claims.MerchantID, nil)
}

// MerchantLoginResponse extends TokenPair with merchant-specific info.
type MerchantLoginResponse struct {
	*TokenPair
	MerchantName string `json:"merchant_name"`
	DisplayName  string `json:"display_name"`
}

// MerchantLogin authenticates a merchant admin user with lockout protection.
func (s *Service) MerchantLogin(ctx context.Context, req LoginRequest) (*MerchantLoginResponse, error) {
	var userID int64
	var username string
	var passwordHash string
	var roleID int64
	var mustChangePassword bool
	var merchantID sql.NullInt64
	var merchantName sql.NullString
	var merchantStatus sql.NullString
	var displayName sql.NullString
	var loginFailCount int
	var lockedUntil sql.NullTime
	var employeeID sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username, u.password_hash, COALESCE(u.role_id, 0),
			COALESCE(u.must_change_password, false),
			COALESCE(u.display_name, ''),
			u.merchant_id, m.name, m.status,
			u.employee_id,
			u.login_fail_count, u.locked_until
		 FROM platform_users u
		 LEFT JOIN merchants m ON u.merchant_id = m.id AND m.deleted_at IS NULL
		 WHERE u.username = $1 AND u.merchant_id IS NOT NULL
		 AND u.deleted_at IS NULL AND u.status = 'active'`,
		req.Username,
	).Scan(&userID, &username, &passwordHash, &roleID, &mustChangePassword,
		&displayName, &merchantID, &merchantName, &merchantStatus,
		&employeeID,
		&loginFailCount, &lockedUntil)

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

	// Check if account is locked.
	if lockedUntil.Valid && lockedUntil.Time.After(time.Now()) {
		remaining := int(time.Until(lockedUntil.Time).Minutes())
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeAccountLocked,
			Message: "account is locked due to too many failed attempts, try again in " + formatMinutes(remaining),
		}
	}

	// Check merchant status.
	if merchantStatus.Valid {
		switch merchantStatus.String {
		case "frozen":
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeMerchantFrozen,
				Message: "merchant account has been frozen, please contact platform administrator",
			}
		case "closed":
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeMerchantClosed,
				Message: "merchant account has been permanently closed",
			}
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		// Increment failed login count.
		newCount := loginFailCount + 1
		if newCount >= 3 {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE platform_users SET login_fail_count = $1, locked_until = NOW() + INTERVAL '15 minutes' WHERE id = $2`,
				newCount, userID,
			)
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeAccountLocked,
				Message: "account is locked due to too many failed attempts, try again in 15 minutes",
			}
		}
		_, _ = s.db.ExecContext(ctx,
			`UPDATE platform_users SET login_fail_count = $1 WHERE id = $2`,
			newCount, userID,
		)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidCredentials,
			Message: "invalid username or password",
		}
	}

	// Successful login — reset fail count and lock.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE platform_users SET last_login_at = NOW(), login_fail_count = 0, locked_until = NULL WHERE id = $1`,
		userID,
	)

	// Auto-unlock employee shift lock on re-login.
	if employeeID.Valid && employeeID.Int64 > 0 {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE employees SET shift_locked = false, updated_at = NOW()
			 WHERE id = $1 AND shift_locked = true`,
			employeeID.Int64,
		)
	}

	tokenPair, err := s.jwt.GenerateTokenPair(userID, username, roleID, nullableToPtr(merchantID), nullableToPtr(employeeID))
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "token generation failed",
			Err:     err,
		}
	}
	tokenPair.MustChangePassword = mustChangePassword

	merchant := ""
	if merchantName.Valid {
		merchant = merchantName.String
	}
	display := ""
	if displayName.Valid {
		display = displayName.String
	}

	return &MerchantLoginResponse{
		TokenPair:    tokenPair,
		MerchantName: merchant,
		DisplayName:  display,
	}, nil
}

func formatMinutes(remaining int) string {
	if remaining <= 0 {
		return "less than a minute"
	}
	if remaining == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", remaining)
}

func nullableToPtr(n sql.NullInt64) *int64 {
	if n.Valid {
		return &n.Int64
	}
	return nil
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
