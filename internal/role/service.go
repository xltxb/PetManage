package role

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
	"golang.org/x/crypto/bcrypt"
)

// Service handles platform role and user management.
type Service struct {
	db *sql.DB
}

// NewService creates a new role Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// --- Models ---

// Role represents a platform role.
type Role struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// CreateRoleRequest is the request to create a new role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleRequest is the request to update an existing role.
type UpdateRoleRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// PlatformUser represents a platform-level user.
type PlatformUser struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
	RoleID      int64  `json:"role_id"`
	RoleName    string `json:"role_name,omitempty"`
	RoleCode    string `json:"role_code,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// CreateUserRequest is the request to create a platform user.
type CreateUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
	RoleID      int64  `json:"role_id"`
}

// AssignRoleRequest is the request to change a user's role.
type AssignRoleRequest struct {
	RoleID int64 `json:"role_id"`
}

// PermissionItem describes a single assignable permission.
type PermissionItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// --- Role CRUD ---

// ListRoles returns all non-deleted platform roles.
func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, code, COALESCE(description, ''),
		 COALESCE(permissions::text, '[]'),
		 to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'),
		 to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')
		 FROM platform_roles WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to list roles", Err: err}
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		var permsJSON string
		if err := rows.Scan(&r.ID, &r.Name, &r.Code, &r.Description, &permsJSON, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to scan role", Err: err}
		}
		json.Unmarshal([]byte(permsJSON), &r.Permissions)
		roles = append(roles, r)
	}
	return roles, nil
}

// CreateRole creates a new platform role.
func (s *Service) CreateRole(ctx context.Context, req CreateRoleRequest, operatorID int64) (*Role, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" || req.Code == "" {
		return nil, &apperrors.AppError{Code: apperrors.CodeInvalidParams, Message: "name and code are required"}
	}

	if req.Permissions == nil {
		req.Permissions = []string{}
	}

	permsJSON, err := json.Marshal(req.Permissions)
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to marshal permissions", Err: err}
	}

	var r Role
	r.Permissions = req.Permissions
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO platform_roles (name, code, description, permissions)
		 VALUES ($1, $2, $3, $4::jsonb)
		 RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')`,
		req.Name, req.Code, req.Description, string(permsJSON),
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return nil, &apperrors.AppError{Code: apperrors.CodeConflict, Message: "role code already exists: " + req.Code}
		}
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to create role", Err: err}
	}
	r.Name = req.Name
	r.Code = req.Code
	r.Description = req.Description

	s.recordLog(ctx, operatorID, "create_role", "platform_role", r.ID, map[string]interface{}{
		"name":        req.Name,
		"code":        req.Code,
		"permissions": req.Permissions,
	})
	return &r, nil
}

// GetRole returns a single role by ID.
func (s *Service) GetRole(ctx context.Context, id int64) (*Role, error) {
	var r Role
	var permsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, code, COALESCE(description, ''),
		 COALESCE(permissions::text, '[]'),
		 to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'),
		 to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')
		 FROM platform_roles WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&r.ID, &r.Name, &r.Code, &r.Description, &permsJSON, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{Code: apperrors.CodeNotFound, Message: "role not found"}
	}
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to get role", Err: err}
	}
	json.Unmarshal([]byte(permsJSON), &r.Permissions)
	return &r, nil
}

