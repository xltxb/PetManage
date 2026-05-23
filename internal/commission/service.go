package commission

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// CommissionRule holds per-merchant commission rate configuration.
type CommissionRule struct {
	ID                     int64     `json:"id"`
	MerchantID             int64     `json:"merchant_id"`
	ProductCommissionRate  float64   `json:"product_commission_rate"`
	ServiceCommissionRate  float64   `json:"service_commission_rate"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// CommissionRecord represents a single commission earned.
type CommissionRecord struct {
	ID                   int64     `json:"id"`
	MerchantID           int64     `json:"merchant_id"`
	EmployeeID           int64     `json:"employee_id"`
	EmployeeName         string    `json:"employee_name"`
	EmployeeNo           string    `json:"employee_no"`
	OrderID              int64     `json:"order_id"`
	OrderItemID          int64     `json:"order_item_id"`
	ItemType             string    `json:"item_type"`
	OrderItemAmountCents int       `json:"order_item_amount_cents"`
	CommissionRate       float64   `json:"commission_rate"`
	CommissionCents      int       `json:"commission_cents"`
	Status               string    `json:"status"`
	RefundID             *int64    `json:"refund_id"`
	Notes                string    `json:"notes"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// AssignTechnicianRequest assigns a technician to an order item.
type AssignTechnicianRequest struct {
	OrderItemID int64 `json:"order_item_id"`
	EmployeeID  int64 `json:"employee_id"`
}

// CommissionSummary holds the monthly commission summary per employee.
type CommissionSummary struct {
	EmployeeID        int64   `json:"employee_id"`
	EmployeeName      string  `json:"employee_name"`
	EmployeeNo        string  `json:"employee_no"`
	ProductCommission float64 `json:"product_commission"`
	ServiceCommission float64 `json:"service_commission"`
	TotalCommission   float64 `json:"total_commission"`
	DeductedCommission float64 `json:"deducted_commission"`
	NetCommission     float64 `json:"net_commission"`
}

// UpdateRuleRequest updates commission rules.
type UpdateRuleRequest struct {
	ProductCommissionRate float64 `json:"product_commission_rate"`
	ServiceCommissionRate float64 `json:"service_commission_rate"`
}

// ListRecordsParams holds filters for listing commission records.
type ListRecordsParams struct {
	EmployeeID int64
	OrderID    int64
	ItemType   string
	Status     string
	StartDate  string
	EndDate    string
	Page       int
	PageSize   int
}

// ListRecordsResult wraps paginated commission record results.
type ListRecordsResult struct {
	Records  []CommissionRecord `json:"records"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// Service handles commission management.
type Service struct {
	db *sql.DB
}

// NewService creates a new commission Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const commissionRecordColumns = `cr.id, cr.merchant_id, cr.employee_id, e.name, e.employee_no,
	cr.order_id, cr.order_item_id, cr.item_type, cr.order_item_amount_cents,
	cr.commission_rate, cr.commission_cents, cr.status, cr.refund_id,
	cr.notes, cr.created_at, cr.updated_at`

func scanRecordRow(row *sql.Row) (*CommissionRecord, error) {
	r := &CommissionRecord{}
	var refundID sql.NullInt64
	var rateBytes []byte
	var notesNull sql.NullString
	err := row.Scan(
		&r.ID, &r.MerchantID, &r.EmployeeID, &r.EmployeeName, &r.EmployeeNo,
		&r.OrderID, &r.OrderItemID, &r.ItemType, &r.OrderItemAmountCents,
		&rateBytes, &r.CommissionCents, &r.Status, &refundID,
		&notesNull, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == nil && len(rateBytes) > 0 {
		r.CommissionRate, _ = strconv.ParseFloat(string(rateBytes), 64)
	}
	if refundID.Valid {
		r.RefundID = &refundID.Int64
	}
	if notesNull.Valid {
		r.Notes = notesNull.String
	}
	return r, err
}

func scanRecordRows(rows *sql.Rows) (*CommissionRecord, error) {
	r := &CommissionRecord{}
	var refundID sql.NullInt64
	var rateBytes []byte
	var notesNull sql.NullString
	err := rows.Scan(
		&r.ID, &r.MerchantID, &r.EmployeeID, &r.EmployeeName, &r.EmployeeNo,
		&r.OrderID, &r.OrderItemID, &r.ItemType, &r.OrderItemAmountCents,
		&rateBytes, &r.CommissionCents, &r.Status, &refundID,
		&notesNull, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == nil && len(rateBytes) > 0 {
		r.CommissionRate, _ = strconv.ParseFloat(string(rateBytes), 64)
	}
	if refundID.Valid {
		r.RefundID = &refundID.Int64
	}
	if notesNull.Valid {
		r.Notes = notesNull.String
	}
	return r, err
}

// ensureRules returns the commission rules for a merchant, creating defaults if needed.
func (s *Service) ensureRules(ctx context.Context, merchantID int64) (*CommissionRule, error) {
	rule := &CommissionRule{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, product_commission_rate::float8, service_commission_rate::float8, created_at, updated_at
		 FROM commission_rules WHERE merchant_id = $1`,
		merchantID,
	).Scan(
		&rule.ID, &rule.MerchantID, &rule.ProductCommissionRate, &rule.ServiceCommissionRate,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		err = s.db.QueryRowContext(ctx,
			`INSERT INTO commission_rules (merchant_id, product_commission_rate, service_commission_rate)
			 VALUES ($1, 5.00, 30.00)
			 RETURNING id, merchant_id, product_commission_rate::float8, service_commission_rate::float8, created_at, updated_at`,
			merchantID,
		).Scan(
			&rule.ID, &rule.MerchantID, &rule.ProductCommissionRate, &rule.ServiceCommissionRate,
			&rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to create default commission rules", err)
		}
		return rule, nil
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get commission rules", err)
	}
	return rule, nil
}

