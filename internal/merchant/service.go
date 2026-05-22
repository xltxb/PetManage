package merchant

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/xltxb/PetManage/pkg/apperrors"
	"golang.org/x/crypto/bcrypt"
)

// Service handles merchant application operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new merchant Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ApplyRequest is the merchant application request body.
type ApplyRequest struct {
	Name         string `json:"name"`
	LicenseNumber string `json:"license_number"`
	LegalPerson  string `json:"legal_person"`
	ContactPhone string `json:"contact_phone"`
	ContactEmail string `json:"contact_email,omitempty"`
	Address      string `json:"address"`
}

// ApplicationResponse is returned after a successful application.
type ApplicationResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// ApplicationDetail is the full application detail.
type ApplicationDetail struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LicenseNumber string `json:"license_number"`
	LegalPerson   string `json:"legal_person"`
	ContactPhone  string `json:"contact_phone"`
	ContactEmail  string `json:"contact_email"`
	Address       string `json:"address"`
	Status        string `json:"status"`
	ReviewRemark  string `json:"review_remark,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Apply submits a new merchant application and returns the application ID with pending status.
func (s *Service) Apply(ctx context.Context, req ApplyRequest) (*ApplicationResponse, error) {
	// Validate required fields.
	missing := validateRequired(req)
	if len(missing) > 0 {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "missing required fields: " + strings.Join(missing, ", "),
		}
	}

	var id int64
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO merchants (name, license_number, legal_person, contact_phone, contact_email, address, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		 RETURNING id, created_at`,
		req.Name, req.LicenseNumber, req.LegalPerson,
		req.ContactPhone, req.ContactEmail, req.Address,
	).Scan(&id, &createdAt)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Constraint == "idx_merchants_license_number" {
				return nil, &apperrors.AppError{
					Code:    apperrors.CodeDuplicateLicense,
					Message: "license number already exists: " + req.LicenseNumber,
				}
			}
			if pqErr.Constraint == "idx_merchants_contact_phone" {
				return nil, &apperrors.AppError{
					Code:    apperrors.CodeConflict,
					Message: "contact phone already exists: " + req.ContactPhone,
				}
			}
		}
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create application",
			Err:     err,
		}
	}

	return &ApplicationResponse{
		ID:        id,
		Name:      req.Name,
		Status:    "pending",
		Message:   "application submitted successfully, pending review",
		CreatedAt: createdAt.Format(time.RFC3339),
	}, nil
}

// GetByID retrieves a merchant application by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*ApplicationDetail, error) {
	var detail ApplicationDetail
	var contactEmail, licenseNumber, legalPerson, contactPhone, address, reviewRemark sql.NullString
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, license_number, legal_person, contact_phone, contact_email, address, status, review_remark, created_at, updated_at
		 FROM merchants WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&detail.ID, &detail.Name, &licenseNumber, &legalPerson,
		&contactPhone, &contactEmail, &address, &detail.Status, &reviewRemark, &createdAt, &updatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "application not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to retrieve application",
			Err:     err,
		}
	}

	detail.LicenseNumber = licenseNumber.String
	detail.LegalPerson = legalPerson.String
	detail.ContactPhone = contactPhone.String
	detail.ContactEmail = contactEmail.String
	detail.Address = address.String
	detail.ReviewRemark = reviewRemark.String
	detail.CreatedAt = createdAt.Format(time.RFC3339)
	detail.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &detail, nil
}

// PendingApplication is a summary row for the pending review list.
type PendingApplication struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// ReviewResponse is returned after an approve or reject action.
type ReviewResponse struct {
	Message       string                `json:"message"`
	Status        string                `json:"status"`
	MerchantAdmin *MerchantAdminAccount `json:"merchant_admin,omitempty"`
}

