package purchase

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// PurchaseOrder represents a purchase order record.
type PurchaseOrder struct {
	ID           int64      `json:"id"`
	MerchantID   int64      `json:"merchant_id"`
	SupplierID   int64      `json:"supplier_id"`
	SupplierName string     `json:"supplier_name,omitempty"`
	OrderNo      string     `json:"order_no"`
	Status       string     `json:"status"`
	TotalCents   int        `json:"total_cents"`
	Notes        string     `json:"notes"`
	CreatedBy    *int64     `json:"created_by"`
	Items        []POItem   `json:"items,omitempty"`
	SubmittedAt  *time.Time `json:"submitted_at,omitempty"`
	ConfirmedAt  *time.Time `json:"confirmed_at,omitempty"`
	ReceivedAt   *time.Time `json:"received_at,omitempty"`
	VoidedAt     *time.Time `json:"voided_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// POItem represents a purchase order item.
type POItem struct {
	ID               int64     `json:"id"`
	PurchaseOrderID  int64     `json:"purchase_order_id"`
	ProductID        *int64    `json:"product_id"`
	ProductSkuID     *int64    `json:"product_sku_id"`
	ProductName      string    `json:"product_name"`
	Quantity         int       `json:"quantity"`
	UnitPriceCents   int       `json:"unit_price_cents"`
	ReceivedQuantity int       `json:"received_quantity"`
	TotalCents       int       `json:"total_cents"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateItem represents a single item in a purchase order creation/update request.
type CreateItem struct {
	ProductID      *int64 `json:"product_id"`
	ProductSkuID   *int64 `json:"product_sku_id"`
	ProductName    string `json:"product_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
}

// CreateRequest is the request body for creating a purchase order.
type CreateRequest struct {
	SupplierID int64        `json:"supplier_id"`
	Notes      string       `json:"notes"`
	Items      []CreateItem `json:"items"`
}

// UpdateRequest is the request body for updating a draft purchase order.
type UpdateRequest struct {
	SupplierID *int64       `json:"supplier_id"`
	Notes      *string      `json:"notes"`
	Items      []CreateItem `json:"items"`
}

// ListParams holds optional filters and pagination for listing purchase orders.
type ListParams struct {
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// ListResult wraps the purchase orders list with pagination info.
type ListResult struct {
	Orders   []PurchaseOrder `json:"orders"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// Service provides purchase order management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new purchase Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const poColumns = `po.id, po.merchant_id, po.supplier_id, po.order_no, po.status, po.total_cents, po.notes, po.created_by, po.submitted_at, po.confirmed_at, po.received_at, po.voided_at, po.created_at, po.updated_at`

const poItemColumns = `poi.id, poi.purchase_order_id, poi.product_id, poi.product_sku_id, poi.product_name, poi.quantity, poi.unit_price_cents, poi.received_quantity, poi.total_cents, poi.created_at, poi.updated_at`

func scanPORow(row *sql.Row) (*PurchaseOrder, error) {
	po := &PurchaseOrder{}
	err := row.Scan(
		&po.ID, &po.MerchantID, &po.SupplierID, &po.OrderNo, &po.Status, &po.TotalCents,
		&po.Notes, &po.CreatedBy, &po.SubmittedAt, &po.ConfirmedAt, &po.ReceivedAt,
		&po.VoidedAt, &po.CreatedAt, &po.UpdatedAt,
	)
	return po, err
}

func scanPORowWithSupplier(row *sql.Row) (*PurchaseOrder, error) {
	po := &PurchaseOrder{}
	err := row.Scan(
		&po.ID, &po.MerchantID, &po.SupplierID, &po.OrderNo, &po.Status, &po.TotalCents,
		&po.Notes, &po.CreatedBy, &po.SubmittedAt, &po.ConfirmedAt, &po.ReceivedAt,
		&po.VoidedAt, &po.CreatedAt, &po.UpdatedAt, &po.SupplierName,
	)
	return po, err
}

func scanPORows(rows *sql.Rows) (*PurchaseOrder, error) {
	po := &PurchaseOrder{}
	err := rows.Scan(
		&po.ID, &po.MerchantID, &po.SupplierID, &po.OrderNo, &po.Status, &po.TotalCents,
		&po.Notes, &po.CreatedBy, &po.SubmittedAt, &po.ConfirmedAt, &po.ReceivedAt,
		&po.VoidedAt, &po.CreatedAt, &po.UpdatedAt,
	)
	return po, err
}

func scanPORowsWithSupplier(rows *sql.Rows) (*PurchaseOrder, error) {
	po := &PurchaseOrder{}
	err := rows.Scan(
		&po.ID, &po.MerchantID, &po.SupplierID, &po.OrderNo, &po.Status, &po.TotalCents,
		&po.Notes, &po.CreatedBy, &po.SubmittedAt, &po.ConfirmedAt, &po.ReceivedAt,
		&po.VoidedAt, &po.CreatedAt, &po.UpdatedAt, &po.SupplierName,
	)
	return po, err
}

func scanPOItemRows(rows *sql.Rows) (*POItem, error) {
	item := &POItem{}
	err := rows.Scan(
		&item.ID, &item.PurchaseOrderID, &item.ProductID, &item.ProductSkuID,
		&item.ProductName, &item.Quantity, &item.UnitPriceCents, &item.ReceivedQuantity,
		&item.TotalCents, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

// generateOrderNo generates a unique purchase order number: PO+YYYYMMDD+4-digit serial.
func (s *Service) generateOrderNo(ctx context.Context) (string, error) {
	today := time.Now().Format("20060102")
	prefix := "PO" + today

	var maxSerial int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(CAST(SUBSTRING(order_no, 11) AS INTEGER)), 0)
		 FROM purchase_orders
		 WHERE order_no LIKE $1 AND deleted_at IS NULL`,
		prefix+"%",
	).Scan(&maxSerial)
	if err != nil {
		return "", err
	}

	serial := maxSerial + 1
	if serial > 9999 {
		serial = 1
	}
	return fmt.Sprintf("%s%04d", prefix, serial), nil
}

func (s *Service) validateMerchantSupplier(ctx context.Context, merchantID, supplierID int64) error {
	var status string
	err := s.db.QueryRowContext(ctx,
		`SELECT status FROM suppliers WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`,
		supplierID, merchantID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return apperrors.NewNotFoundError("supplier not found")
	}
	if err != nil {
		return apperrors.NewInternalError("failed to verify supplier", err)
	}
	if status != "active" {
		return apperrors.NewValidationError("supplier is not active")
	}
	return nil
}

// Create creates a new draft purchase order.
func (s *Service) Create(ctx context.Context, merchantID, userID int64, req CreateRequest) (*PurchaseOrder, error) {
	if req.SupplierID == 0 {
		return nil, apperrors.NewValidationError("supplier_id is required")
	}
	if len(req.Items) == 0 {
		return nil, apperrors.NewValidationError("at least one purchase item is required")
	}

	if err := s.validateMerchantSupplier(ctx, merchantID, req.SupplierID); err != nil {
		return nil, err
	}

	for i, item := range req.Items {
		if strings.TrimSpace(item.ProductName) == "" {
			return nil, apperrors.NewValidationError("item " + strconv.Itoa(i+1) + ": product_name is required")
		}
		if item.Quantity <= 0 {
			return nil, apperrors.NewValidationError("item " + strconv.Itoa(i+1) + ": quantity must be positive")
		}
		if item.UnitPriceCents <= 0 {
			return nil, apperrors.NewValidationError("item " + strconv.Itoa(i+1) + ": unit_price_cents must be positive")
		}
	}

	orderNo, err := s.generateOrderNo(ctx)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate order number", err)
	}

	totalCents := 0
	for _, item := range req.Items {
		totalCents += item.Quantity * item.UnitPriceCents
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	po, err := scanPORow(tx.QueryRowContext(ctx,
		`INSERT INTO purchase_orders AS po (merchant_id, supplier_id, order_no, status, total_cents, notes, created_by)
		 VALUES ($1, $2, $3, 'draft', $4, $5, $6)
		 RETURNING `+poColumns,
		merchantID, req.SupplierID, orderNo, totalCents, req.Notes, userID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create purchase order", err)
	}

	po.SupplierName = ""

	for _, item := range req.Items {
		var pid, skuID interface{}
		if item.ProductID != nil && *item.ProductID > 0 {
			pid = *item.ProductID
		} else {
			pid = nil
		}
		if item.ProductSkuID != nil && *item.ProductSkuID > 0 {
			skuID = *item.ProductSkuID
		} else {
			skuID = nil
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO purchase_order_items (purchase_order_id, product_id, product_sku_id, product_name, quantity, unit_price_cents, total_cents)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			po.ID, pid, skuID, item.ProductName, item.Quantity, item.UnitPriceCents, item.Quantity*item.UnitPriceCents,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to create purchase order item", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	items, err := s.getItems(ctx, po.ID)
	if err != nil {
		return nil, err
	}
	po.Items = items

	return po, nil
}

func (s *Service) getItems(ctx context.Context, poID int64) ([]POItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+poItemColumns+` FROM purchase_order_items poi
		 WHERE poi.purchase_order_id = $1 AND poi.deleted_at IS NULL
		 ORDER BY poi.id`,
		poID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get items", err)
	}
	defer rows.Close()

	items := make([]POItem, 0)
	for rows.Next() {
		item, err := scanPOItemRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan item", err)
		}
		items = append(items, *item)
	}
	return items, nil
}

// GetByID returns a purchase order with items and supplier name.
func (s *Service) GetByID(ctx context.Context, poID, merchantID int64) (*PurchaseOrder, error) {
	po, err := scanPORowWithSupplier(s.db.QueryRowContext(ctx,
		`SELECT `+poColumns+`, s.name as supplier_name
		 FROM purchase_orders po
		 LEFT JOIN suppliers s ON s.id = po.supplier_id
		 WHERE po.id = $1 AND po.merchant_id = $2 AND po.deleted_at IS NULL`,
		poID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("purchase order not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get purchase order", err)
	}

	items, err := s.getItems(ctx, poID)
	if err != nil {
		return nil, err
	}
	po.Items = items
	return po, nil
}

// List returns purchase orders for a merchant with optional filters and pagination.
func (s *Service) List(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
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

	where := "WHERE po.merchant_id = $1 AND po.deleted_at IS NULL"
	if params.Status != "" {
		where += " AND po.status = $" + strconv.Itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Keyword != "" {
		where += " AND po.order_no ILIKE $" + strconv.Itoa(argIdx)
		args = append(args, "%"+params.Keyword+"%")
		argIdx++
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM purchase_orders po `+where,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count purchase orders", err)
	}

	offset := (page - 1) * pageSize
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+poColumns+`, s.name as supplier_name
		 FROM purchase_orders po
		 LEFT JOIN suppliers s ON s.id = po.supplier_id
		 `+where+
			` ORDER BY po.created_at DESC LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx),
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list purchase orders", err)
	}
	defer rows.Close()

	orders := make([]PurchaseOrder, 0)
	for rows.Next() {
		po, err := scanPORowsWithSupplier(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan purchase order", err)
		}
		orders = append(orders, *po)
	}

	return &ListResult{
		Orders:   orders,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Update updates a draft purchase order. Only draft orders can be updated.
func (s *Service) Update(ctx context.Context, poID, merchantID int64, req UpdateRequest) (*PurchaseOrder, error) {
	existing, err := s.GetByID(ctx, poID, merchantID)
	if err != nil {
		return nil, err
	}

	if existing.Status != "draft" {
		return nil, apperrors.NewValidationError("only draft purchase orders can be updated")
	}

	if req.SupplierID != nil && *req.SupplierID != 0 {
		if err := s.validateMerchantSupplier(ctx, merchantID, *req.SupplierID); err != nil {
			return nil, err
		}
		existing.SupplierID = *req.SupplierID
	}
	if req.Notes != nil {
		existing.Notes = *req.Notes
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	if len(req.Items) > 0 {
		for i, item := range req.Items {
			if strings.TrimSpace(item.ProductName) == "" {
				return nil, apperrors.NewValidationError("item " + strconv.Itoa(i+1) + ": product_name is required")
			}
			if item.Quantity <= 0 {
				return nil, apperrors.NewValidationError("item " + strconv.Itoa(i+1) + ": quantity must be positive")
			}
			if item.UnitPriceCents <= 0 {
				return nil, apperrors.NewValidationError("item " + strconv.Itoa(i+1) + ": unit_price_cents must be positive")
			}
		}

		_, err := tx.ExecContext(ctx,
			`UPDATE purchase_order_items SET deleted_at = NOW() WHERE purchase_order_id = $1 AND deleted_at IS NULL`,
			poID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to clear items", err)
		}

		totalCents := 0
		for _, item := range req.Items {
			totalCents += item.Quantity * item.UnitPriceCents
			var pid, skuID interface{}
			if item.ProductID != nil && *item.ProductID > 0 {
				pid = *item.ProductID
			} else {
				pid = nil
			}
			if item.ProductSkuID != nil && *item.ProductSkuID > 0 {
				skuID = *item.ProductSkuID
			} else {
				skuID = nil
			}
			_, err := tx.ExecContext(ctx,
				`INSERT INTO purchase_order_items (purchase_order_id, product_id, product_sku_id, product_name, quantity, unit_price_cents, total_cents)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				poID, pid, skuID, item.ProductName, item.Quantity, item.UnitPriceCents, item.Quantity*item.UnitPriceCents,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to insert item", err)
			}
		}
		existing.TotalCents = totalCents
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE purchase_orders SET supplier_id=$1, total_cents=$2, notes=$3, updated_at=NOW()
		 WHERE id=$4 AND merchant_id=$5 AND deleted_at IS NULL`,
		existing.SupplierID, existing.TotalCents, existing.Notes, poID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update purchase order", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	return s.GetByID(ctx, poID, merchantID)
}

// Submit changes the status from draft to submitted.
func (s *Service) Submit(ctx context.Context, poID, merchantID int64) (*PurchaseOrder, error) {
	existing, err := s.GetByID(ctx, poID, merchantID)
	if err != nil {
		return nil, err
	}

	if existing.Status != "draft" {
		return nil, apperrors.NewValidationError("only draft purchase orders can be submitted")
	}
	if len(existing.Items) == 0 {
		return nil, apperrors.NewValidationError("purchase order must have at least one item")
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		`UPDATE purchase_orders SET status='submitted', submitted_at=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL`,
		now, poID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to submit purchase order", err)
	}

	return s.GetByID(ctx, poID, merchantID)
}

// Confirm changes the status from submitted to confirmed.
func (s *Service) Confirm(ctx context.Context, poID, merchantID int64) (*PurchaseOrder, error) {
	existing, err := s.GetByID(ctx, poID, merchantID)
	if err != nil {
		return nil, err
	}

	if existing.Status != "submitted" {
		return nil, apperrors.NewValidationError("only submitted purchase orders can be confirmed")
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		`UPDATE purchase_orders SET status='confirmed', confirmed_at=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL`,
		now, poID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to confirm purchase order", err)
	}

	return s.GetByID(ctx, poID, merchantID)
}

// Receive changes the status from confirmed to received, increments stock, and creates stock flows.
func (s *Service) Receive(ctx context.Context, poID, merchantID int64) (*PurchaseOrder, error) {
	existing, err := s.GetByID(ctx, poID, merchantID)
	if err != nil {
		return nil, err
	}

	if existing.Status != "confirmed" {
		return nil, apperrors.NewValidationError("only confirmed purchase orders can be received")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	for _, item := range existing.Items {
		if item.ProductID != nil && *item.ProductID > 0 {
			if item.ProductSkuID != nil && *item.ProductSkuID > 0 {
				_, err := tx.ExecContext(ctx,
					`UPDATE product_skus SET stock = stock + $1, updated_at = NOW()
					 WHERE id = $2 AND deleted_at IS NULL`,
					item.Quantity, *item.ProductSkuID,
				)
				if err != nil {
					return nil, apperrors.NewInternalError("failed to update SKU stock", err)
				}

				_, err = tx.ExecContext(ctx,
					`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, quantity_change)
					 VALUES ($1, $2, $3, 'inbound', $4)`,
					merchantID, *item.ProductID, *item.ProductSkuID, item.Quantity,
				)
				if err != nil {
					return nil, apperrors.NewInternalError("failed to record stock flow", err)
				}
			} else {
				_, err := tx.ExecContext(ctx,
					`UPDATE products SET stock = stock + $1, updated_at = NOW()
					 WHERE id = $2 AND deleted_at IS NULL`,
					item.Quantity, *item.ProductID,
				)
				if err != nil {
					return nil, apperrors.NewInternalError("failed to update product stock", err)
				}

				_, err = tx.ExecContext(ctx,
					`INSERT INTO stock_flows (merchant_id, product_id, type, quantity_change)
					 VALUES ($1, $2, 'inbound', $3)`,
					merchantID, *item.ProductID, item.Quantity,
				)
				if err != nil {
					return nil, apperrors.NewInternalError("failed to record stock flow", err)
				}
			}
		}

		_, err = tx.ExecContext(ctx,
			`UPDATE purchase_order_items SET received_quantity = $1, updated_at = NOW()
			 WHERE id = $2`,
			item.Quantity, item.ID,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update item received quantity", err)
		}
	}

	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE purchase_orders SET status='received', received_at=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL`,
		now, poID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to receive purchase order", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	return s.GetByID(ctx, poID, merchantID)
}

// Void cancels a purchase order. Only non-terminal orders can be voided.
func (s *Service) Void(ctx context.Context, poID, merchantID int64) (*PurchaseOrder, error) {
	existing, err := s.GetByID(ctx, poID, merchantID)
	if err != nil {
		return nil, err
	}

	if existing.Status == "received" || existing.Status == "voided" {
		return nil, apperrors.NewValidationError("purchase order cannot be voided in current status: " + existing.Status)
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		`UPDATE purchase_orders SET status='voided', voided_at=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL`,
		now, poID, merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to void purchase order", err)
	}

	return s.GetByID(ctx, poID, merchantID)
}