// GetRules returns the commission rules for a merchant.
func (s *Service) GetRules(ctx context.Context, merchantID int64) (*CommissionRule, error) {
	return s.ensureRules(ctx, merchantID)
}

// UpdateRules updates commission rates for a merchant.
func (s *Service) UpdateRules(ctx context.Context, merchantID int64, req UpdateRuleRequest) (*CommissionRule, error) {
	if req.ProductCommissionRate < 0 || req.ProductCommissionRate > 100 {
		return nil, apperrors.NewValidationError("product commission rate must be between 0 and 100")
	}
	if req.ServiceCommissionRate < 0 || req.ServiceCommissionRate > 100 {
		return nil, apperrors.NewValidationError("service commission rate must be between 0 and 100")
	}

	rule := &CommissionRule{}
	err := s.db.QueryRowContext(ctx,
		`UPDATE commission_rules
		 SET product_commission_rate = $1, service_commission_rate = $2, updated_at = NOW()
		 WHERE merchant_id = $3
		 RETURNING id, merchant_id, product_commission_rate::float8, service_commission_rate::float8, created_at, updated_at`,
		req.ProductCommissionRate, req.ServiceCommissionRate, merchantID,
	).Scan(
		&rule.ID, &rule.MerchantID, &rule.ProductCommissionRate, &rule.ServiceCommissionRate,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update commission rules", err)
	}
	return rule, nil
}

