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
