package servicerecord

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// ServiceRecord represents a completed service archived to pet and member profiles.
type ServiceRecord struct {
	ID            int64           `json:"id"`
	MerchantID    int64           `json:"merchant_id"`
	MemberID      int64           `json:"member_id"`
	PetID         int64           `json:"pet_id"`
	AppointmentID *int64          `json:"appointment_id"`
	ServiceItemID int64           `json:"service_item_id"`
	EmployeeID    int64           `json:"employee_id"`
	ServiceDate   time.Time       `json:"service_date"`
	MaterialsUsed json.RawMessage `json:"materials_used"`
	Notes         string          `json:"notes"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// ServiceRecordDetail includes joined entity names.
type ServiceRecordDetail struct {
	ServiceRecord
	MemberName      string `json:"member_name"`
	PetName         string `json:"pet_name"`
	ServiceItemName string `json:"service_item_name"`
	EmployeeName    string `json:"employee_name"`
	Rating          *int   `json:"rating"`
	EvalContent     string `json:"eval_content"`
	EvalID          *int64 `json:"eval_id"`
}

// Evaluation represents a customer evaluation of a completed service.
type Evaluation struct {
	ID              int64     `json:"id"`
	MerchantID      int64     `json:"merchant_id"`
	MemberID        int64     `json:"member_id"`
	PetID           int64     `json:"pet_id"`
	AppointmentID   *int64    `json:"appointment_id"`
	ServiceRecordID int64     `json:"service_record_id"`
	EmployeeID      int64     `json:"employee_id"`
	Rating          int       `json:"rating"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// EvaluateRequest is the request body for submitting a service evaluation.
type EvaluateRequest struct {
	Rating  int    `json:"rating"`
	Content string `json:"content"`
}

// ListParams holds optional filters and pagination.
type ListParams struct {
	Page     int
	PageSize int
}

// ListResult wraps the service records list with pagination info.
type ListResult struct {
	Records  []ServiceRecordDetail `json:"records"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// Service provides service record archiving operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new service record Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const recordColumns = `sr.id, sr.merchant_id, sr.member_id, sr.pet_id, sr.appointment_id, sr.service_item_id, sr.employee_id, sr.service_date, sr.materials_used, sr.notes, sr.created_at, sr.updated_at`

func scanRecord(row *sql.Row) (*ServiceRecord, error) {
	r := &ServiceRecord{}
	err := row.Scan(
		&r.ID, &r.MerchantID, &r.MemberID, &r.PetID, &r.AppointmentID,
		&r.ServiceItemID, &r.EmployeeID, &r.ServiceDate,
		&r.MaterialsUsed, &r.Notes, &r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// ArchiveOnComplete is called when an appointment is completed.
// It creates a pet service record and deducts materials from inventory.
// Returns error only to match the appointment service interface.
func (s *Service) ArchiveOnComplete(ctx context.Context, merchantID, appointmentID int64) error {
	_, err := s.archiveOnComplete(ctx, merchantID, appointmentID)
	return err
}

// archiveOnComplete is the internal implementation that also returns the created record.
func (s *Service) archiveOnComplete(ctx context.Context, merchantID, appointmentID int64) (*ServiceRecord, error) {
	// Query appointment details.
	var memberID, petID, serviceItemID, employeeID int64
	var serviceDate time.Time
	var remark string
	var materialsStr string

	err := s.db.QueryRowContext(ctx,
		`SELECT a.member_id, a.pet_id, a.service_item_id, a.employee_id, a.appointment_time, a.remark,
		        COALESCE(si.materials, '')
		 FROM appointments a
		 JOIN service_items si ON si.id = a.service_item_id
		 WHERE a.id = $1 AND a.merchant_id = $2 AND a.deleted_at IS NULL`,
		appointmentID, merchantID,
	).Scan(&memberID, &petID, &serviceItemID, &employeeID, &serviceDate, &remark, &materialsStr)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("appointment not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query appointment", err)
	}

	// Parse materials info.
	materialsUsed := json.RawMessage("{}")
	if strings.TrimSpace(materialsStr) != "" {
		// Try to parse as JSON, fall back to wrapping as text.
		if json.Valid([]byte(materialsStr)) {
			materialsUsed = json.RawMessage(materialsStr)
		} else {
			wrapped, _ := json.Marshal(map[string]string{"description": materialsStr})
			materialsUsed = wrapped
		}
	}

	// Deduct consumable materials from inventory if materials are structured.
	// Materials format: {"products": [{"product_id": N, "quantity": N}, ...]}
	if strings.TrimSpace(materialsStr) != "" {
		var matData struct {
			Products []struct {
				ProductID int64 `json:"product_id"`
				Quantity  int   `json:"quantity"`
			} `json:"products"`
		}
		if json.Unmarshal([]byte(materialsStr), &matData) == nil && len(matData.Products) > 0 {
			for _, p := range matData.Products {
				if p.ProductID > 0 && p.Quantity > 0 {
					s.db.ExecContext(ctx,
						`UPDATE products SET stock = stock - $1, updated_at = NOW()
						 WHERE id = $2 AND merchant_id = $3 AND stock >= $1`,
						p.Quantity, p.ProductID, merchantID,
					)
					// Record stock flow for material consumption.
					var productName string
					s.db.QueryRowContext(ctx,
						`SELECT name FROM products WHERE id = $1`, p.ProductID,
					).Scan(&productName)
					s.db.ExecContext(ctx,
						`INSERT INTO stock_flows (merchant_id, product_id, type, quantity_change, notes, created_at)
						 VALUES ($1, $2, 'outbound', $3, $4, NOW())`,
						merchantID, p.ProductID, -p.Quantity,
						"service material consumption: appointment #"+strconv.FormatInt(appointmentID, 10),
					)
				}
			}
		}
	}

	// Create the service record.
	record, err := scanRecord(s.db.QueryRowContext(ctx,
		`INSERT INTO pet_service_records (merchant_id, member_id, pet_id, appointment_id, service_item_id, employee_id, service_date, materials_used, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, merchant_id, member_id, pet_id, appointment_id, service_item_id, employee_id, service_date, materials_used, notes, created_at, updated_at`,
		merchantID, memberID, petID, appointmentID, serviceItemID, employeeID, serviceDate, materialsUsed, remark,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create service record", err)
	}

	return record, nil
}

// GetByID returns a single service record with joined entity names and evaluation.
func (s *Service) GetByID(ctx context.Context, merchantID, recordID int64) (*ServiceRecordDetail, error) {
	d := &ServiceRecordDetail{}
	err := s.db.QueryRowContext(ctx,
		`SELECT `+recordColumns+`,
		 COALESCE(m.name, ''), COALESCE(p.name, ''),
		 COALESCE(si.name, ''), COALESCE(e.name, ''),
		 se.rating, COALESCE(se.content, ''), se.id
		 FROM pet_service_records sr
		 LEFT JOIN members m ON m.id = sr.member_id
		 LEFT JOIN pets p ON p.id = sr.pet_id
		 LEFT JOIN service_items si ON si.id = sr.service_item_id
		 LEFT JOIN employees e ON e.id = sr.employee_id
		 LEFT JOIN service_evaluations se ON se.service_record_id = sr.id
		 WHERE sr.id = $1 AND sr.merchant_id = $2 AND sr.deleted_at IS NULL`,
		recordID, merchantID,
	).Scan(
		&d.ID, &d.MerchantID, &d.MemberID, &d.PetID, &d.AppointmentID,
		&d.ServiceItemID, &d.EmployeeID, &d.ServiceDate,
		&d.MaterialsUsed, &d.Notes, &d.CreatedAt, &d.UpdatedAt,
		&d.MemberName, &d.PetName, &d.ServiceItemName, &d.EmployeeName,
		&d.Rating, &d.EvalContent, &d.EvalID,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("service record not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get service record", err)
	}
	return d, nil
}

// ListByPet returns service records for a pet.
func (s *Service) ListByPet(ctx context.Context, merchantID, petID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pet_service_records sr
		 WHERE sr.pet_id = $1 AND sr.merchant_id = $2 AND sr.deleted_at IS NULL`,
		petID, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count service records", err)
	}

	offset := (params.Page - 1) * params.PageSize
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+recordColumns+`,
		 COALESCE(m.name, ''), COALESCE(p.name, ''),
		 COALESCE(si.name, ''), COALESCE(e.name, ''),
		 se.rating, COALESCE(se.content, ''), se.id
		 FROM pet_service_records sr
		 LEFT JOIN members m ON m.id = sr.member_id
		 LEFT JOIN pets p ON p.id = sr.pet_id
		 LEFT JOIN service_items si ON si.id = sr.service_item_id
		 LEFT JOIN employees e ON e.id = sr.employee_id
		 LEFT JOIN service_evaluations se ON se.service_record_id = sr.id
		 WHERE sr.pet_id = $1 AND sr.merchant_id = $2 AND sr.deleted_at IS NULL
		 ORDER BY sr.service_date DESC LIMIT $3 OFFSET $4`,
		petID, merchantID, params.PageSize, offset,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list service records", err)
	}
	defer rows.Close()

	var records []ServiceRecordDetail
	for rows.Next() {
		d := ServiceRecordDetail{}
		if err := rows.Scan(
			&d.ID, &d.MerchantID, &d.MemberID, &d.PetID, &d.AppointmentID,
			&d.ServiceItemID, &d.EmployeeID, &d.ServiceDate,
			&d.MaterialsUsed, &d.Notes, &d.CreatedAt, &d.UpdatedAt,
			&d.MemberName, &d.PetName, &d.ServiceItemName, &d.EmployeeName,
			&d.Rating, &d.EvalContent, &d.EvalID,
		); err != nil {
			return nil, apperrors.NewInternalError("failed to scan service record", err)
		}
		records = append(records, d)
	}
	if records == nil {
		records = []ServiceRecordDetail{}
	}

	return &ListResult{
		Records:  records,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// ListByMember returns service records for a member (across all their pets).
func (s *Service) ListByMember(ctx context.Context, merchantID, memberID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pet_service_records sr
		 WHERE sr.member_id = $1 AND sr.merchant_id = $2 AND sr.deleted_at IS NULL`,
		memberID, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count service records", err)
	}

	offset := (params.Page - 1) * params.PageSize
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+recordColumns+`,
		 COALESCE(m.name, ''), COALESCE(p.name, ''),
		 COALESCE(si.name, ''), COALESCE(e.name, ''),
		 se.rating, COALESCE(se.content, ''), se.id
		 FROM pet_service_records sr
		 LEFT JOIN members m ON m.id = sr.member_id
		 LEFT JOIN pets p ON p.id = sr.pet_id
		 LEFT JOIN service_items si ON si.id = sr.service_item_id
		 LEFT JOIN employees e ON e.id = sr.employee_id
		 LEFT JOIN service_evaluations se ON se.service_record_id = sr.id
		 WHERE sr.member_id = $1 AND sr.merchant_id = $2 AND sr.deleted_at IS NULL
		 ORDER BY sr.service_date DESC LIMIT $3 OFFSET $4`,
		memberID, merchantID, params.PageSize, offset,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list service records", err)
	}
	defer rows.Close()

	var records []ServiceRecordDetail
	for rows.Next() {
		d := ServiceRecordDetail{}
		if err := rows.Scan(
			&d.ID, &d.MerchantID, &d.MemberID, &d.PetID, &d.AppointmentID,
			&d.ServiceItemID, &d.EmployeeID, &d.ServiceDate,
			&d.MaterialsUsed, &d.Notes, &d.CreatedAt, &d.UpdatedAt,
			&d.MemberName, &d.PetName, &d.ServiceItemName, &d.EmployeeName,
			&d.Rating, &d.EvalContent, &d.EvalID,
		); err != nil {
			return nil, apperrors.NewInternalError("failed to scan service record", err)
		}
		records = append(records, d)
	}
	if records == nil {
		records = []ServiceRecordDetail{}
	}

	return &ListResult{
		Records:  records,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// SubmitEvaluation creates or updates an evaluation for a service record.
func (s *Service) SubmitEvaluation(ctx context.Context, merchantID, recordID int64, req EvaluateRequest) (*Evaluation, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, apperrors.NewValidationError("rating must be between 1 and 5")
	}

	// Verify service record exists and belongs to this merchant.
	var memberID, petID, employeeID int64
	var appointmentID *int64
	err := s.db.QueryRowContext(ctx,
		`SELECT member_id, pet_id, employee_id, appointment_id FROM pet_service_records
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		recordID, merchantID,
	).Scan(&memberID, &petID, &employeeID, &appointmentID)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("service record not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify service record", err)
	}

	// Upsert evaluation (one evaluation per service record).
	eval := &Evaluation{}
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO service_evaluations (merchant_id, member_id, pet_id, appointment_id, service_record_id, employee_id, rating, content)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (service_record_id) DO UPDATE
		 SET rating = $7, content = $8, updated_at = NOW()
		 RETURNING id, merchant_id, member_id, pet_id, appointment_id, service_record_id, employee_id, rating, content, created_at, updated_at`,
		merchantID, memberID, petID, appointmentID, recordID, employeeID, req.Rating, req.Content,
	).Scan(
		&eval.ID, &eval.MerchantID, &eval.MemberID, &eval.PetID, &eval.AppointmentID,
		&eval.ServiceRecordID, &eval.EmployeeID, &eval.Rating, &eval.Content,
		&eval.CreatedAt, &eval.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to submit evaluation", err)
	}

	return eval, nil
}

// GetEmployeeRating returns average rating for an employee.
func (s *Service) GetEmployeeRating(ctx context.Context, merchantID, employeeID int64) (float64, int, error) {
	var avgRating sql.NullFloat64
	var totalCount int
	err := s.db.QueryRowContext(ctx,
		`SELECT AVG(rating)::numeric(3,1), COUNT(*) FROM service_evaluations
		 WHERE merchant_id = $1 AND employee_id = $2`,
		merchantID, employeeID,
	).Scan(&avgRating, &totalCount)
	if err != nil {
		return 0, 0, apperrors.NewInternalError("failed to get employee rating", err)
	}
	if !avgRating.Valid {
		return 0, 0, nil
	}
	return avgRating.Float64, totalCount, nil
}
