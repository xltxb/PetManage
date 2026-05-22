package merchantrole

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
	"golang.org/x/crypto/bcrypt"
)

// Service handles merchant-level role and permission management.
type Service struct {
	db *sql.DB
}

// NewService creates a new merchant role Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Role represents a merchant role.
type Role struct {
	ID          int64    `json:"id"`
	MerchantID  int64    `json:"merchant_id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// CreateRoleRequest is the request to create a merchant role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleRequest is the request to update a merchant role.
type UpdateRoleRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// AssignRoleRequest is the request to assign a role to an employee.
type AssignRoleRequest struct {
	RoleID int64 `json:"role_id"`
}

// PermissionItem describes a single assignable permission for merchants.
type PermissionItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// CreateRole creates a new merchant role.
func (s *Service) CreateRole(ctx context.Context, merchantID int64, req CreateRoleRequest) (*Role, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" || req.Code == "" {
		return nil, apperrors.NewValidationError("name and code are required")
	}
	if req.Permissions == nil {
		req.Permissions = []string{}
	}

	permsJSON, err := json.Marshal(req.Permissions)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to marshal permissions", err)
	}

	var r Role
	r.Permissions = req.Permissions
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO merchant_roles (merchant_id, name, code, description, permissions)
		 VALUES ($1, $2, $3, $4, $5::jsonb)
		 RETURNING id, to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')`,
		merchantID, req.Name, req.Code, req.Description, string(permsJSON),
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return nil, apperrors.NewConflictError("role code already exists: " + req.Code)
		}
		return nil, apperrors.NewInternalError("failed to create role", err)
	}
	r.MerchantID = merchantID
	r.Name = req.Name
	r.Code = req.Code
	r.Description = req.Description
	return &r, nil
}

// ListRoles returns all non-deleted roles for a merchant.
func (s *Service) ListRoles(ctx context.Context, merchantID int64) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, name, code, COALESCE(description, ''),
		 COALESCE(permissions::text, '[]'),
		 to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'),
		 to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')
		 FROM merchant_roles WHERE merchant_id = $1 AND deleted_at IS NULL ORDER BY id`, merchantID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list roles", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		var permsJSON string
		if err := rows.Scan(&r.ID, &r.MerchantID, &r.Name, &r.Code, &r.Description, &permsJSON, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan role", err)
		}
		json.Unmarshal([]byte(permsJSON), &r.Permissions)
		roles = append(roles, r)
	}
	if roles == nil {
		roles = []Role{}
	}
	return roles, nil
}

// GetRole returns a single role by ID with merchant ownership verification.
func (s *Service) GetRole(ctx context.Context, merchantID, roleID int64) (*Role, error) {
	var r Role
	var permsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, name, code, COALESCE(description, ''),
		 COALESCE(permissions::text, '[]'),
		 to_char(created_at, 'YYYY-MM-DD HH24:MI:SS'),
		 to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')
		 FROM merchant_roles WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		roleID, merchantID,
	).Scan(&r.ID, &r.MerchantID, &r.Name, &r.Code, &r.Description, &permsJSON, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("role not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get role", err)
	}
	json.Unmarshal([]byte(permsJSON), &r.Permissions)
	return &r, nil
}

// UpdateRole updates a merchant role.
func (s *Service) UpdateRole(ctx context.Context, merchantID, roleID int64, req UpdateRoleRequest) (*Role, error) {
	existing, err := s.GetRole(ctx, merchantID, roleID)
	if err != nil {
		return nil, err
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
		return nil, apperrors.NewInternalError("failed to marshal permissions", err)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE merchant_roles SET name = $1, description = $2, permissions = $3::jsonb, updated_at = NOW()
		 WHERE id = $4 AND merchant_id = $5 AND deleted_at IS NULL`,
		name, desc, string(permsJSON), roleID, merchantID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update role", err)
	}

	existing.Name = name
	existing.Description = desc
	existing.Permissions = perms
	existing.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	return existing, nil
}

