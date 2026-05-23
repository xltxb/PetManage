package payable

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// PayableRecord represents an accounts payable record.
type PayableRecord struct {
	ID              int64      `json:"id"`
	MerchantID      int64      `json:"merchant_id"`
	SupplierID      int64      `json:"supplier_id"`
	SupplierName    string     `json:"supplier_name,omitempty"`
	PurchaseOrderID int64      `json:"purchase_order_id"`
	OrderNo         string     `json:"order_no,omitempty"`
	TotalCents      int        `json:"total_cents"`
	PaidCents       int        `json:"paid_cents"`
	Status          string     `json:"status"`
	DueDate         *string    `json:"due_date,omitempty"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Payments        []PaymentRecord `json:"payments,omitempty"`
}

// PaymentRecord represents a single payment against a payable.
type PaymentRecord struct {
	ID              int64     `json:"id"`
	MerchantID      int64     `json:"merchant_id"`
	SupplierID      int64     `json:"supplier_id"`
	PayableRecordID int64     `json:"payable_record_id"`
	AmountCents     int       `json:"amount_cents"`
	PaymentMethod   string    `json:"payment_method"`
	PaymentDate     string    `json:"payment_date"`
	ReferenceNo     string    `json:"reference_no"`
	Notes           string    `json:"notes"`
	CreatedBy       *int64    `json:"created_by,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RegisterPaymentRequest is the request body for registering a payment.
type RegisterPaymentRequest struct {
	AmountCents   int    `json:"amount_cents"`
	PaymentMethod string `json:"payment_method"`
	PaymentDate   string `json:"payment_date"`
	ReferenceNo   string `json:"reference_no"`
	Notes         string `json:"notes"`
}

// ListPayablesParams holds filter/pagination for listing payables.
type ListPayablesParams struct {
	SupplierID int64
	Status     string
	Page       int
	PageSize   int
}

// SupplierPayableSummary groups payable data by supplier.
type SupplierPayableSummary struct {
	SupplierID   int64  `json:"supplier_id"`
	SupplierName string `json:"supplier_name"`
	TotalCents   int    `json:"total_cents"`
	PaidCents    int    `json:"paid_cents"`
	UnpaidCents  int    `json:"unpaid_cents"`
	RecordCount  int    `json:"record_count"`
}

// ListPayablesResult wraps the payables list result.
type ListPayablesResult struct {
	Payables []PayableRecord `json:"payables"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// StatementRecord is a single line in a supplier statement.
type StatementRecord struct {
	Date        string `json:"date"`
	Type        string `json:"type"` // "purchase" or "payment"
	Reference   string `json:"reference"`
	Description string `json:"description"`
	AmountCents int    `json:"amount_cents"`
}

// Statement represents a supplier reconciliation statement.
type Statement struct {
	SupplierID     int64             `json:"supplier_id"`
	SupplierName   string            `json:"supplier_name"`
	StartDate      string            `json:"start_date"`
	EndDate        string            `json:"end_date"`
	OpeningBalance int               `json:"opening_balance_cents"`
	PeriodPurchases int              `json:"period_purchases_cents"`
	PeriodPayments int               `json:"period_payments_cents"`
	ClosingBalance int               `json:"closing_balance_cents"`
	Records        []StatementRecord `json:"records"`
}

// Service provides accounts payable operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new payable Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreatePayableRecord creates a payable record for a received purchase order.
func (s *Service) CreatePayableRecord(ctx context.Context, merchantID, supplierID, poID int64, totalCents int, orderNo string) (*PayableRecord, error) {
	// Check if already exists (idempotent)
	var existingID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM payable_records
		 WHERE purchase_order_id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		poID, merchantID,
	).Scan(&existingID)
	if err == nil {
		// Already exists, return existing
		return s.GetByID(ctx, existingID, merchantID)
	}
	if err != sql.ErrNoRows {
		return nil, apperrors.NewInternalError("failed to check existing payable", err)
	}

	pr := &PayableRecord{}
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO payable_records (merchant_id, supplier_id, purchase_order_id, total_cents, status)
		 VALUES ($1, $2, $3, $4, 'unpaid')
		 RETURNING id, merchant_id, supplier_id, purchase_order_id, total_cents, paid_cents, status, due_date, notes, created_at, updated_at`,
		merchantID, supplierID, poID, totalCents,
	).Scan(
		&pr.ID, &pr.MerchantID, &pr.SupplierID, &pr.PurchaseOrderID,
		&pr.TotalCents, &pr.PaidCents, &pr.Status, &pr.DueDate, &pr.Notes,
		&pr.CreatedAt, &pr.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create payable record", err)
	}
	pr.OrderNo = orderNo

	return pr, nil
}

