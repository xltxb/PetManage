package openplatform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Available API permissions for open platform developers.
var AvailablePermissions = []PermissionItem{
	{Code: "shop:read", Name: "店铺信息", Description: "获取店铺名称、地址、营业时间、Logo"},
	{Code: "products:read", Name: "商品查询", Description: "查询商品列表和详情"},
	{Code: "services:read", Name: "服务查询", Description: "查询服务项目和套餐"},
	{Code: "breeds:read", Name: "品种查询", Description: "查询宠物品种库"},
	{Code: "members:read", Name: "会员查询", Description: "查询会员信息、等级、积分"},
	{Code: "members:write", Name: "会员管理", Description: "注册会员、更新会员信息"},
	{Code: "pets:read", Name: "宠物查询", Description: "查询会员宠物档案"},
	{Code: "pets:write", Name: "宠物管理", Description: "添加、编辑宠物档案"},
	{Code: "bookings:read", Name: "预约查询", Description: "查询预约列表和技师排班"},
	{Code: "bookings:write", Name: "预约管理", Description: "创建、修改、取消预约"},
	{Code: "orders:read", Name: "订单查询", Description: "查询订单列表和详情"},
	{Code: "orders:write", Name: "订单管理", Description: "创建订单、发起退款"},
	{Code: "coupons:read", Name: "优惠券查询", Description: "查询优惠券和活动"},
	{Code: "coupons:write", Name: "优惠券核销", Description: "优惠券领取、核销"},
}

// Default permissions assigned on approval.
var defaultPermissions = []string{
	"shop:read", "products:read", "services:read", "breeds:read",
}

type PermissionItem struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Application struct {
	ID            int64           `json:"id"`
	CompanyName   string          `json:"company_name"`
	ContactPerson string          `json:"contact_person"`
	ContactPhone  string          `json:"contact_phone"`
	ContactEmail  string          `json:"contact_email"`
	UsagePurpose  string          `json:"usage_purpose"`
	CallbackURL   string          `json:"callback_url"`
	Status        string          `json:"status"`
	AppKey        string          `json:"app_key,omitempty"`
	AppSecret     string          `json:"app_secret,omitempty"`
	MerchantID    *int64          `json:"merchant_id,omitempty"`
	Permissions   json.RawMessage `json:"permissions"`
	ReviewRemark  string          `json:"review_remark,omitempty"`
	ReviewedBy    *int64          `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type ApplyRequest struct {
	CompanyName   string `json:"company_name"`
	ContactPerson string `json:"contact_person"`
	ContactPhone  string `json:"contact_phone"`
	ContactEmail  string `json:"contact_email"`
	UsagePurpose  string `json:"usage_purpose"`
	CallbackURL   string `json:"callback_url"`
}

type RejectRequest struct {
	Remark string `json:"remark"`
}

type ApproveResponse struct {
	Application
	AppKey    string `json:"app_key"`
	AppSecret string `json:"app_secret"`
}

type RequestPermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

type UpdatePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

type ListResult struct {
	Applications []Application `json:"applications"`
	Total        int64         `json:"total"`
	Page         int           `json:"page"`
	PageSize     int           `json:"page_size"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// generateAppKey generates a random AppKey starting with "AK".
func generateAppKey() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "AK" + hex.EncodeToString(b), nil
}

// generateAppSecret generates a random AppSecret starting with "AS".
func generateAppSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "AS" + hex.EncodeToString(b), nil
}

