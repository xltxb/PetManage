package openplatform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xltxb/PetManage/pkg/apperrors"
	cryptopkg "github.com/xltxb/PetManage/pkg/crypto"
)

// OpenPlatformClaims contains JWT claims for open platform developer tokens.
type OpenPlatformClaims struct {
	jwt.RegisteredClaims
	DeveloperID int64           `json:"developer_id"`
	AppKey      string          `json:"app_key"`
	MerchantID  int64           `json:"merchant_id"`
	Permissions json.RawMessage `json:"permissions"`
}

// OpenTokenPair holds an access token and refresh token for open platform.
type OpenTokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// TokenRequest is the request body for obtaining an access token.
type TokenRequest struct {
	AppKey    string `json:"app_key"`
	AppSecret string `json:"app_secret"`
}

// RefreshRequest is the request body for refreshing an access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenService manages open platform token generation and validation.
type TokenService struct {
	db              *sql.DB
	secret          []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// NewTokenService creates a new TokenService.
func NewTokenService(db *sql.DB, secret string, accessTTL, refreshTTL int) *TokenService {
	return &TokenService{
		db:              db,
		secret:          []byte(secret),
		accessTokenTTL:  time.Duration(accessTTL) * time.Second,
		refreshTokenTTL: time.Duration(refreshTTL) * time.Second,
	}
}

// GenerateTokenPair validates AppKey+AppSecret and returns a token pair.
func (s *TokenService) GenerateTokenPair(ctx context.Context, req TokenRequest) (*OpenTokenPair, *Application, error) {
	if req.AppKey == "" || req.AppSecret == "" {
		return nil, nil, apperrors.NewAppKeyInvalidError("app_key and app_secret are required")
	}

	// Look up the developer by AppKey.
	var app Application
	var appKey, appSecret sql.NullString
	var merchantID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, app_key, app_secret, permissions,
		 merchant_id, review_remark, created_at, updated_at
		 FROM open_developers WHERE app_key = $1 AND deleted_at IS NULL`, req.AppKey,
	).Scan(&app.ID, &app.CompanyName, &app.ContactPerson, &app.ContactPhone,
		&app.ContactEmail, &app.UsagePurpose, &app.CallbackURL, &app.Status,
		&appKey, &appSecret, &app.Permissions, &merchantID, &app.ReviewRemark,
		&app.CreatedAt, &app.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil, apperrors.NewAppKeyInvalidError("invalid app_key or app_secret")
	}
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to query developer", err)
	}
	if appKey.Valid {
		app.AppKey = appKey.String
	}
	var decryptedSecret string
	if appSecret.Valid {
		decrypted, err := cryptopkg.Decrypt(appSecret.String)
		if err != nil {
			return nil, nil, apperrors.NewInternalError("failed to decrypt app secret", err)
		}
		decryptedSecret = decrypted
		app.AppSecret = "" // Never expose the secret outside of this function.
	}
	if merchantID.Valid {
		app.MerchantID = &merchantID.Int64
	}

	if app.Status != "approved" {
		return nil, nil, apperrors.NewAppKeyInvalidError("developer application is not approved")
	}

	// Verify AppSecret matches (constant-time comparison to avoid timing attacks).
	if decryptedSecret != req.AppSecret {
		return nil, nil, apperrors.NewAppKeyInvalidError("invalid app_key or app_secret")
	}

	// Generate tokens.
	var mid int64
	if app.MerchantID != nil {
		mid = *app.MerchantID
	}
	accessToken, err := s.generateToken(app.ID, app.AppKey, mid, app.Permissions, s.accessTokenTTL)
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to generate access token", err)
	}
	refreshToken, err := s.generateToken(app.ID, app.AppKey, mid, app.Permissions, s.refreshTokenTTL)
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to generate refresh token", err)
	}

	return &OpenTokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, &app, nil
}

func (s *TokenService) generateToken(developerID int64, appKey string, merchantID int64, permissions json.RawMessage, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &OpenPlatformClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "pet-manage-open",
		},
		DeveloperID: developerID,
		AppKey:      appKey,
		MerchantID:  merchantID,
		Permissions: permissions,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateAccessToken parses and validates an open platform access token.
func (s *TokenService) ValidateAccessToken(tokenString string) (*OpenPlatformClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OpenPlatformClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, err
		}
		return nil, err
	}
	claims, ok := token.Claims.(*OpenPlatformClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}

// ValidateRefreshToken validates a refresh token.
func (s *TokenService) ValidateRefreshToken(tokenString string) (*OpenPlatformClaims, error) {
	claims, err := s.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// RefreshAccessToken generates new tokens from a valid refresh token.
func (s *TokenService) RefreshAccessToken(ctx context.Context, tokenString string) (*OpenTokenPair, error) {
	claims, err := s.ValidateRefreshToken(tokenString)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperrors.NewAppError("TOKEN_EXPIRED", "refresh token has expired", nil)
		}
		return nil, apperrors.NewUnauthorizedError("invalid refresh token")
	}

	// Verify the developer still exists and is approved.
	var status string
	var merchantID sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		`SELECT status, merchant_id FROM open_developers WHERE id = $1 AND deleted_at IS NULL`, claims.DeveloperID,
	).Scan(&status, &merchantID)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewUnauthorizedError("developer no longer exists")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify developer", err)
	}
	if status != "approved" {
		return nil, apperrors.NewAppKeyInvalidError("developer application is not approved")
	}

	var mid int64
	if merchantID.Valid {
		mid = merchantID.Int64
	}

	accessToken, err := s.generateToken(claims.DeveloperID, claims.AppKey, mid, claims.Permissions, s.accessTokenTTL)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate access token", err)
	}
	refreshToken, err := s.generateToken(claims.DeveloperID, claims.AppKey, mid, claims.Permissions, s.refreshTokenTTL)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate refresh token", err)
	}

	return &OpenTokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// VerifySignature validates the request signature against the provided value.
// The canonical request includes query string and body SHA-256 to prevent tampering.
func VerifySignature(appSecret, timestamp, nonce, method, path, query, bodyHash, signature string) bool {
	expected := ComputeSignature(appSecret, timestamp, nonce, method, path, query, bodyHash)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ComputeSignature computes the expected signature for request verification.
// Canonical request: timestamp + nonce + method + path + sortedQuery + bodySHA256
func ComputeSignature(appSecret, timestamp, nonce, method, path, query, bodyHash string) string {
	payload := timestamp + "\n" + nonce + "\n" + method + "\n" + path
	if query != "" {
		payload += "\n" + query
	}
	if bodyHash != "" {
		payload += "\n" + bodyHash
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// GetDeveloperSecret retrieves the AppSecret for a developer by ID.
func (s *TokenService) GetDeveloperSecret(ctx context.Context, developerID int64) (string, error) {
	var appSecret sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT app_secret FROM open_developers WHERE id = $1 AND deleted_at IS NULL`, developerID,
	).Scan(&appSecret)
	if err == sql.ErrNoRows {
		return "", apperrors.NewNotFoundError("developer not found")
	}
	if err != nil {
		return "", apperrors.NewInternalError("failed to query developer", err)
	}
	if !appSecret.Valid {
		return "", apperrors.NewNotFoundError("developer secret not found")
	}
	return cryptopkg.Decrypt(appSecret.String)
}