// UpdateRole updates a role's name, description, and/or permissions.
func (s *Service) UpdateRole(ctx context.Context, id int64, req UpdateRoleRequest, operatorID int64) (*Role, error) {
	existing, err := s.GetRole(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Code == "super_admin" {
		return nil, &apperrors.AppError{Code: apperrors.CodeForbidden, Message: "super admin role cannot be modified"}
	}

	name := existing.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	desc := existing.Description
	if req.Description != nil {
		desc = *req.Description
	}
	perms := existing.Permissions
	if req.Permissions != nil {
		perms = req.Permissions
	}

	permsJSON, err := json.Marshal(perms)
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to marshal permissions", Err: err}
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE platform_roles SET name = $1, description = $2, permissions = $3::jsonb, updated_at = NOW()
		 WHERE id = $4 AND deleted_at IS NULL`,
		name, desc, string(permsJSON), id)
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to update role", Err: err}
	}

	s.recordLog(ctx, operatorID, "update_role", "platform_role", id, map[string]interface{}{
		"name":        name,
		"permissions": perms,
		"previous_permissions": existing.Permissions,
	})

	existing.Name = name
	existing.Description = desc
	existing.Permissions = perms
	existing.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	return existing, nil
}

// DeleteRole soft-deletes a role that has no users assigned.
func (s *Service) DeleteRole(ctx context.Context, id int64, operatorID int64) error {
	existing, err := s.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if existing.Code == "super_admin" {
		return &apperrors.AppError{Code: apperrors.CodeForbidden, Message: "super admin role cannot be deleted"}
	}

	var userCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM platform_users WHERE role_id = $1 AND deleted_at IS NULL`, id,
	).Scan(&userCount)
	if err != nil {
		return &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to check role usage", Err: err}
	}
	if userCount > 0 {
		return &apperrors.AppError{Code: apperrors.CodeForbidden, Message: "role is assigned to users and cannot be deleted"}
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE platform_roles SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to delete role", Err: err}
	}

	s.recordLog(ctx, operatorID, "delete_role", "platform_role", id, map[string]interface{}{
		"name": existing.Name,
		"code": existing.Code,
	})
	return nil
}

// --- User management ---

// ListUsers returns all non-deleted platform users with their role info.
func (s *Service) ListUsers(ctx context.Context) ([]PlatformUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.username, COALESCE(u.display_name, ''),
		 COALESCE(u.phone, ''), COALESCE(u.email, ''),
		 COALESCE(u.role_id, 0), COALESCE(r.name, ''), COALESCE(r.code, ''),
		 u.status,
		 to_char(u.created_at, 'YYYY-MM-DD HH24:MI:SS')
		 FROM platform_users u
		 LEFT JOIN platform_roles r ON u.role_id = r.id AND r.deleted_at IS NULL
		 WHERE u.deleted_at IS NULL AND u.merchant_id IS NULL
		 ORDER BY u.id`)
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to list users", Err: err}
	}
	defer rows.Close()

	var users []PlatformUser
	for rows.Next() {
		var u PlatformUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Phone, &u.Email,
			&u.RoleID, &u.RoleName, &u.RoleCode, &u.Status, &u.CreatedAt); err != nil {
			return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to scan user", Err: err}
		}
		users = append(users, u)
	}
	return users, nil
}

// CreateUser creates a new platform user.
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest, operatorID int64) (*PlatformUser, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Username == "" || req.Password == "" {
		return nil, &apperrors.AppError{Code: apperrors.CodeInvalidParams, Message: "username and password are required"}
	}
	if len(req.Password) < 6 {
		return nil, &apperrors.AppError{Code: apperrors.CodeInvalidParams, Message: "password must be at least 6 characters"}
	}
	if req.RoleID <= 0 {
		return nil, &apperrors.AppError{Code: apperrors.CodeInvalidParams, Message: "role_id is required"}
	}

	// Verify the role exists.
	_, err := s.GetRole(ctx, req.RoleID)
	if err != nil {
		return nil, err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to hash password", Err: err}
	}

	var u PlatformUser
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO platform_users (username, password_hash, display_name, phone, email, role_id, status, must_change_password)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active', true)
		 RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS')`,
		req.Username, string(hashed), req.DisplayName, req.Phone, req.Email, req.RoleID,
	).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return nil, &apperrors.AppError{Code: apperrors.CodeConflict, Message: "username already exists: " + req.Username}
		}
		return nil, &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to create user", Err: err}
	}
	u.Username = req.Username
	u.DisplayName = req.DisplayName
	u.Phone = req.Phone
	u.Email = req.Email
	u.RoleID = req.RoleID
	u.Status = "active"

	s.recordLog(ctx, operatorID, "create_user", "platform_user", u.ID, map[string]interface{}{
		"username":     req.Username,
		"display_name": req.DisplayName,
		"role_id":      req.RoleID,
	})
	return &u, nil
}