// DeleteRole soft-deletes a merchant role that has no employees assigned.
func (s *Service) DeleteRole(ctx context.Context, merchantID, roleID int64) error {
	_, err := s.GetRole(ctx, merchantID, roleID)
	if err != nil {
		return err
	}

	var empCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM employees WHERE merchant_role_id = $1 AND deleted_at IS NULL`, roleID,
	).Scan(&empCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check role usage", err)
	}
	if empCount > 0 {
		return apperrors.NewForbiddenError("role is assigned to employees and cannot be deleted")
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE merchant_roles SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, roleID)
	if err != nil {
		return apperrors.NewInternalError("failed to delete role", err)
	}
	return nil
}

// GetAvailablePermissions returns the full list of assignable permissions for merchants.
func (s *Service) GetAvailablePermissions() []PermissionItem {
	return []PermissionItem{
		{Key: "pos:*", Name: "收银", Category: "收银管理", Description: "POS收银操作权限"},
		{Key: "order:view", Name: "订单查询", Category: "订单管理", Description: "查看订单列表与详情"},
		{Key: "order:manage", Name: "订单管理", Category: "订单管理", Description: "退款、修改订单状态"},
		{Key: "member:view", Name: "会员查看", Category: "会员管理", Description: "查看会员列表与详情"},
		{Key: "member:manage", Name: "会员管理", Category: "会员管理", Description: "创建、编辑会员信息"},
		{Key: "pet:view", Name: "宠物查看", Category: "宠物管理", Description: "查看宠物档案"},
		{Key: "pet:manage", Name: "宠物管理", Category: "宠物管理", Description: "创建、编辑、删除宠物档案"},
		{Key: "product:view", Name: "商品查看", Category: "商品管理", Description: "查看商品列表与详情"},
		{Key: "product:manage", Name: "商品管理", Category: "商品管理", Description: "创建、编辑、删除商品"},
		{Key: "inventory:view", Name: "库存查看", Category: "库存管理", Description: "查看库存信息"},
		{Key: "inventory:manage", Name: "库存管理", Category: "库存管理", Description: "库存调整、盘点"},
		{Key: "service:view", Name: "服务查看", Category: "服务管理", Description: "查看服务项目与分类"},
		{Key: "service:manage", Name: "服务管理", Category: "服务管理", Description: "创建、编辑、删除服务项目"},
		{Key: "appointment:view", Name: "预约查看", Category: "预约管理", Description: "查看预约列表"},
		{Key: "appointment:manage", Name: "预约管理", Category: "预约管理", Description: "创建、编辑、取消预约"},
		{Key: "employee:view", Name: "员工查看", Category: "员工管理", Description: "查看员工列表"},
		{Key: "employee:manage", Name: "员工管理", Category: "员工管理", Description: "创建、编辑、管理员工"},
		{Key: "role:manage", Name: "角色管理", Category: "权限管理", Description: "创建、编辑、删除角色并分配权限"},
		{Key: "report:view", Name: "报表查看", Category: "报表管理", Description: "查看经营报表与统计"},
		{Key: "settings:manage", Name: "店铺设置", Category: "系统设置", Description: "修改店铺信息与配置"},
	}
}

// AssignRole assigns a merchant role to an employee.
func (s *Service) AssignRole(ctx context.Context, merchantID, employeeID, roleID int64) error {
	if employeeID <= 0 || roleID <= 0 {
		return apperrors.NewValidationError("employee_id and role_id are required")
	}

	// Verify employee belongs to merchant.
	var empStatus string
	var empNo string
	err := s.db.QueryRowContext(ctx,
		`SELECT status, employee_no FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		employeeID, merchantID,
	).Scan(&empStatus, &empNo)
	if err == sql.ErrNoRows {
		return apperrors.NewNotFoundError("employee not found")
	}
	if err != nil {
		return apperrors.NewInternalError("failed to find employee", err)
	}

	// Verify role belongs to merchant.
	_, err = s.GetRole(ctx, merchantID, roleID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE employees SET merchant_role_id = $1, updated_at = NOW() WHERE id = $2 AND merchant_id = $3`,
		roleID, employeeID, merchantID)
	if err != nil {
		return apperrors.NewInternalError("failed to assign role", err)
	}

	// Also link platform_users to this employee.
	username := "e_" + itoa(int(merchantID)) + "_" + empNo
	_, _ = s.db.ExecContext(ctx,
		`UPDATE platform_users SET employee_id = $1, updated_at = NOW()
		 WHERE username = $2 AND merchant_id = $3 AND deleted_at IS NULL`,
		employeeID, username, merchantID,
	)

	return nil
}

// EmployeeAccount represents the platform account for an employee.
type EmployeeAccount struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateEmployeeAccount creates a platform_users account for an employee.
func (s *Service) CreateEmployeeAccount(ctx context.Context, merchantID, employeeID int64) (*EmployeeAccount, error) {
	var empNo string
	var empName string
	err := s.db.QueryRowContext(ctx,
		`SELECT employee_no, name FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		employeeID, merchantID,
	).Scan(&empNo, &empName)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("employee not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to find employee", err)
	}

	// Check if account already exists.
	username := "e_" + itoa(int(merchantID)) + "_" + empNo
	var existingID int64
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM platform_users WHERE username = $1 AND deleted_at IS NULL`,
		username,
	).Scan(&existingID)
	if err == nil {
		return nil, apperrors.NewConflictError("employee account already exists: " + username)
	}

	password := generatePassword(8)
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to hash password", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO platform_users (username, password_hash, display_name, role_id, merchant_id, employee_id, status)
		 VALUES ($1, $2, $3, NULL, $4, $5, 'active')`,
		username, string(hashed), empName, merchantID, employeeID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create employee account", err)
	}

	return &EmployeeAccount{Username: username, Password: password}, nil
}

// GetEmployeePermissions fetches the permissions for an employee's assigned role.
func (s *Service) GetEmployeePermissions(ctx context.Context, merchantID, employeeID int64) ([]string, error) {
	var permsStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT mr.permissions::text
		 FROM employees e
		 JOIN merchant_roles mr ON e.merchant_role_id = mr.id AND mr.deleted_at IS NULL
		 WHERE e.id = $1 AND e.merchant_id = $2 AND e.deleted_at IS NULL`,
		employeeID, merchantID,
	).Scan(&permsStr)
	if err == sql.ErrNoRows || !permsStr.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get employee permissions", err)
	}
	var perms []string
	json.Unmarshal([]byte(permsStr.String), &perms)
	return perms, nil
}

// DisableEmployeeAccount disables the platform_users account for an employee.
func (s *Service) DisableEmployeeAccount(ctx context.Context, merchantID, employeeID int64) error {
	var empNo string
	err := s.db.QueryRowContext(ctx,
		`SELECT employee_no FROM employees WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		employeeID, merchantID,
	).Scan(&empNo)
	if err == sql.ErrNoRows {
		return apperrors.NewNotFoundError("employee not found")
	}
	if err != nil {
		return apperrors.NewInternalError("failed to find employee", err)
	}

	username := "e_" + itoa(int(merchantID)) + "_" + empNo
	result, err := s.db.ExecContext(ctx,
		`UPDATE platform_users SET status = 'disabled', updated_at = NOW()
		 WHERE username = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		username, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to disable account", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.NewNotFoundError("employee has no platform account to disable")
	}

	return nil
}

func generatePassword(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