// GetByID returns a single payable record with payments.
func (s *Service) GetByID(ctx context.Context, id, merchantID int64) (*PayableRecord, error) {
	pr := &PayableRecord{}
	err := s.db.QueryRowContext(ctx,
		`SELECT pr.id, pr.merchant_id, pr.supplier_id, s.name,
		 pr.purchase_order_id, po.order_no,
		 pr.total_cents, pr.paid_cents, pr.status, pr.due_date, pr.notes,
		 pr.created_at, pr.updated_at
		 FROM payable_records pr
		 LEFT JOIN suppliers s ON s.id = pr.supplier_id
		 LEFT JOIN purchase_orders po ON po.id = pr.purchase_order_id
		 WHERE pr.id = $1 AND pr.merchant_id = $2 AND pr.deleted_at IS NULL`,
		id, merchantID,
	).Scan(
		&pr.ID, &pr.MerchantID, &pr.SupplierID, &pr.SupplierName,
		&pr.PurchaseOrderID, &pr.OrderNo,
		&pr.TotalCents, &pr.PaidCents, &pr.Status, &pr.DueDate, &pr.Notes,
		&pr.CreatedAt, &pr.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("payable record not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get payable record", err)
	}

	payments, err := s.getPayments(ctx, id)
	if err != nil {
		return nil, err
	}
	pr.Payments = payments

	return pr, nil
}