// MerchantAdminAccount holds auto-generated credentials for a new merchant admin.
type MerchantAdminAccount struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ListPending returns all merchant applications with status 'pending', ordered by created_at.
func (s *Service) ListPending(ctx context.Context) ([]PendingApplication, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM merchants
		 WHERE status = 'pending' AND deleted_at IS NULL
		 ORDER BY created_at ASC`)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to list pending applications",
			Err:     err,
		}
	}
	defer rows.Close()

	var apps []PendingApplication
	for rows.Next() {
		var app PendingApplication
		var createdAt time.Time
		if err := rows.Scan(&app.ID, &app.Name, &createdAt); err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan pending application",
				Err:     err,
			}
		}
		app.CreatedAt = createdAt.Format(time.RFC3339)
		apps = append(apps, app)
	}

	if apps == nil {
		apps = []PendingApplication{}
	}
	return apps, nil
}

// Approve approves a pending merchant application and creates a merchant admin account.
func (s *Service) Approve(ctx context.Context, id int64, reviewerID int64) (*ReviewResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	// Lock the merchant row and check status.
	var status string
	var licenseNumber string
	err = tx.QueryRowContext(ctx,
		`SELECT status, license_number FROM merchants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&status, &licenseNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "application not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up application",
			Err:     err,
		}
	}
	if status != "pending" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "application status is '" + status + "', expected 'pending'",
		}
	}

	// Update merchant status to approved.
	_, err = tx.ExecContext(ctx,
		`UPDATE merchants SET status = 'approved', reviewed_by = $1, reviewed_at = NOW(), updated_at = NOW() WHERE id = $2`,
		reviewerID, id,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to approve application",
			Err:     err,
		}
	}

	// Create merchant admin account.
	username := "m_" + strings.ReplaceAll(strings.ToLower(licenseNumber), "-", "_")
	password := generatePassword(12)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to generate password",
			Err:     err,
		}
	}

	// Get merchant_admin role id.
	var roleID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM platform_roles WHERE code = 'merchant_admin' AND deleted_at IS NULL`,
	).Scan(&roleID)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "merchant_admin role not found",
			Err:     err,
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO platform_users (username, password_hash, display_name, role_id, merchant_id, status, must_change_password)
		 VALUES ($1, $2, (SELECT legal_person FROM merchants WHERE id = $3), $4, $3, 'active', true)`,
		username, string(hashedPassword), id, roleID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create merchant admin account",
			Err:     err,
		}
	}

	// Record operation log.
	if err := s.recordOperationTx(ctx, tx, reviewerID, "approve_merchant", "merchant", id, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit approval",
			Err:     err,
		}
	}

	return &ReviewResponse{
		Message: "application approved",
		Status:  "approved",
		MerchantAdmin: &MerchantAdminAccount{
			Username: username,
			Password: password,
		},
	}, nil
}

// Reject rejects a pending merchant application with a reason.
func (s *Service) Reject(ctx context.Context, id int64, reason string, reviewerID int64) (*ReviewResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM merchants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "application not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up application",
			Err:     err,
		}
	}
	if status != "pending" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "application status is '" + status + "', expected 'pending'",
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE merchants SET status = 'rejected', review_remark = $1, reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW() WHERE id = $3`,
		reason, reviewerID, id,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to reject application",
			Err:     err,
		}
	}

	detail := map[string]string{"reason": reason}
	detailJSON, _ := json.Marshal(detail)
	if err := s.recordOperationTx(ctx, tx, reviewerID, "reject_merchant", "merchant", id, detailJSON); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit rejection",
			Err:     err,
		}
	}

	return &ReviewResponse{
		Message: "application rejected",
		Status:  "rejected",
	}, nil
}

// Resubmit updates a rejected application and resets its status to pending.
func (s *Service) Resubmit(ctx context.Context, id int64, req ApplyRequest) (*ApplicationResponse, error) {
	missing := validateRequired(req)
	if len(missing) > 0 {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "missing required fields: " + strings.Join(missing, ", "),
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM merchants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "application not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up application",
			Err:     err,
		}
	}
	if status != "rejected" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "only rejected applications can be resubmitted, current status: " + status,
		}
	}

	var createdAt time.Time
	err = tx.QueryRowContext(ctx,
		`UPDATE merchants SET name = $1, license_number = $2, legal_person = $3, contact_phone = $4,
		 contact_email = $5, address = $6, status = 'pending', review_remark = NULL,
		 reviewed_by = NULL, reviewed_at = NULL, updated_at = NOW()
		 WHERE id = $7
		 RETURNING created_at`,
		req.Name, req.LicenseNumber, req.LegalPerson, req.ContactPhone,
		req.ContactEmail, req.Address, id,
	).Scan(&createdAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Constraint == "idx_merchants_license_number" {
				return nil, &apperrors.AppError{
					Code:    apperrors.CodeDuplicateLicense,
					Message: "license number already exists: " + req.LicenseNumber,
				}
			}
			if pqErr.Constraint == "idx_merchants_contact_phone" {
				return nil, &apperrors.AppError{
					Code:    apperrors.CodeConflict,
					Message: "contact phone already exists: " + req.ContactPhone,
				}
			}
		}
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to resubmit application",
			Err:     err,
		}
	}

	if err := s.recordOperationTx(ctx, tx, 0, "resubmit_merchant", "merchant", id, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit resubmission",
			Err:     err,
		}
	}

	return &ApplicationResponse{
		ID:        id,
		Name:      req.Name,
		Status:    "pending",
		Message:   "application resubmitted successfully, pending review",
		CreatedAt: createdAt.Format(time.RFC3339),
	}, nil
}

// recordOperation inserts an operation log (standalone, outside a transaction).
func (s *Service) recordOperation(ctx context.Context, userID int64, action, targetType string, targetID int64, detail json.RawMessage) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, action, targetType, targetID, detail,
	)
	return err
}

// recordOperationTx inserts an operation log within a transaction.
func (s *Service) recordOperationTx(ctx context.Context, tx *sql.Tx, userID int64, action, targetType string, targetID int64, detail json.RawMessage) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, action, targetType, targetID, detail,
	)
	return err
}

// generatePassword creates a random alphanumeric password of the given length.
func generatePassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// MerchantSummary is a compact row for the merchant list.
type MerchantSummary struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	LicenseNumber string `json:"license_number"`
	LegalPerson  string `json:"legal_person"`
	ContactPhone string `json:"contact_phone"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

