package merchant

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/xltxb/PetManage/pkg/apperrors"
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
	var contactEmail, licenseNumber, legalPerson, contactPhone, address sql.NullString
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, license_number, legal_person, contact_phone, contact_email, address, status, created_at, updated_at
		 FROM merchants WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&detail.ID, &detail.Name, &licenseNumber, &legalPerson,
		&contactPhone, &contactEmail, &address, &detail.Status, &createdAt, &updatedAt)

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
	detail.CreatedAt = createdAt.Format(time.RFC3339)
	detail.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &detail, nil
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