// Apply submits a new developer onboarding application.
func (s *Service) Apply(ctx context.Context, req ApplyRequest) (*Application, error) {
	if req.CompanyName == "" || req.ContactPerson == "" || req.ContactPhone == "" ||
		req.ContactEmail == "" || req.UsagePurpose == "" {
		missing := []string{}
		if req.CompanyName == "" {
			missing = append(missing, "company_name")
		}
		if req.ContactPerson == "" {
			missing = append(missing, "contact_person")
		}
		if req.ContactPhone == "" {
			missing = append(missing, "contact_phone")
		}
		if req.ContactEmail == "" {
			missing = append(missing, "contact_email")
		}
		if req.UsagePurpose == "" {
			missing = append(missing, "usage_purpose")
		}
		return nil, apperrors.NewValidationError(
			fmt.Sprintf("missing required fields: %v", missing),
		)
	}

	defaultPerms, _ := json.Marshal([]string{})
	var app Application
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO open_developers (company_name, contact_person, contact_phone, contact_email, usage_purpose, callback_url, permissions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, company_name, contact_person, contact_phone, contact_email, usage_purpose, callback_url, status, permissions, review_remark, created_at, updated_at`,
		req.CompanyName, req.ContactPerson, req.ContactPhone, req.ContactEmail,
		req.UsagePurpose, req.CallbackURL, defaultPerms,
	).Scan(&app.ID, &app.CompanyName, &app.ContactPerson, &app.ContactPhone,
		&app.ContactEmail, &app.UsagePurpose, &app.CallbackURL, &app.Status,
		&app.Permissions, &app.ReviewRemark, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to submit application", err)
	}
	return &app, nil
}

// GetByID retrieves a developer application by ID.
// Only shows credentials if the application is approved.
func (s *Service) GetByID(ctx context.Context, id int64) (*Application, error) {
	var app Application
	var appKey, appSecret sql.NullString
	var reviewedBy, merchantID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, app_key, app_secret, permissions,
		 merchant_id, review_remark, reviewed_by, reviewed_at, created_at, updated_at
		 FROM open_developers WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&app.ID, &app.CompanyName, &app.ContactPerson, &app.ContactPhone,
		&app.ContactEmail, &app.UsagePurpose, &app.CallbackURL, &app.Status,
		&appKey, &appSecret, &app.Permissions, &merchantID, &app.ReviewRemark,
		&reviewedBy, &app.ReviewedAt, &app.CreatedAt, &app.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("developer application not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query application", err)
	}
	if appKey.Valid {
		app.AppKey = appKey.String
	}
	if appSecret.Valid {
		app.AppSecret = appSecret.String
	}
	if reviewedBy.Valid {
		app.ReviewedBy = &reviewedBy.Int64
	}
	if merchantID.Valid {
		app.MerchantID = &merchantID.Int64
	}
	return &app, nil
}

// ListPending lists all pending developer applications (platform admin).
func (s *Service) ListPending(ctx context.Context, page, pageSize int) (*ListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM open_developers WHERE status = 'pending' AND deleted_at IS NULL`,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count applications", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, permissions, review_remark, created_at, updated_at
		 FROM open_developers WHERE status = 'pending' AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query applications", err)
	}
	defer rows.Close()

	apps := []Application{}
	for rows.Next() {
		var app Application
		if err := rows.Scan(&app.ID, &app.CompanyName, &app.ContactPerson,
			&app.ContactPhone, &app.ContactEmail, &app.UsagePurpose, &app.CallbackURL,
			&app.Status, &app.Permissions, &app.ReviewRemark, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan application", err)
		}
		apps = append(apps, app)
	}
	return &ListResult{Applications: apps, Total: total, Page: page, PageSize: pageSize}, nil
}

// Approve approves a developer application, generating AppKey and AppSecret.
// If merchantID is non-zero, it associates the developer with the given merchant.
func (s *Service) Approve(ctx context.Context, id int64, reviewerID int64, merchantID int64) (*ApproveResponse, error) {
	app, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != "pending" {
		return nil, apperrors.NewValidationError("only pending applications can be approved")
	}

	appKey, err := generateAppKey()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate app key", err)
	}
	appSecret, err := generateAppSecret()
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate app secret", err)
	}

	defaultPerms, _ := json.Marshal(defaultPermissions)
	now := time.Now()

	var result Application
	var reviewedBy, scannedMerchantID sql.NullInt64
	var midParam interface{}
	if merchantID > 0 {
		midParam = merchantID
	} else {
		midParam = nil
	}
	err = s.db.QueryRowContext(ctx,
		`UPDATE open_developers SET status = 'approved', app_key = $1, app_secret = $2,
		 permissions = $3, reviewed_by = $4, reviewed_at = $5, review_remark = '',
		 merchant_id = $7, updated_at = $5
		 WHERE id = $6 AND deleted_at IS NULL
		 RETURNING id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, app_key, app_secret, permissions,
		 merchant_id, review_remark, reviewed_by, reviewed_at, created_at, updated_at`,
		appKey, appSecret, defaultPerms, reviewerID, now, id, midParam,
	).Scan(&result.ID, &result.CompanyName, &result.ContactPerson, &result.ContactPhone,
		&result.ContactEmail, &result.UsagePurpose, &result.CallbackURL, &result.Status,
		&result.AppKey, &result.AppSecret, &result.Permissions, &scannedMerchantID, &result.ReviewRemark,
		&reviewedBy, &result.ReviewedAt, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to approve application", err)
	}
	if reviewedBy.Valid {
		result.ReviewedBy = &reviewedBy.Int64
	}
	if scannedMerchantID.Valid {
		result.MerchantID = &scannedMerchantID.Int64
	}
	return &ApproveResponse{
		Application: result,
		AppKey:      appKey,
		AppSecret:   appSecret,
	}, nil
}

