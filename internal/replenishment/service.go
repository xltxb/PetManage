package replenishment

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Suggestion represents a single replenishment suggestion for a product.
type Suggestion struct {
	ProductID     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	Barcode       string  `json:"barcode"`
	Stock         int     `json:"stock"`
	AlertStock    int     `json:"alert_stock"`
	SuggestedQty  int     `json:"suggested_qty"`
	AvgDailySales float64 `json:"avg_daily_sales"`
	HasSales      bool    `json:"has_sales"`
	SupplierID    *int64  `json:"supplier_id"`
	SupplierName  string  `json:"supplier_name"`
}

// SuggestionResult contains replenishment suggestions grouped by supplier.
type SuggestionResult struct {
	Suggestions []Suggestion              `json:"suggestions"`
	Total       int                       `json:"total"`
	BySupplier  map[string][]Suggestion   `json:"by_supplier,omitempty"`
}

// GeneratePOItem is a single item in a generate-PO request.
type GeneratePOItem struct {
	ProductID  int64 `json:"product_id"`
	Quantity   int   `json:"quantity"`
	SupplierID int64 `json:"supplier_id"`
}

// GeneratePORequest is the request body for generating purchase orders.
type GeneratePORequest struct {
	Items []GeneratePOItem `json:"items"`
}

// Service provides replenishment suggestion operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new replenishment Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetSuggestions returns products below alert stock with suggested replenishment quantities.
func (s *Service) GetSuggestions(ctx context.Context, merchantID int64, groupBySupplier bool) (*SuggestionResult, error) {
	// Find products below alert stock.
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.name, p.barcode, p.stock, p.alert_stock
		 FROM products p
		 WHERE p.merchant_id = $1
		   AND p.deleted_at IS NULL
		   AND p.status = 'active'
		   AND p.alert_stock > 0
		   AND p.stock < p.alert_stock
		 ORDER BY (p.alert_stock - p.stock) DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query products below alert stock", err)
	}
	defer rows.Close()

	type rawProduct struct {
		id         int64
		name       string
		barcode    string
		stock      int
		alertStock int
	}
	var products []rawProduct
	for rows.Next() {
		var p rawProduct
		if err := rows.Scan(&p.id, &p.name, &p.barcode, &p.stock, &p.alertStock); err != nil {
			return nil, apperrors.NewInternalError("failed to scan product", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternalError("failed iterating products", err)
	}

	if len(products) == 0 {
		return &SuggestionResult{
			Suggestions: []Suggestion{},
			Total:       0,
			BySupplier:  map[string][]Suggestion{},
		}, nil
	}

	// Build product ID list for batch queries.
	productIDs := make([]int64, len(products))
	productMap := make(map[int64]rawProduct, len(products))
	for i, p := range products {
		productIDs[i] = p.id
		productMap[p.id] = p
	}

	// Calculate 30-day avg daily sales for each product.
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	avgSalesMap, err := s.calcAvgDailySales(ctx, merchantID, productIDs, thirtyDaysAgo)
	if err != nil {
		return nil, err
	}

	// Get supplier info for each product.
	supplierMap, err := s.getProductSuppliers(ctx, productIDs)
	if err != nil {
		return nil, err
	}

	suggestions := make([]Suggestion, 0, len(products))
	bySupplier := map[string][]Suggestion{}

	for _, p := range products {
		avgDaily := avgSalesMap[p.id]
		hasSales := avgDaily > 0
		var suggestedQty int

		if hasSales {
			// 30-day avg daily sales × 7 - current stock (floor at 0).
			suggestedQty = int(math.Max(0, math.Round(avgDaily*7-float64(p.stock))))
		} else {
			// No recent sales: alert_stock - current_stock.
			suggestedQty = p.alertStock - p.stock
			if suggestedQty < 0 {
				suggestedQty = 0
			}
		}

		supInfo, ok := supplierMap[p.id]
		var supIDPtr *int64
		var supName string
		if ok && supInfo.id > 0 {
			supIDPtr = &supInfo.id
			supName = supInfo.name
		}

		sug := Suggestion{
			ProductID:     p.id,
			ProductName:   p.name,
			Barcode:       p.barcode,
			Stock:         p.stock,
			AlertStock:    p.alertStock,
			SuggestedQty:  suggestedQty,
			AvgDailySales: math.Round(avgDaily*100) / 100,
			HasSales:      hasSales,
			SupplierID:    supIDPtr,
			SupplierName:  supName,
		}
		suggestions = append(suggestions, sug)

		if groupBySupplier {
			key := supName
			if key == "" {
				key = "未指定供应商"
			}
			bySupplier[key] = append(bySupplier[key], sug)
		}
	}

	return &SuggestionResult{
		Suggestions: suggestions,
		Total:       len(suggestions),
		BySupplier:  bySupplier,
	}, nil
}

func (s *Service) calcAvgDailySales(ctx context.Context, merchantID int64, productIDs []int64, since time.Time) (map[int64]float64, error) {
	if len(productIDs) == 0 {
		return map[int64]float64{}, nil
	}

	// Build IN clause placeholders.
	placeholders := make([]string, len(productIDs))
	args := make([]interface{}, 0, len(productIDs)+2)
	args = append(args, merchantID, since)
	for i, id := range productIDs {
		placeholders[i] = "$" + strconv.Itoa(i+3)
		args = append(args, id)
	}

	query := `SELECT oi.product_id, SUM(oi.quantity) as total_sold
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 WHERE o.merchant_id = $1
		   AND o.created_at >= $2
		   AND o.status = 'completed'
		   AND oi.product_id IN (` + strings.Join(placeholders, ",") + `)
		 GROUP BY oi.product_id`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to calculate sales", err)
	}
	defer rows.Close()

	result := make(map[int64]float64, len(productIDs))
	for rows.Next() {
		var pid int64
		var totalSold int
		if err := rows.Scan(&pid, &totalSold); err != nil {
			return nil, apperrors.NewInternalError("failed to scan sales data", err)
		}
		result[pid] = float64(totalSold) / 30.0
	}
	return result, rows.Err()
}

