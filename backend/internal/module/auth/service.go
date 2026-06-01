package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"pawprint/backend/internal/pkg/apperr"
	"pawprint/backend/internal/pkg/errcode"
)

// Service handles authentication business logic.
type Service struct {
	repo          Repository
	accessSecret  string
	refreshSecret string
}

func NewService(repo Repository, accessSecret, refreshSecret string) *Service {
	return &Service{
		repo:          repo,
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
	}
}

// AccessClaims for access tokens.
type AccessClaims struct {
	jwt.RegisteredClaims
	UserID  int64  `json:"uid"`
	StoreID int64  `json:"store_id"`
	Role    string `json:"role"`
}

// RefreshClaims for refresh tokens.
type RefreshClaims struct {
	jwt.RegisteredClaims
	UserID int64 `json:"uid"`
}

// Login authenticates a user and returns tokens with store access list.
func (s *Service) Login(username, password string) (*LoginResponse, error) {
	user, err := s.repo.FindUserByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.Unauthorized("用户名或密码错误")
		}
		return nil, apperr.Internal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperr.Unauthorized("用户名或密码错误")
	}

	stores, err := s.repo.FindUserStores(user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if len(stores) == 0 {
		return nil, apperr.Forbidden("未分配到任何门店")
	}

	firstStoreID := stores[0].ID
	firstRole := stores[0].Role

	access, err := s.issueAccess(user.ID, firstStoreID, firstRole)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	refresh, err := s.issueRefresh(user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	return &LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		Stores:       stores,
	}, nil
}

// RefreshToken validates a refresh token and issues new tokens.
func (s *Service) RefreshToken(refreshToken string) (*LoginResponse, error) {
	claims := &RefreshClaims{}
	token, err := jwt.ParseWithClaims(refreshToken, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(s.refreshSecret), nil
		})
	if err != nil || !token.Valid {
		return nil, apperr.Unauthorized("refresh token 无效或已过期")
	}

	user, err := s.repo.FindUserByID(claims.UserID)
	if err != nil {
		return nil, apperr.Unauthorized("用户已禁用或不存在")
	}

	stores, err := s.repo.FindUserStores(user.ID)
	if err != nil || len(stores) == 0 {
		return nil, apperr.Internal(err)
	}

	access, _ := s.issueAccess(user.ID, stores[0].ID, stores[0].Role)
	refresh, _ := s.issueRefresh(user.ID)

	return &LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		Stores:       stores,
	}, nil
}

// SwitchStore validates the user has access to the target store and issues new scoped tokens.
func (s *Service) SwitchStore(userID, storeID int64) (*SwitchStoreResponse, error) {
	usr, err := s.repo.FindUserStoreRole(userID, storeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(errcode.StoreForbidden, "无该门店访问权限")
		}
		return nil, apperr.Internal(err)
	}

	access, err := s.issueAccess(userID, storeID, usr.Role.Code)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	refresh, err := s.issueRefresh(userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	_ = s.repo.UpdateLastStore(userID, storeID)

	return &SwitchStoreResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

// GetPermissions returns the permission codes for a user in a store.
func (s *Service) GetPermissions(userID, storeID int64) ([]string, error) {
	return s.repo.FindUserPermissions(userID, storeID)
}

// VerifyStoreAccess checks a user has a role in the given store.
func (s *Service) VerifyStoreAccess(userID, storeID int64) (*StoreInfo, error) {
	usr, err := s.repo.FindUserStoreRole(userID, storeID)
	if err != nil {
		return nil, apperr.New(errcode.StoreForbidden, "无该门店访问权限")
	}
	return &StoreInfo{
		ID:   storeID,
		Name: "",
		Role: usr.Role.Code,
	}, nil
}

// ParseAccessToken validates and returns access token claims.
func (s *Service) ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(s.accessSecret), nil
		})
	if err != nil || !token.Valid {
		return nil, apperr.Unauthorized("access token 无效或已过期")
	}
	return claims, nil
}

func (s *Service) issueAccess(userID, storeID int64, role string) (string, error) {
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:  userID,
		StoreID: storeID,
		Role:    role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.accessSecret))
}

func (s *Service) issueRefresh(userID int64) (string, error) {
	claims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(720 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.refreshSecret))
}