// AssignTechnician assigns an employee to an order item and calculates commission.
func (s *Service) AssignTechnician(ctx context.Context, merchantID int64, req AssignTechnicianRequest) (*CommissionRecord, error) {
	// Verify the order item belongs to this merchant via order.
	var itemType string
	var orderID int64
	var orderStatus string
	var orderItemAmountCents int
	var productID sql.NullInt64
	var serviceItemID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT oi.id, oi.order_id, oi.price_cents, oi.product_id, oi.service_item_id, o.status
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 WHERE oi.id = $1 AND o.merchant_id = $2`,
		req.OrderItemID, merchantID,
	).Scan(&req.OrderItemID, &orderID, &orderItemAmountCents, &productID, &serviceItemID, &orderStatus)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("order item not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get order item", err)
	}

	if orderStatus != "completed" {
		return nil, apperrors.NewValidationError("commission can only be assigned for completed orders")
	}

	if serviceItemID.Valid {
		itemType = "service"
	} else {
		itemType = "product"
	}

	// Verify employee belongs to this merchant.
	var empID int64
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM employees WHERE id = $1 AND merchant_id = $2 AND status = 'active' AND deleted_at IS NULL`,
		req.EmployeeID, merchantID,
	).Scan(&empID)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("employee not found or not active")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify employee", err)
	}

	// Get commission rules.
	rules, err := s.ensureRules(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	var rate float64
	if itemType == "service" {
		rate = rules.ServiceCommissionRate
	} else {
		rate = rules.ProductCommissionRate
	}

	commissionCents := int(math.Round(float64(orderItemAmountCents) * rate / 100.0))

	// Check if there's already a commission record for this order item (upsert).
	var existingID int64
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM commission_records WHERE order_item_id = $1 AND status = 'confirmed'`,
		req.OrderItemID,
	).Scan(&existingID)
	if err == nil {
		// Update existing.
		_, err = s.db.ExecContext(ctx,
			`UPDATE commission_records
			 SET employee_id = $1, commission_rate = $2, commission_cents = $3, updated_at = NOW()
			 WHERE id = $4`,
			req.EmployeeID, rate, commissionCents, existingID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update commission record", err)
		}
		// Fetch updated record with join.
		rec := &CommissionRecord{}
		var refundID sql.NullInt64
		var rateBytes []byte
		var notesNull sql.NullString
		err = s.db.QueryRowContext(ctx,
			`SELECT `+commissionRecordColumns+`
			 FROM commission_records cr
			 JOIN employees e ON e.id = cr.employee_id
			 WHERE cr.id = $1`,
			existingID,
		).Scan(
			&rec.ID, &rec.MerchantID, &rec.EmployeeID, &rec.EmployeeName, &rec.EmployeeNo,
			&rec.OrderID, &rec.OrderItemID, &rec.ItemType, &rec.OrderItemAmountCents,
			&rateBytes, &rec.CommissionCents, &rec.Status, &refundID,
			&notesNull, &rec.CreatedAt, &rec.UpdatedAt,
		)
		if err == nil && len(rateBytes) > 0 {
			rec.CommissionRate, _ = strconv.ParseFloat(string(rateBytes), 64)
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to get updated commission record", err)
		}
		if refundID.Valid {
			rec.RefundID = &refundID.Int64
		}
		if notesNull.Valid {
			rec.Notes = notesNull.String
		}
		return rec, nil
	}
	if err != sql.ErrNoRows {
		return nil, apperrors.NewInternalError("failed to check existing commission", err)
	}

	// Update order_item to track the technician.
	_, err = s.db.ExecContext(ctx,
		`UPDATE order_items SET employee_id = $1 WHERE id = $2`,
		req.EmployeeID, req.OrderItemID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update order item technician", err)
	}

	// Create commission record.
	var recID int64
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO commission_records
		 (merchant_id, employee_id, order_id, order_item_id, item_type, order_item_amount_cents, commission_rate, commission_cents)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		merchantID, req.EmployeeID, orderID, req.OrderItemID, itemType, orderItemAmountCents, rate, commissionCents,
	).Scan(&recID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create commission record", err)
	}

	// Fetch the full record with employee name join.
	rec := &CommissionRecord{}
	var refundID sql.NullInt64
	var rateBytes []byte
	var notesNull sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT `+commissionRecordColumns+`
		 FROM commission_records cr
		 JOIN employees e ON e.id = cr.employee_id
		 WHERE cr.id = $1`,
		recID,
	).Scan(
		&rec.ID, &rec.MerchantID, &rec.EmployeeID, &rec.EmployeeName, &rec.EmployeeNo,
		&rec.OrderID, &rec.OrderItemID, &rec.ItemType, &rec.OrderItemAmountCents,
		&rateBytes, &rec.CommissionCents, &rec.Status, &refundID,
		&notesNull, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err == nil && len(rateBytes) > 0 {
		rec.CommissionRate, _ = strconv.ParseFloat(string(rateBytes), 64)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get created commission record", err)
	}
	if refundID.Valid {
		rec.RefundID = &refundID.Int64
	}
	if notesNull.Valid {
		rec.Notes = notesNull.String
	}
	return rec, nil
}