// ListResponse is the paginated merchant list response.
type ListResponse struct {
	Merchants []MerchantSummary `json:"merchants"`
	Total     int               `json:"total"`
	Page      int               `json:"page"`
	PageSize  int               `json:"page_size"`
}

// ListParams holds the query parameters for listing merchants.
type ListParams struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

// List returns a paginated list of merchants with optional keyword search and status filter.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResponse, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	keyword := "%" + params.Keyword + "%"
	args := []interface{}{keyword, params.Status}

	// Count total matching rows.
	var total int
	countQuery := `SELECT COUNT(*) FROM merchants WHERE deleted_at IS NULL`
	countQuery += ` AND ($1 = '%%' OR name ILIKE $1)`
	if params.Status != "" {
		countQuery += ` AND status = $2`
	} else {
		countQuery += ` AND ($2 = '' OR status = $2)`
	}
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to count merchants",
			Err:     err,
		}
	}

	// Fetch paginated results.
	offset := (params.Page - 1) * params.PageSize
	dataQuery := `SELECT id, name, license_number, legal_person, contact_phone, status, created_at
		FROM merchants WHERE deleted_at IS NULL`
	dataQuery += ` AND ($1 = '%%' OR name ILIKE $1)`
	if params.Status != "" {
		dataQuery += ` AND status = $2`
	} else {
		dataQuery += ` AND ($2 = '' OR status = $2)`
	}
	dataQuery += ` ORDER BY created_at DESC LIMIT $3 OFFSET $4`

	rows, err := s.db.QueryContext(ctx, dataQuery, keyword, params.Status, params.PageSize, offset)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to list merchants",
			Err:     err,
		}
	}
	defer rows.Close()

	var merchants []MerchantSummary
	for rows.Next() {
		var m MerchantSummary
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.Name, &m.LicenseNumber, &m.LegalPerson,
			&m.ContactPhone, &m.Status, &createdAt); err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan merchant row",
				Err:     err,
			}
		}
		m.CreatedAt = createdAt.Format(time.RFC3339)
		merchants = append(merchants, m)
	}

	if merchants == nil {
		merchants = []MerchantSummary{}
	}

	return &ListResponse{
		Merchants: merchants,
		Total:     total,
		Page:      params.Page,
		PageSize:  params.PageSize,
	}, nil
}

// StatusControlRequest is the request body for freeze/unfreeze/close operations.
type StatusControlRequest struct {
	Reason string `json:"reason"`
}