// Reject rejects a developer application with a remark.
func (s *Service) Reject(ctx context.Context, id int64, reviewerID int64, remark string) (*Application, error) {
	if remark == "" {
		return nil, apperrors.NewValidationError("review remark is required for rejection")
	}

	app, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != "pending" {
		return nil, apperrors.NewValidationError("only pending applications can be rejected")
	}

	now := time.Now()
	var updated Application
	var reviewedBy sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		`UPDATE open_developers SET status = 'rejected', review_remark = $1,
		 reviewed_by = $2, reviewed_at = $3, updated_at = $3
		 WHERE id = $4 AND deleted_at IS NULL
		 RETURNING id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, permissions, review_remark,
		 reviewed_by, reviewed_at, created_at, updated_at`,
		remark, reviewerID, now, id,
	).Scan(&updated.ID, &updated.CompanyName, &updated.ContactPerson, &updated.ContactPhone,
		&updated.ContactEmail, &updated.UsagePurpose, &updated.CallbackURL, &updated.Status,
		&updated.Permissions, &updated.ReviewRemark, &reviewedBy, &updated.ReviewedAt,
		&updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to reject application", err)
	}
	if reviewedBy.Valid {
		updated.ReviewedBy = &reviewedBy.Int64
	}
	return &updated, nil
}

// Resubmit allows a rejected developer to re-submit their application.
func (s *Service) Resubmit(ctx context.Context, id int64, req ApplyRequest) (*Application, error) {
	app, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != "rejected" {
		return nil, apperrors.NewValidationError("only rejected applications can be resubmitted")
	}
	if req.CompanyName == "" || req.ContactPerson == "" || req.ContactPhone == "" ||
		req.ContactEmail == "" || req.UsagePurpose == "" {
		missing := []string{}
		if req.CompanyName == "" {
			missing = append(missing, "company_name")
		}
		if req.ContactPerson == "" {
			missing = append(missing, "contact_person")
		}
		if req.ContactPhone == "" {
			missing = append(missing, "contact_phone")
		}
		if req.ContactEmail == "" {
			missing = append(missing, "contact_email")
		}
		if req.UsagePurpose == "" {
			missing = append(missing, "usage_purpose")
		}
		return nil, apperrors.NewValidationError(
			fmt.Sprintf("missing required fields: %v", missing),
		)
	}

	defaultPerms, _ := json.Marshal([]string{})
	var updated Application
	err = s.db.QueryRowContext(ctx,
		`UPDATE open_developers SET company_name = $1, contact_person = $2, contact_phone = $3,
		 contact_email = $4, usage_purpose = $5, callback_url = $6, status = 'pending',
		 review_remark = '', reviewed_by = NULL, reviewed_at = NULL, permissions = $7,
		 app_key = NULL, app_secret = NULL, updated_at = NOW()
		 WHERE id = $8 AND deleted_at IS NULL
		 RETURNING id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, permissions, review_remark, created_at, updated_at`,
		req.CompanyName, req.ContactPerson, req.ContactPhone, req.ContactEmail,
		req.UsagePurpose, req.CallbackURL, defaultPerms, id,
	).Scan(&updated.ID, &updated.CompanyName, &updated.ContactPerson, &updated.ContactPhone,
		&updated.ContactEmail, &updated.UsagePurpose, &updated.CallbackURL, &updated.Status,
		&updated.Permissions, &updated.ReviewRemark, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to resubmit application", err)
	}
	return &updated, nil
}

