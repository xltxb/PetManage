package inventory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// StockFlow represents a stock movement record.
type StockFlow struct {
	ID             int64     `json:"id"`
	MerchantID     int64     `json:"merchant_id"`
	ProductID      int64     `json:"product_id"`
	ProductSkuID   *int64    `json:"product_sku_id,omitempty"`
	OrderID        *int64    `json:"order_id,omitempty"`
	Type           string    `json:"type"`
	WarehouseID    *int64    `json:"warehouse_id,omitempty"`
	WarehouseName  string    `json:"warehouse_name,omitempty"`
	QuantityChange int       `json:"quantity_change"`
	Notes          string    `json:"notes,omitempty"`
	OperatorID     *int64    `json:"operator_id,omitempty"`
	OperatorName   string    `json:"operator_name"`
	RelatedFlowID  *int64    `json:"related_flow_id,omitempty"`
	ProductName    string    `json:"product_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Warehouse represents a storage location.
type Warehouse struct {
	ID         int64      `json:"id"`
	MerchantID int64      `json:"merchant_id"`
	Name       string     `json:"name"`
	Address    string     `json:"address"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// WarehouseStock represents product stock at a specific warehouse.
type WarehouseStock struct {
	ID           int64  `json:"id"`
	WarehouseID  int64  `json:"warehouse_id"`
	ProductID    int64  `json:"product_id"`
	ProductSkuID *int64 `json:"product_sku_id,omitempty"`
	Stock        int    `json:"stock"`
}

// Operation request types

// InboundRequest represents a manual stock-in request.
type InboundRequest struct {
	ProductID    int64  `json:"product_id"`
	ProductSkuID *int64 `json:"product_sku_id,omitempty"`
	WarehouseID  *int64 `json:"warehouse_id,omitempty"`
	Quantity     int    `json:"quantity"`
	Notes        string `json:"notes"`
	UnitCostCents *int  `json:"unit_cost_cents,omitempty"`
}

// OutboundRequest represents a manual stock-out request.
type OutboundRequest struct {
	ProductID    int64  `json:"product_id"`
	ProductSkuID *int64 `json:"product_sku_id,omitempty"`
	WarehouseID  *int64 `json:"warehouse_id,omitempty"`
	Quantity     int    `json:"quantity"`
	Reason       string `json:"reason"`
}

// TransferRequest represents a stock transfer between warehouses.
type TransferRequest struct {
	ProductID        int64  `json:"product_id"`
	ProductSkuID     *int64 `json:"product_sku_id,omitempty"`
	FromWarehouseID  int64  `json:"from_warehouse_id"`
	ToWarehouseID    int64  `json:"to_warehouse_id"`
	Quantity         int    `json:"quantity"`
	Notes            string `json:"notes"`
}

// LossRequest represents a stock loss (damage/expiry) request.
type LossRequest struct {
	ProductID    int64  `json:"product_id"`
	ProductSkuID *int64 `json:"product_sku_id,omitempty"`
	WarehouseID  *int64 `json:"warehouse_id,omitempty"`
	Quantity     int    `json:"quantity"`
	Reason       string `json:"reason"`
}

// SurplusRequest represents a stock surplus (found extra inventory) request.
type SurplusRequest struct {
	ProductID    int64  `json:"product_id"`
	ProductSkuID *int64 `json:"product_sku_id,omitempty"`
	WarehouseID  *int64 `json:"warehouse_id,omitempty"`
	Quantity     int    `json:"quantity"`
	Reason       string `json:"reason"`
}

// CreateWarehouseRequest is the request to create a warehouse.
type CreateWarehouseRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// ListFlowsParams holds filter/pagination for stock flow queries.
type ListFlowsParams struct {
	Type        string
	ProductID   *int64
	WarehouseID *int64
	StartTime   string
	EndTime     string
	Page        int
	PageSize    int
}

// ListFlowsResult wraps stock flow list with pagination.
type ListFlowsResult struct {
	Flows    []StockFlow `json:"flows"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// Service provides inventory operations.
type Service struct {
	db *sql.DB
}

// NewService creates an inventory Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateWarehouse adds a new warehouse for a merchant.
func (s *Service) CreateWarehouse(ctx context.Context, merchantID int64, req CreateWarehouseRequest) (*Warehouse, error) {
	if req.Name == "" {
		return nil, apperrors.NewValidationError("warehouse name is required")
	}
	w := &Warehouse{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO warehouses (merchant_id, name, address) VALUES ($1,$2,$3)
		 RETURNING id, merchant_id, name, address, status, created_at, updated_at`,
		merchantID, req.Name, req.Address,
	).Scan(&w.ID, &w.MerchantID, &w.Name, &w.Address, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create warehouse: %w", err)
	}
	return w, nil
}

