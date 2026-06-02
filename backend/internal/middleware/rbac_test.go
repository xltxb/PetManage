package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pawprint/backend/internal/module/auth"
	"pawprint/backend/internal/pkg/errcode"
)

type rbacAuthRepo struct{}

func (r rbacAuthRepo) FindUserByUsername(username string) (*auth.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r rbacAuthRepo) FindUserByID(id int64) (*auth.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r rbacAuthRepo) FindUserStores(userID int64) ([]auth.StoreInfo, error) {
	return nil, nil
}

func (r rbacAuthRepo) FindUserPermissions(userID int64, storeID int64) ([]string, error) {
	return []string{"appointment:create"}, nil
}

func (r rbacAuthRepo) FindUserStoreRole(userID, storeID int64) (*auth.UserStoreRole, error) {
	return nil, gorm.ErrRecordNotFound
}

func (r rbacAuthRepo) UpdateLastStore(userID, storeID int64) error {
	return nil
}

func TestRequirePermissionReturnsUnauthorizedWhenAuthContextIncomplete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		context func(*gin.Context)
	}{
		{
			name: "missing user_id",
			context: func(c *gin.Context) {
				c.Set("store_id", int64(1))
				c.Set("role", "staff")
			},
		},
		{
			name: "invalid user_id",
			context: func(c *gin.Context) {
				c.Set("user_id", "1")
				c.Set("store_id", int64(1))
				c.Set("role", "staff")
			},
		},
		{
			name: "missing store_id",
			context: func(c *gin.Context) {
				c.Set("user_id", int64(1))
				c.Set("role", "staff")
			},
		},
		{
			name: "invalid store_id",
			context: func(c *gin.Context) {
				c.Set("user_id", int64(1))
				c.Set("store_id", "1")
				c.Set("role", "staff")
			},
		},
	}

	authSvc := auth.NewService(rbacAuthRepo{}, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/protected", tt.context, RequirePermission(authSvc, "appointment:create"), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
			}

			var body struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body is not JSON: %v", err)
			}
			if body.Code != errcode.Unauthenticated {
				t.Fatalf("code = %d, want %d; body=%s", body.Code, errcode.Unauthenticated, w.Body.String())
			}
		})
	}
}
