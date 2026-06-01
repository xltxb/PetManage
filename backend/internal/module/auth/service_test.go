package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// mockRepo implements Repository for testing
type mockRepo struct {
	users        map[string]*User
	stores       map[int64][]StoreInfo
	permissions  map[int64][]string
	storeRoles   map[int64]*UserStoreRole
	lastStoreErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:       make(map[string]*User),
		stores:      make(map[int64][]StoreInfo),
		permissions: make(map[int64][]string),
		storeRoles:  make(map[int64]*UserStoreRole),
	}
}

func (m *mockRepo) FindUserByUsername(username string) (*User, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (m *mockRepo) FindUserByID(id int64) (*User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockRepo) FindUserStores(userID int64) ([]StoreInfo, error) {
	return m.stores[userID], nil
}

func (m *mockRepo) FindUserPermissions(userID, storeID int64) ([]string, error) {
	return m.permissions[userID], nil
}

func (m *mockRepo) FindUserStoreRole(userID, storeID int64) (*UserStoreRole, error) {
	usr, ok := m.storeRoles[storeID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return usr, nil
}

func (m *mockRepo) UpdateLastStore(userID, storeID int64) error {
	return m.lastStoreErr
}

func mustHashPassword(pw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func TestLoginSuccess(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{
		ID:           1,
		Username:     "admin",
		PasswordHash: mustHashPassword("pawprint123"),
		DisplayName:  "管理员",
		Status:       1,
	}
	repo.stores[1] = []StoreInfo{
		{ID: 1, Name: "旗舰店", Role: "super_admin"},
	}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	resp, err := svc.Login("admin", "pawprint123")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if resp.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	if len(resp.Stores) != 1 {
		t.Errorf("stores count = %d, want 1", len(resp.Stores))
	}
	if resp.Stores[0].Role != "super_admin" {
		t.Errorf("role = %q, want super_admin", resp.Stores[0].Role)
	}
}

func TestLoginBadPassword(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{
		ID: 1, Username: "admin",
		PasswordHash: mustHashPassword("correct"),
		Status:       1,
	}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	_, err := svc.Login("admin", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if ae, ok := err.(interface{ CodeValue() int }); ok {
		t.Errorf("unexpected interface")
		_ = ae
	}
}

func TestLoginUserNotFound(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	_, err := svc.Login("nonexistent", "pw")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestRefreshToken(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{ID: 1, Username: "admin", Status: 1,
		PasswordHash: mustHashPassword("pw")}
	repo.stores[1] = []StoreInfo{{ID: 1, Name: "旗舰店", Role: "super_admin"}}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	loginResp, err := svc.Login("admin", "pw")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	resp, err := svc.RefreshToken(loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("new access token should not be empty")
	}
}

func TestSwitchStoreUnauthorized(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	_, err := svc.SwitchStore(1, 999)
	if err == nil {
		t.Fatal("expected error for unauthorized store")
	}
}

func TestParseAccessToken(t *testing.T) {
	repo := newMockRepo()
	repo.users["admin"] = &User{ID: 1, Username: "admin", Status: 1,
		PasswordHash: mustHashPassword("pw")}
	repo.stores[1] = []StoreInfo{{ID: 1, Name: "旗舰店", Role: "super_admin"}}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	resp, _ := svc.Login("admin", "pw")
	claims, err := svc.ParseAccessToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken() error: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Role != "super_admin" {
		t.Errorf("Role = %q, want super_admin", claims.Role)
	}
}

func TestVerifyStoreAccess(t *testing.T) {
	repo := newMockRepo()
	repo.storeRoles[1] = &UserStoreRole{
		UserID: 1, StoreID: 1, RoleID: 1,
		Role: Role{Code: "store_manager", Name: "店长"},
	}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	info, err := svc.VerifyStoreAccess(1, 1)
	if err != nil {
		t.Fatalf("VerifyStoreAccess() error: %v", err)
	}
	if info.Role != "store_manager" {
		t.Errorf("Role = %q, want store_manager", info.Role)
	}
}

func TestGetPermissions(t *testing.T) {
	repo := newMockRepo()
	repo.permissions[1] = []string{"appointment:create", "dashboard:view"}

	svc := NewService(repo, "access-secret-32-chars-minimum!!", "refresh-secret-32-chars-minimum!!")

	perms, err := svc.GetPermissions(1, 1)
	if err != nil {
		t.Fatalf("GetPermissions() error: %v", err)
	}
	if len(perms) != 2 {
		t.Errorf("perms count = %d, want 2", len(perms))
	}
}