func (s *Service) getPayments(ctx context.Context, payableID int64) ([]PaymentRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, supplier_id, payable_record_id, amount_cents,
		 payment_method, payment_date, reference_no, notes, created_by,
		 created_at, updated_at
		 FROM payment_records
		 WHERE payable_record_id = $1 AND deleted_at IS NULL
		 ORDER BY payment_date DESC, created_at DESC`,
		payableID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get payments", err)
	}
	defer rows.Close()

	payments := make([]PaymentRecord, 0)
	for rows.Next() {
		var p PaymentRecord
		var paymentDate time.Time
		if err := rows.Scan(
			&p.ID, &p.MerchantID, &p.SupplierID, &p.PayableRecordID,
			&p.AmountCents, &p.PaymentMethod, &paymentDate,
			&p.ReferenceNo, &p.Notes, &p.CreatedBy,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, apperrors.NewInternalError("failed to scan payment", err)
		}
		p.PaymentDate = paymentDate.Format("2006-01-02")
		payments = append(payments, p)
	}
	return payments, nil
}

// ListPayables returns payables with optional filtering and pagination.
func (s *Service) ListPayables(ctx context.Context, merchantID int64, params ListPayablesParams) (*ListPayablesResult, error) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	args := []interface{}{merchantID}
	argIdx := 2
	where := "WHERE pr.merchant_id = $1 AND pr.deleted_at IS NULL"

	if params.SupplierID > 0 {
		where += " AND pr.supplier_id = $" + strconv.Itoa(argIdx)
		args = append(args, params.SupplierID)
		argIdx++
	}
	if params.Status != "" {
		where += " AND pr.status = $" + strconv.Itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM payable_records pr `+where,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count payables", err)
	}

	offset := (page - 1) * pageSize
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2

	rows, err := s.db.QueryContext(ctx,
		`SELECT pr.id, pr.merchant_id, pr.supplier_id, s.name,
		 pr.purchase_order_id, po.order_no,
		 pr.total_cents, pr.paid_cents, pr.status, pr.due_date, pr.notes,
		 pr.created_at, pr.updated_at
		 FROM payable_records pr
		 LEFT JOIN suppliers s ON s.id = pr.supplier_id
		 LEFT JOIN purchase_orders po ON po.id = pr.purchase_order_id
		 `+where+
			` ORDER BY pr.created_at DESC LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx),
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list payables", err)
	}
	defer rows.Close()

	payables := make([]PayableRecord, 0)
	for rows.Next() {
		var pr PayableRecord
		if err := rows.Scan(
			&pr.ID, &pr.MerchantID, &pr.SupplierID, &pr.SupplierName,
			&pr.PurchaseOrderID, &pr.OrderNo,
			&pr.TotalCents, &pr.PaidCents, &pr.Status, &pr.DueDate, &pr.Notes,
			&pr.CreatedAt, &pr.UpdatedAt,
		); err != nil {
			return nil, apperrors.NewInternalError("failed to scan payable", err)
		}
		payables = append(payables, pr)
	}

	return &ListPayablesResult{
		Payables: payables,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListBySupplier returns payables grouped by supplier with summary data.
func (s *Service) ListBySupplier(ctx context.Context, merchantID int64) ([]SupplierPayableSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pr.supplier_id, s.name,
		 SUM(pr.total_cents) as total_cents,
		 SUM(pr.paid_cents) as paid_cents,
		 SUM(pr.total_cents - pr.paid_cents) as unpaid_cents,
		 COUNT(pr.id) as record_count
		 FROM payable_records pr
		 LEFT JOIN suppliers s ON s.id = pr.supplier_id
		 WHERE pr.merchant_id = $1 AND pr.deleted_at IS NULL
		   AND pr.status != 'paid'
		 GROUP BY pr.supplier_id, s.name
		 ORDER BY unpaid_cents DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list payables by supplier", err)
	}
	defer rows.Close()

	summaries := make([]SupplierPayableSummary, 0)
	for rows.Next() {
		var s SupplierPayableSummary
		if err := rows.Scan(&s.SupplierID, &s.SupplierName, &s.TotalCents, &s.PaidCents, &s.UnpaidCents, &s.RecordCount); err != nil {
			return nil, apperrors.NewInternalError("failed to scan supplier summary", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// RegisterPayment records a payment against a payable record.
func (s *Service) RegisterPayment(ctx context.Context, merchantID, userID int64, payableID int64, req RegisterPaymentRequest) (*PaymentRecord, error) {
	if req.AmountCents <= 0 {
		return nil, apperrors.NewValidationError("amount_cents must be positive")
	}
	if req.PaymentMethod == "" {
		req.PaymentMethod = "bank_transfer"
	}
	if req.PaymentDate == "" {
		req.PaymentDate = time.Now().Format("2006-01-02")
	}

	// Get the payable record to verify and calculate
	pr, err := s.GetByID(ctx, payableID, merchantID)
	if err != nil {
		return nil, err
	}

	if pr.Status == "paid" {
		return nil, apperrors.NewValidationError("payable record is already fully paid")
	}

	remaining := pr.TotalCents - pr.PaidCents
	if req.AmountCents > remaining {
		return nil, apperrors.NewValidationError("payment amount exceeds remaining balance")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Insert payment record
	var payment PaymentRecord
	var paymentDate time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO payment_records
		 (merchant_id, supplier_id, payable_record_id, amount_cents, payment_method, payment_date, reference_no, notes, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, merchant_id, supplier_id, payable_record_id, amount_cents,
		 payment_method, payment_date, reference_no, notes, created_by,
		 created_at, updated_at`,
		merchantID, pr.SupplierID, payableID, req.AmountCents,
		req.PaymentMethod, req.PaymentDate, req.ReferenceNo, req.Notes, userID,
	).Scan(
		&payment.ID, &payment.MerchantID, &payment.SupplierID, &payment.PayableRecordID,
		&payment.AmountCents, &payment.PaymentMethod, &paymentDate,
		&payment.ReferenceNo, &payment.Notes, &payment.CreatedBy,
		&payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create payment record", err)
	}
	payment.PaymentDate = paymentDate.Format("2006-01-02")

	// Update payable record
	newPaidCents := pr.PaidCents + req.AmountCents
	newStatus := "partial"
	if newPaidCents >= pr.TotalCents {
		newStatus = "paid"
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE payable_records SET paid_cents = $1, status = $2, updated_at = NOW()
		 WHERE id = $3 AND merchant_id = $4 AND deleted_at IS NULL`,
		newPaidCents, newStatus, payableID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update payable record", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	return &payment, nil
}

// GetStatement generates a supplier statement for a date range.
func (s *Service) GetStatement(ctx context.Context, merchantID, supplierID int64, startDate, endDate string) (*Statement, error) {
	// Get supplier name
	var supplierName string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM suppliers WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		supplierID, merchantID,
	).Scan(&supplierName)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("supplier not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get supplier", err)
	}

	// Opening balance: sum of (total_cents - paid_cents) for payables created before start_date
	var openingBalance int
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_cents - paid_cents), 0)
		 FROM payable_records
		 WHERE merchant_id = $1 AND supplier_id = $2
		   AND created_at < $3::TIMESTAMPTZ
		   AND deleted_at IS NULL`,
		merchantID, supplierID, startDate+" 00:00:00",
	).Scan(&openingBalance)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to calculate opening balance", err)
	}

	// Period purchases: payable records created within the date range
	var periodPurchases int
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_cents), 0)
		 FROM payable_records
		 WHERE merchant_id = $1 AND supplier_id = $2
		   AND created_at >= $3::TIMESTAMPTZ
		   AND created_at < ($4::DATE + INTERVAL '1 day')::TIMESTAMPTZ
		   AND deleted_at IS NULL`,
		merchantID, supplierID, startDate+" 00:00:00", endDate,
	).Scan(&periodPurchases)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to calculate period purchases", err)
	}

	// Period payments
	var periodPayments int
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(pr2.amount_cents), 0)
		 FROM payment_records pr2
		 JOIN payable_records pr ON pr.id = pr2.payable_record_id
		 WHERE pr2.merchant_id = $1 AND pr2.supplier_id = $2
		   AND pr2.payment_date >= $3::DATE
		   AND pr2.payment_date <= $4::DATE
		   AND pr2.deleted_at IS NULL`,
		merchantID, supplierID, startDate, endDate,
	).Scan(&periodPayments)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to calculate period payments", err)
	}

	closingBalance := openingBalance + periodPurchases - periodPayments

	// Detail records
	records := make([]StatementRecord, 0)

	// Purchase records in period
	purchaseRows, err := s.db.QueryContext(ctx,
		`SELECT pr.created_at::DATE, po.order_no, pr.total_cents::TEXT
		 FROM payable_records pr
		 JOIN purchase_orders po ON po.id = pr.purchase_order_id
		 WHERE pr.merchant_id = $1 AND pr.supplier_id = $2
		   AND pr.created_at >= $3::TIMESTAMPTZ
		   AND pr.created_at < ($4::DATE + INTERVAL '1 day')::TIMESTAMPTZ
		   AND pr.deleted_at IS NULL
		 ORDER BY pr.created_at`,
		merchantID, supplierID, startDate+" 00:00:00", endDate,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get purchase records", err)
	}
	defer purchaseRows.Close()
	for purchaseRows.Next() {
		var dateStr string
		var orderNo string
		var amountStr string
		if err := purchaseRows.Scan(&dateStr, &orderNo, &amountStr); err != nil {
			return nil, apperrors.NewInternalError("failed to scan purchase row", err)
		}
		amount, _ := strconv.Atoi(amountStr)
		records = append(records, StatementRecord{
			Date:        dateStr,
			Type:        "purchase",
			Reference:   orderNo,
			Description: "采购入库",
			AmountCents: amount,
		})
	}

	// Payment records in period
	paymentRows, err := s.db.QueryContext(ctx,
		`SELECT pr2.payment_date::TEXT, pr2.reference_no, pr2.payment_method, pr2.amount_cents
		 FROM payment_records pr2
		 WHERE pr2.merchant_id = $1 AND pr2.supplier_id = $2
		   AND pr2.payment_date >= $3::DATE
		   AND pr2.payment_date <= $4::DATE
		   AND pr2.deleted_at IS NULL
		 ORDER BY pr2.payment_date, pr2.created_at`,
		merchantID, supplierID, startDate, endDate,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get payment records", err)
	}
	defer paymentRows.Close()
	for paymentRows.Next() {
		var dateStr, refNo, method string
		var amountCents int
		if err := paymentRows.Scan(&dateStr, &refNo, &method, &amountCents); err != nil {
			return nil, apperrors.NewInternalError("failed to scan payment row", err)
		}
		records = append(records, StatementRecord{
			Date:        dateStr,
			Type:        "payment",
			Reference:   refNo,
			Description: "付款 (" + method + ")",
			AmountCents: amountCents,
		})
	}

	return &Statement{
		SupplierID:      supplierID,
		SupplierName:    supplierName,
		StartDate:       startDate,
		EndDate:         endDate,
		OpeningBalance:  openingBalance,
		PeriodPurchases: periodPurchases,
		PeriodPayments:  periodPayments,
		ClosingBalance:  closingBalance,
		Records:         records,
	}, nil
}

