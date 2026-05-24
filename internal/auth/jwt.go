package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims contains the JWT token claims for platform users.
type Claims struct {
	jwt.RegisteredClaims
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	RoleID     int64  `json:"role_id"`
	MerchantID *int64 `json:"merchant_id,omitempty"`
	EmployeeID *int64 `json:"employee_id,omitempty"`
	TokenType  string `json:"token_type,omitempty"`
}

// TokenPair holds an access token and refresh token.
type TokenPair struct {
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"refresh_token"`
	ExpiresIn          int64  `json:"expires_in"`
	TokenType          string `json:"token_type"`
	MustChangePassword bool   `json:"must_change_password"`
}

// JWTManager handles token generation and validation.
type JWTManager struct {
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewJWTManager creates a new JWTManager.
func NewJWTManager(secret string, accessTTL, refreshTTL int) *JWTManager {
	return &JWTManager{
		secret:          []byte(secret),
		accessTokenTTL:  time.Duration(accessTTL) * time.Second,
		refreshTokenTTL: time.Duration(refreshTTL) * time.Second,
	}
}

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// GenerateTokenPair creates both access and refresh tokens.
func (m *JWTManager) GenerateTokenPair(userID int64, username string, roleID int64, merchantID *int64, employeeID *int64) (*TokenPair, error) {
	accessToken, err := m.generateToken(userID, username, roleID, merchantID, employeeID, m.accessTokenTTL, tokenTypeAccess)
	if err != nil {
		return nil, err
	}

	refreshToken, err := m.generateToken(userID, username, roleID, merchantID, employeeID, m.refreshTokenTTL, tokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(m.accessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func (m *JWTManager) generateToken(userID int64, username string, roleID int64, merchantID *int64, employeeID *int64, ttl time.Duration, tokenType string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "pet-manage",
		},
		UserID:     userID,
		Username:   username,
		RoleID:     roleID,
		MerchantID: merchantID,
		EmployeeID: employeeID,
		TokenType:  tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateAccessToken parses and validates a JWT access token.
// Rejects tokens with token_type "refresh". Old tokens without token_type are accepted.
func (m *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := m.parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType == tokenTypeRefresh {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}

// ValidateRefreshToken parses and validates a JWT refresh token.
// Only accepts tokens with token_type "refresh". Also accepts old tokens without
// token_type for backward compatibility during the transition window.
func (m *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := m.parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType == tokenTypeAccess {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}

// ValidateToken is the legacy method — prefer ValidateAccessToken / ValidateRefreshToken.
// Deprecated: retained for backward compatibility with existing callers that have not
// been updated for token-type isolation.
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	return m.ValidateAccessToken(tokenString)
}

func (m *JWTManager) parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