// ListWarehouses returns all active warehouses for a merchant.
func (s *Service) ListWarehouses(ctx context.Context, merchantID int64) ([]Warehouse, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, name, address, status, created_at, updated_at
		 FROM warehouses WHERE merchant_id=$1 AND deleted_at IS NULL ORDER BY id`,
		merchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	defer rows.Close()

	var ws []Warehouse
	for rows.Next() {
		var w Warehouse
		if err := rows.Scan(&w.ID, &w.MerchantID, &w.Name, &w.Address, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan warehouse: %w", err)
		}
		ws = append(ws, w)
	}
	if ws == nil {
		ws = []Warehouse{}
	}
	return ws, nil
}

// Inbound adds stock to a product, optionally to a specific warehouse.
func (s *Service) Inbound(ctx context.Context, merchantID int64, operatorID int64, operatorName string, req InboundRequest) (*StockFlow, error) {
	if req.ProductID <= 0 {
		return nil, apperrors.NewValidationError("product_id is required")
	}
	if req.Quantity <= 0 {
		return nil, apperrors.NewValidationError("quantity must be positive")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update product stock.
	if req.ProductSkuID != nil && *req.ProductSkuID > 0 {
		_, err = tx.ExecContext(ctx,
			`UPDATE product_skus SET stock=stock+$1, updated_at=NOW() WHERE id=$2`,
			req.Quantity, *req.ProductSkuID)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE products SET stock=stock+$1, updated_at=NOW() WHERE id=$2 AND merchant_id=$3`,
			req.Quantity, req.ProductID, merchantID)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update stock", err)
	}

	// Update warehouse_stock if warehouse specified.
	if req.WarehouseID != nil && *req.WarehouseID > 0 {
		err = s.upsertWarehouseStock(ctx, tx, *req.WarehouseID, req.ProductID, req.ProductSkuID, req.Quantity)
		if err != nil {
			return nil, err
		}
	}

	// Create stock flow record.
	var flow StockFlow
	err = tx.QueryRowContext(ctx,
		`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, merchant_id, product_id, product_sku_id, order_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id, created_at`,
		merchantID, req.ProductID, req.ProductSkuID, "inbound", req.WarehouseID, req.Quantity, req.Notes, operatorID, operatorName,
	).Scan(&flow.ID, &flow.MerchantID, &flow.ProductID, &flow.ProductSkuID, &flow.OrderID, &flow.Type,
		&flow.WarehouseID, &flow.QuantityChange, &flow.Notes, &flow.OperatorID, &flow.OperatorName, &flow.RelatedFlowID, &flow.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert stock_flow: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Fetch product name.
	s.fillProductName(ctx, &flow)
	return &flow, nil
}