func centsToYuan(cents int) string {
	yuan := float64(cents) / 100.0
	if cents < 0 {
		return fmt.Sprintf("-%.2f", -yuan)
	}
	return fmt.Sprintf("%.2f", yuan)
}

// GenerateStatementPDF generates a PDF file for the given statement.
func (s *Service) GenerateStatementPDF(stmt *Statement) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(190, 10, "Supplier Statement / 供应商对账单", "", 1, "C", false, 0, "")

	// Supplier and period info
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(4)
	pdf.CellFormat(40, 6, "Supplier:", "", 0, "L", false, 0, "")
	pdf.CellFormat(60, 6, stmt.SupplierName, "", 0, "L", false, 0, "")
	pdf.CellFormat(30, 6, "Period:", "", 0, "L", false, 0, "")
	pdf.CellFormat(60, 6, stmt.StartDate+" to "+stmt.EndDate, "", 1, "L", false, 0, "")

	pdf.Ln(4)

	// Summary section
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(190, 8, "Summary", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)

	colWidths := []float64{40, 50, 50, 50}
	headers := []string{"Item", "Amount (Yuan)", "Item", "Amount (Yuan)"}
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	summaryRows := [][]string{
		{"Opening Balance", centsToYuan(stmt.OpeningBalance), "Period Purchases", centsToYuan(stmt.PeriodPurchases)},
		{"Period Payments", centsToYuan(stmt.PeriodPayments), "Closing Balance", centsToYuan(stmt.ClosingBalance)},
	}
	for _, row := range summaryRows {
		for i, cell := range row {
			pdf.CellFormat(colWidths[i], 7, cell, "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(6)

	// Detail records
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(190, 8, "Transaction Details", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)

	detailColWidths := []float64{28, 22, 50, 55, 35}
	detailHeaders := []string{"Date", "Type", "Reference", "Description", "Amount (Yuan)"}
	for i, h := range detailHeaders {
		pdf.CellFormat(detailColWidths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	for _, rec := range stmt.Records {
		typeLabel := "Purchase"
		if rec.Type == "payment" {
			typeLabel = "Payment"
		}
		row := []string{
			rec.Date,
			typeLabel,
			rec.Reference,
			rec.Description,
			centsToYuan(rec.AmountCents),
		}
		for i, cell := range row {
			align := "L"
			if i == 4 {
				align = "R"
			} else if i == 0 || i == 1 {
				align = "C"
			}
			pdf.CellFormat(detailColWidths[i], 6, cell, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(190, 5, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, apperrors.NewInternalError("failed to generate PDF", err)
	}

	return buf.Bytes(), nil
}