// AssignRole changes a platform user's role and records the change.
func (s *Service) AssignRole(ctx context.Context, userID, roleID, operatorID int64) error {
	if userID <= 0 || roleID <= 0 {
		return &apperrors.AppError{Code: apperrors.CodeInvalidParams, Message: "user_id and role_id are required"}
	}

	// Get current role for logging.
	var oldRoleID int64
	var oldRoleName string
	var username string
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(u.role_id, 0), COALESCE(r.name, ''), u.username
		 FROM platform_users u
		 LEFT JOIN platform_roles r ON u.role_id = r.id
		 WHERE u.id = $1 AND u.deleted_at IS NULL AND u.merchant_id IS NULL`,
		userID,
	).Scan(&oldRoleID, &oldRoleName, &username)
	if errors.Is(err, sql.ErrNoRows) {
		return &apperrors.AppError{Code: apperrors.CodeNotFound, Message: "platform user not found"}
	}
	if err != nil {
		return &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to find user", Err: err}
	}

	// Verify new role exists.
	newRole, err := s.GetRole(ctx, roleID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE platform_users SET role_id = $1, updated_at = NOW() WHERE id = $2`,
		roleID, userID)
	if err != nil {
		return &apperrors.AppError{Code: apperrors.CodeInternalError, Message: "failed to assign role", Err: err}
	}

	s.recordLog(ctx, operatorID, "assign_role", "platform_user", userID, map[string]interface{}{
		"username":       username,
		"old_role_id":    oldRoleID,
		"old_role_name":  oldRoleName,
		"new_role_id":    roleID,
		"new_role_name":  newRole.Name,
	})
	return nil
}

// GetRolePermissions fetches the permissions array for a given role.
func (s *Service) GetRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	var permsStr string
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(permissions::text, '[]') FROM platform_roles
		 WHERE id = $1 AND deleted_at IS NULL`, roleID,
	).Scan(&permsStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var perms []string
	json.Unmarshal([]byte(permsStr), &perms)
	return perms, nil
}

// GetAvailablePermissions returns the full list of assignable permissions.
func (s *Service) GetAvailablePermissions() []PermissionItem {
	return []PermissionItem{
		{Key: "merchant:view", Name: "商户查看", Category: "商户管理", Description: "查看商户列表与详情"},
		{Key: "merchant:manage", Name: "商户管理", Category: "商户管理", Description: "审核、冻结、解冻、关停商户"},
		{Key: "contract:view", Name: "合同查看", Category: "合同管理", Description: "查看合同列表与详情"},
		{Key: "contract:manage", Name: "合同管理", Category: "合同管理", Description: "上传、续签合同"},
		{Key: "dict:view", Name: "数据字典查看", Category: "数据字典", Description: "查看分类与品种"},
		{Key: "dict:manage", Name: "数据字典管理", Category: "数据字典", Description: "创建、编辑、删除分类与品种"},
		{Key: "role:view", Name: "角色查看", Category: "权限管理", Description: "查看角色列表与详情"},
		{Key: "role:manage", Name: "角色管理", Category: "权限管理", Description: "创建、编辑、删除角色"},
		{Key: "user:view", Name: "用户查看", Category: "用户管理", Description: "查看平台用户列表"},
		{Key: "user:manage", Name: "用户管理", Category: "用户管理", Description: "创建用户、分配角色"},
		{Key: "announcement:view", Name: "公告查看", Category: "公告管理", Description: "查看公告列表"},
		{Key: "announcement:manage", Name: "公告管理", Category: "公告管理", Description: "发布、编辑、删除公告"},
	}
}

// --- Audit logging ---

func (s *Service) recordLog(ctx context.Context, userID int64, action, targetType string, targetID int64, detail map[string]interface{}) {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return
	}
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail)
		 VALUES ($1, $2, $3, $4, $5::jsonb)`,
		userID, action, targetType, targetID, string(detailJSON),
	)
}