func (s *Service) getProductSuppliers(ctx context.Context, productIDs []int64) (map[int64]supplierInfo, error) {
	if len(productIDs) == 0 {
		return map[int64]supplierInfo{}, nil
	}

	placeholders := make([]string, len(productIDs))
	args := make([]interface{}, len(productIDs))
	for i, id := range productIDs {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}

	query := `SELECT DISTINCT ON (sp.product_id) sp.product_id, s.id, s.name
		 FROM supplier_products sp
		 JOIN suppliers s ON s.id = sp.supplier_id AND s.deleted_at IS NULL AND s.status = 'active'
		 WHERE sp.product_id IN (` + strings.Join(placeholders, ",") + `)
		 ORDER BY sp.product_id, sp.created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query product suppliers", err)
	}
	defer rows.Close()

	result := make(map[int64]supplierInfo, len(productIDs))
	for rows.Next() {
		var pid, sid int64
		var name string
		if err := rows.Scan(&pid, &sid, &name); err != nil {
			return nil, apperrors.NewInternalError("failed to scan supplier info", err)
		}
		result[pid] = supplierInfo{id: sid, name: name}
	}
	return result, rows.Err()
}

type supplierInfo struct {
	id   int64
	name string
}

// GeneratePO creates draft purchase orders from replenishment suggestions,
// grouping items by supplier. Returns the created purchase orders.
func (s *Service) GeneratePO(ctx context.Context, merchantID, userID int64, req GeneratePORequest) ([]poSummary, error) {
	if len(req.Items) == 0 {
		return nil, apperrors.NewValidationError("at least one item is required")
	}

	// Group items by supplier_id.
	groups := make(map[int64][]GeneratePOItem)
	for _, item := range req.Items {
		if item.ProductID <= 0 {
			return nil, apperrors.NewValidationError("invalid product_id")
		}
		if item.Quantity <= 0 {
			return nil, apperrors.NewValidationError("quantity must be positive for product_id=" + strconv.FormatInt(item.ProductID, 10))
		}
		if item.SupplierID <= 0 {
			return nil, apperrors.NewValidationError("supplier_id is required for product_id=" + strconv.FormatInt(item.ProductID, 10))
		}
		groups[item.SupplierID] = append(groups[item.SupplierID], item)
	}

	// Create one PO per supplier group.
	results := make([]poSummary, 0, len(groups))

	for supplierID, items := range groups {
		// Verify supplier belongs to merchant and is active.
		var supStatus string
		var supName string
		err := s.db.QueryRowContext(ctx,
			`SELECT status, name FROM suppliers WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`,
			supplierID, merchantID,
		).Scan(&supStatus, &supName)
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFoundError("supplier not found: " + strconv.FormatInt(supplierID, 10))
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to verify supplier", err)
		}
		if supStatus != "active" {
			return nil, apperrors.NewValidationError("supplier is not active: " + supName)
		}

		// Get product info and build PO items.
		poItems := make([]poItemInput, 0, len(items))
		for _, item := range items {
			var name string
			var costCents int
			err := s.db.QueryRowContext(ctx,
				`SELECT name, cost_cents FROM products WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`,
				item.ProductID, merchantID,
			).Scan(&name, &costCents)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("product not found: " + strconv.FormatInt(item.ProductID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to get product info", err)
			}
			poItems = append(poItems, poItemInput{
				productID:      item.ProductID,
				productName:    name,
				quantity:       item.Quantity,
				unitPriceCents: costCents,
			})
		}

		// Create draft PO.
		po, err := s.createPO(ctx, merchantID, userID, supplierID, supName, poItems)
		if err != nil {
			return nil, err
		}
		results = append(results, *po)
	}

	return results, nil
}

type poItemInput struct {
	productID      int64
	productName    string
	quantity       int
	unitPriceCents int
}

type poSummary struct {
	ID           int64  `json:"id"`
	OrderNo      string `json:"order_no"`
	SupplierID   int64  `json:"supplier_id"`
	SupplierName string `json:"supplier_name"`
	ItemCount    int    `json:"item_count"`
	TotalCents   int    `json:"total_cents"`
	Status       string `json:"status"`
}

func (s *Service) createPO(ctx context.Context, merchantID, userID, supplierID int64, supplierName string, items []poItemInput) (*poSummary, error) {
	// Generate order number.
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
		return nil, apperrors.NewInternalError("failed to generate order number", err)
	}

	serial := maxSerial + 1
	if serial > 9999 {
		serial = 1
	}
	orderNo := prefix + fmt.Sprintf("%04d", serial)

	totalCents := 0
	for _, item := range items {
		totalCents += item.quantity * item.unitPriceCents
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	var poID int64
	var poStatus string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO purchase_orders (merchant_id, supplier_id, order_no, status, total_cents, notes, created_by)
		 VALUES ($1, $2, $3, 'draft', $4, $5, $6)
		 RETURNING id, status`,
		merchantID, supplierID, orderNo, totalCents, "自动生成补货采购单", userID,
	).Scan(&poID, &poStatus)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create purchase order", err)
	}

	for _, item := range items {
		var pid interface{}
		if item.productID > 0 {
			pid = item.productID
		} else {
			pid = nil
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO purchase_order_items (purchase_order_id, product_id, product_name, quantity, unit_price_cents, total_cents)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			poID, pid, item.productName, item.quantity, item.unitPriceCents, item.quantity*item.unitPriceCents,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to create purchase order item", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	return &poSummary{
		ID:           poID,
		OrderNo:      orderNo,
		SupplierID:   supplierID,
		SupplierName: supplierName,
		ItemCount:    len(items),
		TotalCents:   totalCents,
		Status:       poStatus,
	}, nil
}