// Outbound removes stock from a product.
func (s *Service) Outbound(ctx context.Context, merchantID int64, operatorID int64, operatorName string, req OutboundRequest) (*StockFlow, error) {
	if req.ProductID <= 0 {
		return nil, apperrors.NewValidationError("product_id is required")
	}
	if req.Quantity <= 0 {
		return nil, apperrors.NewValidationError("quantity must be positive")
	}
	if req.Reason == "" {
		return nil, apperrors.NewValidationError("reason is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check stock sufficiency.
	currentStock := 0
	if req.ProductSkuID != nil && *req.ProductSkuID > 0 {
		err = tx.QueryRowContext(ctx,
			`SELECT stock FROM product_skus WHERE id=$1`, *req.ProductSkuID).Scan(&currentStock)
		if err != nil {
			return nil, apperrors.NewNotFoundError("product sku not found")
		}
		if currentStock < req.Quantity {
			return nil, apperrors.NewValidationError(
				fmt.Sprintf("insufficient stock: available %d, requested %d", currentStock, req.Quantity))
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE product_skus SET stock=stock-$1, updated_at=NOW() WHERE id=$2`,
			req.Quantity, *req.ProductSkuID)
	} else {
		err = tx.QueryRowContext(ctx,
			`SELECT stock FROM products WHERE id=$1 AND merchant_id=$2`, req.ProductID, merchantID).Scan(&currentStock)
		if err != nil {
			return nil, apperrors.NewNotFoundError("product not found")
		}
		if currentStock < req.Quantity {
			return nil, apperrors.NewValidationError(
				fmt.Sprintf("insufficient stock: available %d, requested %d", currentStock, req.Quantity))
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE products SET stock=stock-$1, updated_at=NOW() WHERE id=$2 AND merchant_id=$3`,
			req.Quantity, req.ProductID, merchantID)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update stock", err)
	}

	// Update warehouse_stock if warehouse specified.
	if req.WarehouseID != nil && *req.WarehouseID > 0 {
		err = s.upsertWarehouseStock(ctx, tx, *req.WarehouseID, req.ProductID, req.ProductSkuID, -req.Quantity)
		if err != nil {
			return nil, err
		}
	}

	var flow StockFlow
	err = tx.QueryRowContext(ctx,
		`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, merchant_id, product_id, product_sku_id, order_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id, created_at`,
		merchantID, req.ProductID, req.ProductSkuID, "outbound", req.WarehouseID, -req.Quantity, req.Reason, operatorID, operatorName,
	).Scan(&flow.ID, &flow.MerchantID, &flow.ProductID, &flow.ProductSkuID, &flow.OrderID, &flow.Type,
		&flow.WarehouseID, &flow.QuantityChange, &flow.Notes, &flow.OperatorID, &flow.OperatorName, &flow.RelatedFlowID, &flow.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert stock_flow: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.fillProductName(ctx, &flow)
	return &flow, nil
}

// Transfer moves stock from one warehouse to another.
func (s *Service) Transfer(ctx context.Context, merchantID int64, operatorID int64, operatorName string, req TransferRequest) (*StockFlow, *StockFlow, error) {
	if req.ProductID <= 0 {
		return nil, nil, apperrors.NewValidationError("product_id is required")
	}
	if req.Quantity <= 0 {
		return nil, nil, apperrors.NewValidationError("quantity must be positive")
	}
	if req.FromWarehouseID <= 0 || req.ToWarehouseID <= 0 {
		return nil, nil, apperrors.NewValidationError("both from_warehouse_id and to_warehouse_id are required")
	}
	if req.FromWarehouseID == req.ToWarehouseID {
		return nil, nil, apperrors.NewValidationError("source and target warehouses must differ")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Reduce from source warehouse.
	err = s.upsertWarehouseStock(ctx, tx, req.FromWarehouseID, req.ProductID, req.ProductSkuID, -req.Quantity)
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to deduct from source warehouse", err)
	}

	// Add to target warehouse.
	err = s.upsertWarehouseStock(ctx, tx, req.ToWarehouseID, req.ProductID, req.ProductSkuID, req.Quantity)
	if err != nil {
		return nil, nil, apperrors.NewInternalError("failed to add to target warehouse", err)
	}

	// Create transfer_out flow record.
	var flowOut StockFlow
	err = tx.QueryRowContext(ctx,
		`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, merchant_id, product_id, product_sku_id, order_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id, created_at`,
		merchantID, req.ProductID, req.ProductSkuID, "transfer_out", &req.FromWarehouseID, -req.Quantity, req.Notes, operatorID, operatorName,
	).Scan(&flowOut.ID, &flowOut.MerchantID, &flowOut.ProductID, &flowOut.ProductSkuID, &flowOut.OrderID, &flowOut.Type,
		&flowOut.WarehouseID, &flowOut.QuantityChange, &flowOut.Notes, &flowOut.OperatorID, &flowOut.OperatorName, &flowOut.RelatedFlowID, &flowOut.CreatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert transfer_out: %w", err)
	}

	// Create transfer_in flow record, linked to the out record.
	var flowIn StockFlow
	err = tx.QueryRowContext(ctx,
		`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, merchant_id, product_id, product_sku_id, order_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id, created_at`,
		merchantID, req.ProductID, req.ProductSkuID, "transfer_in", &req.ToWarehouseID, req.Quantity, req.Notes, operatorID, operatorName, &flowOut.ID,
	).Scan(&flowIn.ID, &flowIn.MerchantID, &flowIn.ProductID, &flowIn.ProductSkuID, &flowIn.OrderID, &flowIn.Type,
		&flowIn.WarehouseID, &flowIn.QuantityChange, &flowIn.Notes, &flowIn.OperatorID, &flowIn.OperatorName, &flowIn.RelatedFlowID, &flowIn.CreatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert transfer_in: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	s.fillProductName(ctx, &flowOut)
	s.fillProductName(ctx, &flowIn)
	return &flowOut, &flowIn, nil
}

// Loss records a stock loss (damage/expiry).
func (s *Service) Loss(ctx context.Context, merchantID int64, operatorID int64, operatorName string, req LossRequest) (*StockFlow, error) {
	if req.ProductID <= 0 {
		return nil, apperrors.NewValidationError("product_id is required")
	}
	if req.Quantity <= 0 {
		return nil, apperrors.NewValidationError("quantity must be positive")
	}
	if req.Reason == "" {
		return nil, apperrors.NewValidationError("reason is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check stock and decrease.
	if req.ProductSkuID != nil && *req.ProductSkuID > 0 {
		var cur int
		err = tx.QueryRowContext(ctx, `SELECT stock FROM product_skus WHERE id=$1`, *req.ProductSkuID).Scan(&cur)
		if err != nil {
			return nil, apperrors.NewNotFoundError("product sku not found")
		}
		if cur < req.Quantity {
			return nil, apperrors.NewValidationError(fmt.Sprintf("insufficient stock: available %d, requested %d", cur, req.Quantity))
		}
		_, err = tx.ExecContext(ctx, `UPDATE product_skus SET stock=stock-$1, updated_at=NOW() WHERE id=$2`, req.Quantity, *req.ProductSkuID)
	} else {
		var cur int
		err = tx.QueryRowContext(ctx, `SELECT stock FROM products WHERE id=$1 AND merchant_id=$2`, req.ProductID, merchantID).Scan(&cur)
		if err != nil {
			return nil, apperrors.NewNotFoundError("product not found")
		}
		if cur < req.Quantity {
			return nil, apperrors.NewValidationError(fmt.Sprintf("insufficient stock: available %d, requested %d", cur, req.Quantity))
		}
		_, err = tx.ExecContext(ctx, `UPDATE products SET stock=stock-$1, updated_at=NOW() WHERE id=$2 AND merchant_id=$3`, req.Quantity, req.ProductID, merchantID)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update stock", err)
	}

	if req.WarehouseID != nil && *req.WarehouseID > 0 {
		err = s.upsertWarehouseStock(ctx, tx, *req.WarehouseID, req.ProductID, req.ProductSkuID, -req.Quantity)
		if err != nil {
			return nil, err
		}
	}

	var flow StockFlow
	err = tx.QueryRowContext(ctx,
		`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, merchant_id, product_id, product_sku_id, order_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id, created_at`,
		merchantID, req.ProductID, req.ProductSkuID, "loss", req.WarehouseID, -req.Quantity, req.Reason, operatorID, operatorName,
	).Scan(&flow.ID, &flow.MerchantID, &flow.ProductID, &flow.ProductSkuID, &flow.OrderID, &flow.Type,
		&flow.WarehouseID, &flow.QuantityChange, &flow.Notes, &flow.OperatorID, &flow.OperatorName, &flow.RelatedFlowID, &flow.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert stock_flow: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.fillProductName(ctx, &flow)
	return &flow, nil
}

// Surplus records a stock surplus (found extra inventory).
func (s *Service) Surplus(ctx context.Context, merchantID int64, operatorID int64, operatorName string, req SurplusRequest) (*StockFlow, error) {
	if req.ProductID <= 0 {
		return nil, apperrors.NewValidationError("product_id is required")
	}
	if req.Quantity <= 0 {
		return nil, apperrors.NewValidationError("quantity must be positive")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if req.ProductSkuID != nil && *req.ProductSkuID > 0 {
		_, err = tx.ExecContext(ctx, `UPDATE product_skus SET stock=stock+$1, updated_at=NOW() WHERE id=$2`, req.Quantity, *req.ProductSkuID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE products SET stock=stock+$1, updated_at=NOW() WHERE id=$2 AND merchant_id=$3`, req.Quantity, req.ProductID, merchantID)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update stock", err)
	}

	if req.WarehouseID != nil && *req.WarehouseID > 0 {
		err = s.upsertWarehouseStock(ctx, tx, *req.WarehouseID, req.ProductID, req.ProductSkuID, req.Quantity)
		if err != nil {
			return nil, err
		}
	}

	var flow StockFlow
	err = tx.QueryRowContext(ctx,
		`INSERT INTO stock_flows (merchant_id, product_id, product_sku_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, merchant_id, product_id, product_sku_id, order_id, type, warehouse_id, quantity_change, notes, operator_id, operator_name, related_flow_id, created_at`,
		merchantID, req.ProductID, req.ProductSkuID, "surplus", req.WarehouseID, req.Quantity, req.Reason, operatorID, operatorName,
	).Scan(&flow.ID, &flow.MerchantID, &flow.ProductID, &flow.ProductSkuID, &flow.OrderID, &flow.Type,
		&flow.WarehouseID, &flow.QuantityChange, &flow.Notes, &flow.OperatorID, &flow.OperatorName, &flow.RelatedFlowID, &flow.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert stock_flow: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.fillProductName(ctx, &flow)
	return &flow, nil
}

// ListFlows returns stock flow records with filtering and pagination.
func (s *Service) ListFlows(ctx context.Context, merchantID int64, params ListFlowsParams) (*ListFlowsResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	where := "WHERE sf.merchant_id = $1"
	args := []interface{}{merchantID}
	argIdx := 2

	if params.Type != "" {
		where += fmt.Sprintf(" AND sf.type = $%d", argIdx)
		args = append(args, params.Type)
		argIdx++
	}
	if params.ProductID != nil && *params.ProductID > 0 {
		where += fmt.Sprintf(" AND sf.product_id = $%d", argIdx)
		args = append(args, *params.ProductID)
		argIdx++
	}
	if params.WarehouseID != nil && *params.WarehouseID > 0 {
		where += fmt.Sprintf(" AND sf.warehouse_id = $%d", argIdx)
		args = append(args, *params.WarehouseID)
		argIdx++
	}
	if params.StartTime != "" {
		where += fmt.Sprintf(" AND sf.created_at >= $%d", argIdx)
		args = append(args, params.StartTime)
		argIdx++
	}
	if params.EndTime != "" {
		where += fmt.Sprintf(" AND sf.created_at <= $%d", argIdx)
		args = append(args, params.EndTime)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM stock_flows sf %s", where)
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count flows: %w", err)
	}

	limit := params.PageSize
	offset := (params.Page - 1) * params.PageSize
	query := fmt.Sprintf(
		`SELECT sf.id, sf.merchant_id, sf.product_id, sf.product_sku_id, sf.order_id,
		        sf.type, sf.warehouse_id, sf.quantity_change, sf.notes,
		        sf.operator_id, sf.operator_name, sf.related_flow_id, sf.created_at,
		        COALESCE(p.name, '') AS product_name
		 FROM stock_flows sf
		 LEFT JOIN products p ON p.id = sf.product_id
		 %s ORDER BY sf.created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query flows: %w", err)
	}
	defer rows.Close()

	var flows []StockFlow
	for rows.Next() {
		var f StockFlow
		if err := rows.Scan(&f.ID, &f.MerchantID, &f.ProductID, &f.ProductSkuID, &f.OrderID,
			&f.Type, &f.WarehouseID, &f.QuantityChange, &f.Notes,
			&f.OperatorID, &f.OperatorName, &f.RelatedFlowID, &f.CreatedAt, &f.ProductName); err != nil {
			return nil, fmt.Errorf("scan flow: %w", err)
		}
		flows = append(flows, f)
	}
	if flows == nil {
		flows = []StockFlow{}
	}

	return &ListFlowsResult{
		Flows:    flows,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetWarehouseStock returns stock levels for a product across all warehouses.
func (s *Service) GetWarehouseStock(ctx context.Context, merchantID int64, productID int64, productSkuID *int64) ([]WarehouseStock, error) {
	query := `SELECT ws.id, ws.warehouse_id, ws.product_id, ws.product_sku_id, ws.stock
	           FROM warehouse_stocks ws
	           JOIN warehouses w ON w.id = ws.warehouse_id
	           WHERE w.merchant_id=$1 AND ws.product_id=$2`
	args := []interface{}{merchantID, productID}

	if productSkuID != nil && *productSkuID > 0 {
		query += " AND ws.product_sku_id=$3"
		args = append(args, *productSkuID)
	} else {
		query += " AND ws.product_sku_id IS NULL"
	}
	query += " ORDER BY ws.warehouse_id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query warehouse stock: %w", err)
	}
	defer rows.Close()

	var stocks []WarehouseStock
	for rows.Next() {
		var ws WarehouseStock
		if err := rows.Scan(&ws.ID, &ws.WarehouseID, &ws.ProductID, &ws.ProductSkuID, &ws.Stock); err != nil {
			return nil, fmt.Errorf("scan warehouse stock: %w", err)
		}
		stocks = append(stocks, ws)
	}
	if stocks == nil {
		stocks = []WarehouseStock{}
	}
	return stocks, nil
}

// upsertWarehouseStock inserts or updates warehouse-level stock.
func (s *Service) upsertWarehouseStock(ctx context.Context, tx *sql.Tx, warehouseID, productID int64, productSkuID *int64, delta int) error {
	if productSkuID != nil && *productSkuID > 0 {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO warehouse_stocks (warehouse_id, product_id, product_sku_id, stock)
			 VALUES ($1,$2,$3,$4)
			 ON CONFLICT (warehouse_id, product_id, COALESCE(product_sku_id, 0))
			 DO UPDATE SET stock=warehouse_stocks.stock+$4, updated_at=NOW()`,
			warehouseID, productID, *productSkuID, delta)
		return err
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO warehouse_stocks (warehouse_id, product_id, stock)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (warehouse_id, product_id, COALESCE(product_sku_id, 0))
		 DO UPDATE SET stock=warehouse_stocks.stock+$3, updated_at=NOW()`,
		warehouseID, productID, delta)
	return err
}

func (s *Service) fillProductName(ctx context.Context, f *StockFlow) {
	if f == nil || f.ProductID == 0 {
		return
	}
	_ = s.db.QueryRowContext(ctx, `SELECT name FROM products WHERE id=$1`, f.ProductID).Scan(&f.ProductName)
}