// ListRecords returns a filtered, paginated list of commission records.
func (s *Service) ListRecords(ctx context.Context, merchantID int64, params ListRecordsParams) (*ListRecordsResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	args := []interface{}{merchantID}
	argIdx := 2
	where := "WHERE cr.merchant_id = $1"

	if params.EmployeeID > 0 {
		where += " AND cr.employee_id = $" + itoa(argIdx)
		args = append(args, params.EmployeeID)
		argIdx++
	}
	if params.OrderID > 0 {
		where += " AND cr.order_id = $" + itoa(argIdx)
		args = append(args, params.OrderID)
		argIdx++
	}
	if params.ItemType != "" {
		where += " AND cr.item_type = $" + itoa(argIdx)
		args = append(args, params.ItemType)
		argIdx++
	}
	if params.Status != "" {
		where += " AND cr.status = $" + itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.StartDate != "" {
		where += " AND cr.created_at >= $" + itoa(argIdx)
		args = append(args, params.StartDate)
		argIdx++
	}
	if params.EndDate != "" {
		where += " AND cr.created_at <= $" + itoa(argIdx) + "::date + interval '1 day'"
		args = append(args, params.EndDate)
		argIdx++
	}

	// Count.
	var total int
	ctQuery := `SELECT COUNT(*) FROM commission_records cr JOIN employees e ON e.id = cr.employee_id ` + where
	ctArgs := make([]interface{}, len(args))
	copy(ctArgs, args)
	if err := s.db.QueryRowContext(ctx, ctQuery, ctArgs...).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("failed to count commission records", err)
	}

	offset := (params.Page - 1) * params.PageSize
	query := `SELECT ` + commissionRecordColumns + `
		FROM commission_records cr
		JOIN employees e ON e.id = cr.employee_id
		` + where + `
		ORDER BY cr.created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query commission records", err)
	}
	defer rows.Close()

	var records []CommissionRecord
	for rows.Next() {
		r, err := scanRecordRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan commission record", err)
		}
		records = append(records, *r)
	}
	if records == nil {
		records = []CommissionRecord{}
	}

	return &ListRecordsResult{
		Records:  records,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetMonthlySummary returns commission summary per employee for a given month.
func (s *Service) GetMonthlySummary(ctx context.Context, merchantID int64, year int, month int) ([]CommissionSummary, error) {
	query := `
		SELECT
			cr.employee_id,
			e.name,
			e.employee_no,
			COALESCE(SUM(CASE WHEN cr.item_type = 'product' AND cr.status = 'confirmed' THEN cr.commission_cents END), 0)::numeric / 100.0 AS product_commission,
			COALESCE(SUM(CASE WHEN cr.item_type = 'service' AND cr.status = 'confirmed' THEN cr.commission_cents END), 0)::numeric / 100.0 AS service_commission,
			COALESCE(SUM(CASE WHEN cr.status = 'confirmed' THEN cr.commission_cents END), 0)::numeric / 100.0 AS total_commission,
			COALESCE(SUM(CASE WHEN cr.status = 'deducted' THEN cr.commission_cents END), 0)::numeric / 100.0 AS deducted_commission,
			COALESCE(SUM(cr.commission_cents), 0)::numeric / 100.0 AS net_commission
		FROM commission_records cr
		JOIN employees e ON e.id = cr.employee_id
		WHERE cr.merchant_id = $1
		  AND EXTRACT(YEAR FROM cr.created_at) = $2
		  AND EXTRACT(MONTH FROM cr.created_at) = $3
		GROUP BY cr.employee_id, e.name, e.employee_no
		ORDER BY net_commission DESC`

	rows, err := s.db.QueryContext(ctx, query, merchantID, year, month)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query commission summary", err)
	}
	defer rows.Close()

	var summaries []CommissionSummary
	for rows.Next() {
		var s CommissionSummary
		if err := rows.Scan(&s.EmployeeID, &s.EmployeeName, &s.EmployeeNo,
			&s.ProductCommission, &s.ServiceCommission, &s.TotalCommission,
			&s.DeductedCommission, &s.NetCommission); err != nil {
			return nil, apperrors.NewInternalError("failed to scan summary row", err)
		}
		// Net = confirmed - deducted
		s.NetCommission = s.TotalCommission - s.DeductedCommission
		summaries = append(summaries, s)
	}
	if summaries == nil {
		summaries = []CommissionSummary{}
	}

	return summaries, nil
}

// DeductCommission marks commission records as deducted due to a refund.
func (s *Service) DeductCommission(ctx context.Context, merchantID int64, orderItemID int64, refundID int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE commission_records
		 SET status = 'deducted', refund_id = $1, notes = 'Refund deduction', updated_at = NOW()
		 WHERE order_item_id = $2 AND merchant_id = $3 AND status = 'confirmed'`,
		refundID, orderItemID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to deduct commission", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return apperrors.NewNotFoundError("no confirmed commission record found for deduction")
	}
	return nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