// StatusControlResponse is returned after a status control operation.
type StatusControlResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// OperationLogEntry is a single operation log record.
type OperationLogEntry struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Detail     string `json:"detail,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Freeze sets a merchant's status to 'frozen' and records the operation.
func (s *Service) Freeze(ctx context.Context, merchantID int64, reason string, operatorID int64) (*StatusControlResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM merchants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		merchantID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "merchant not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up merchant",
			Err:     err,
		}
	}
	if status != "approved" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "only approved merchants can be frozen, current status: " + status,
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE merchants SET status = 'frozen', updated_at = NOW() WHERE id = $1`,
		merchantID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to freeze merchant",
			Err:     err,
		}
	}

	detail := map[string]string{"reason": reason}
	detailJSON, _ := json.Marshal(detail)
	if err := s.recordOperationTx(ctx, tx, operatorID, "freeze_merchant", "merchant", merchantID, detailJSON); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit freeze",
			Err:     err,
		}
	}

	return &StatusControlResponse{
		Message: "merchant frozen successfully",
		Status:  "frozen",
	}, nil
}

// Unfreeze restores a merchant's status from 'frozen' to 'approved' and records the operation.
func (s *Service) Unfreeze(ctx context.Context, merchantID int64, reason string, operatorID int64) (*StatusControlResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM merchants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		merchantID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "merchant not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up merchant",
			Err:     err,
		}
	}
	if status != "frozen" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "only frozen merchants can be unfrozen, current status: " + status,
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE merchants SET status = 'approved', updated_at = NOW() WHERE id = $1`,
		merchantID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to unfreeze merchant",
			Err:     err,
		}
	}

	detail := map[string]string{"reason": reason}
	detailJSON, _ := json.Marshal(detail)
	if err := s.recordOperationTx(ctx, tx, operatorID, "unfreeze_merchant", "merchant", merchantID, detailJSON); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit unfreeze",
			Err:     err,
		}
	}

	return &StatusControlResponse{
		Message: "merchant unfrozen successfully",
		Status:  "approved",
	}, nil
}

// Close permanently closes a merchant and records the operation.
func (s *Service) Close(ctx context.Context, merchantID int64, reason string, operatorID int64) (*StatusControlResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM merchants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		merchantID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "merchant not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up merchant",
			Err:     err,
		}
	}
	if status != "approved" && status != "frozen" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "only approved or frozen merchants can be closed, current status: " + status,
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE merchants SET status = 'closed', updated_at = NOW() WHERE id = $1`,
		merchantID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to close merchant",
			Err:     err,
		}
	}

	detail := map[string]string{"reason": reason}
	detailJSON, _ := json.Marshal(detail)
	if err := s.recordOperationTx(ctx, tx, operatorID, "close_merchant", "merchant", merchantID, detailJSON); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit close",
			Err:     err,
		}
	}

	return &StatusControlResponse{
		Message: "merchant permanently closed",
		Status:  "closed",
	}, nil
}

// GetOperationLogs returns operation logs for a specific merchant.
func (s *Service) GetOperationLogs(ctx context.Context, merchantID int64) ([]OperationLogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, action, target_type, target_id, COALESCE(detail::text, ''), created_at
		 FROM operation_logs
		 WHERE target_type = 'merchant' AND target_id = $1
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query operation logs",
			Err:     err,
		}
	}
	defer rows.Close()

	var logs []OperationLogEntry
	for rows.Next() {
		var entry OperationLogEntry
		var createdAt time.Time
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.Action,
			&entry.TargetType, &entry.TargetID, &entry.Detail, &createdAt); err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan operation log",
				Err:     err,
			}
		}
		entry.CreatedAt = createdAt.Format(time.RFC3339)
		logs = append(logs, entry)
	}

	if logs == nil {
		logs = []OperationLogEntry{}
	}
	return logs, nil
}

func validateRequired(req ApplyRequest) []string {
	var missing []string
	if strings.TrimSpace(req.Name) == "" {
		missing = append(missing, "name")
	}
	if strings.TrimSpace(req.LicenseNumber) == "" {
		missing = append(missing, "license_number")
	}
	if strings.TrimSpace(req.LegalPerson) == "" {
		missing = append(missing, "legal_person")
	}
	if strings.TrimSpace(req.ContactPhone) == "" {
		missing = append(missing, "contact_phone")
	}
	if strings.TrimSpace(req.Address) == "" {
		missing = append(missing, "address")
	}
	return missing
}