// List lists all developer applications (platform admin, with filters).
func (s *Service) List(ctx context.Context, status string, page, pageSize int) (*ListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	if status != "" {
		whereClause += " AND status = $1"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM open_developers %s", whereClause)
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count applications", err)
	}

	argIdx := len(args) + 1
	query := fmt.Sprintf(
		`SELECT id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, permissions, review_remark, created_at, updated_at
		 FROM open_developers %s
		 ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query applications", err)
	}
	defer rows.Close()

	apps := []Application{}
	for rows.Next() {
		var app Application
		if err := rows.Scan(&app.ID, &app.CompanyName, &app.ContactPerson,
			&app.ContactPhone, &app.ContactEmail, &app.UsagePurpose, &app.CallbackURL,
			&app.Status, &app.Permissions, &app.ReviewRemark, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan application", err)
		}
		apps = append(apps, app)
	}
	return &ListResult{Applications: apps, Total: total, Page: page, PageSize: pageSize}, nil
}

// UpdatePermissions allows platform admin to update a developer's API permissions.
func (s *Service) UpdatePermissions(ctx context.Context, id int64, permissions []string) (*Application, error) {
	app, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != "approved" {
		return nil, apperrors.NewValidationError("can only update permissions for approved applications")
	}

	permsJSON, _ := json.Marshal(permissions)
	var updated Application
	var appKey, appSecret sql.NullString
	var reviewedBy sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		`UPDATE open_developers SET permissions = $1, updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL
		 RETURNING id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, app_key, app_secret, permissions,
		 review_remark, reviewed_by, reviewed_at, created_at, updated_at`,
		permsJSON, id,
	).Scan(&updated.ID, &updated.CompanyName, &updated.ContactPerson, &updated.ContactPhone,
		&updated.ContactEmail, &updated.UsagePurpose, &updated.CallbackURL, &updated.Status,
		&appKey, &appSecret, &updated.Permissions, &updated.ReviewRemark,
		&reviewedBy, &updated.ReviewedAt, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update permissions", err)
	}
	if appKey.Valid {
		updated.AppKey = appKey.String
	}
	if appSecret.Valid {
		updated.AppSecret = appSecret.String
	}
	if reviewedBy.Valid {
		updated.ReviewedBy = &reviewedBy.Int64
	}
	return &updated, nil
}

// RequestPermissions allows a developer to request additional API permissions.
func (s *Service) RequestPermissions(ctx context.Context, id int64, permissions []string) (*Application, error) {
	app, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != "approved" {
		return nil, apperrors.NewValidationError("only approved developers can request additional permissions")
	}

	// Merge existing permissions with new ones (no duplicates).
	existing := []string{}
	if len(app.Permissions) > 0 {
		json.Unmarshal(app.Permissions, &existing)
	}
	seen := map[string]bool{}
	for _, p := range existing {
		seen[p] = true
	}
	for _, p := range permissions {
		if !seen[p] {
			seen[p] = true
		}
	}
	merged := []string{}
	for p := range seen {
		merged = append(merged, p)
	}

	permsJSON, _ := json.Marshal(merged)
	var updated Application
	var appKey, appSecret sql.NullString
	var reviewedBy sql.NullInt64
	err = s.db.QueryRowContext(ctx,
		`UPDATE open_developers SET permissions = $1, updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL
		 RETURNING id, company_name, contact_person, contact_phone, contact_email,
		 usage_purpose, callback_url, status, app_key, app_secret, permissions,
		 review_remark, reviewed_by, reviewed_at, created_at, updated_at`,
		permsJSON, id,
	).Scan(&updated.ID, &updated.CompanyName, &updated.ContactPerson, &updated.ContactPhone,
		&updated.ContactEmail, &updated.UsagePurpose, &updated.CallbackURL, &updated.Status,
		&appKey, &appSecret, &updated.Permissions, &updated.ReviewRemark,
		&reviewedBy, &updated.ReviewedAt, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to request permissions", err)
	}
	if appKey.Valid {
		updated.AppKey = appKey.String
	}
	if appSecret.Valid {
		updated.AppSecret = appSecret.String
	}
	if reviewedBy.Valid {
		updated.ReviewedBy = &reviewedBy.Int64
	}
	return &updated, nil
}

// GetAvailablePermissions returns the list of all available API permissions.
func (s *Service) GetAvailablePermissions() []PermissionItem {
	return AvailablePermissions
}


