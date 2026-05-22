package product

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Product represents a product record.
type Product struct {
	ID            int64      `json:"id"`
	MerchantID    int64      `json:"merchant_id"`
	Barcode       string     `json:"barcode"`
	Name          string     `json:"name"`
	Brand         string     `json:"brand"`
	Specification string     `json:"specification"`
	PriceCents    int        `json:"price_cents"`
	CostCents     int        `json:"cost_cents"`
	Stock         int        `json:"stock"`
	AlertStock    int        `json:"alert_stock"`
	ExpiryDate    *string    `json:"expiry_date"`
	CategoryID    *int64     `json:"category_id"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// CreateProductRequest is the request body for creating a product.
type CreateProductRequest struct {
	Barcode       string `json:"barcode"`
	Name          string `json:"name"`
	Brand         string `json:"brand"`
	Specification string `json:"specification"`
	PriceCents    int    `json:"price_cents"`
	CostCents     int    `json:"cost_cents"`
	Stock         int    `json:"stock"`
	AlertStock    int    `json:"alert_stock"`
	ExpiryDate    string `json:"expiry_date"`
	CategoryID    *int64 `json:"category_id"`
}

// UpdateProductRequest is the request body for updating a product.
type UpdateProductRequest struct {
	Barcode       *string `json:"barcode"`
	Name          *string `json:"name"`
	Brand         *string `json:"brand"`
	Specification *string `json:"specification"`
	PriceCents    *int    `json:"price_cents"`
	CostCents     *int    `json:"cost_cents"`
	Stock         *int    `json:"stock"`
	AlertStock    *int    `json:"alert_stock"`
	ExpiryDate    *string `json:"expiry_date"`
	CategoryID    *int64  `json:"category_id"`
}

// ListParams holds optional filters and pagination for listing products.
type ListParams struct {
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// ListResult wraps the products list with pagination info.
type ListResult struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// Service provides product management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new product Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const productColumns = `id, merchant_id, barcode, name, brand, specification, price_cents, cost_cents, stock, alert_stock, expiry_date, category_id, status, created_at, updated_at`

func scanRow(row *sql.Row) (*Product, error) {
	p := &Product{}
	err := row.Scan(
		&p.ID, &p.MerchantID, &p.Barcode, &p.Name, &p.Brand, &p.Specification,
		&p.PriceCents, &p.CostCents, &p.Stock, &p.AlertStock, &p.ExpiryDate,
		&p.CategoryID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func scanRows(rows *sql.Rows) (*Product, error) {
	p := &Product{}
	err := rows.Scan(
		&p.ID, &p.MerchantID, &p.Barcode, &p.Name, &p.Brand, &p.Specification,
		&p.PriceCents, &p.CostCents, &p.Stock, &p.AlertStock, &p.ExpiryDate,
		&p.CategoryID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

// Create creates a new product for a merchant.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateProductRequest) (*Product, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("product name is required")
	}
	if req.PriceCents < 0 {
		return nil, apperrors.NewValidationError("price_cents must be non-negative")
	}
	if req.Stock < 0 {
		return nil, apperrors.NewValidationError("stock must be non-negative")
	}

	var expiryDate *string
	if strings.TrimSpace(req.ExpiryDate) != "" {
		expiryDate = &req.ExpiryDate
	}

	p, err := scanRow(s.db.QueryRowContext(ctx,
		`INSERT INTO products (merchant_id, barcode, name, brand, specification, price_cents, cost_cents, stock, alert_stock, expiry_date, category_id, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active')
		 RETURNING `+productColumns,
		merchantID, req.Barcode, req.Name, req.Brand, req.Specification, req.PriceCents, req.CostCents, req.Stock, req.AlertStock, expiryDate, req.CategoryID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create product", err)
	}
	return p, nil
}

// GetByID returns a single product by ID, scoped to a merchant.
func (s *Service) GetByID(ctx context.Context, productID, merchantID int64) (*Product, error) {
	p, err := scanRow(s.db.QueryRowContext(ctx,
		`SELECT `+productColumns+` FROM products
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		productID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("product not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get product", err)
	}
	return p, nil
}

// Update updates product fields. Only non-nil fields in the request are applied.
func (s *Service) Update(ctx context.Context, productID, merchantID int64, req UpdateProductRequest) (*Product, error) {
	existing, err := s.GetByID(ctx, productID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Barcode != nil {
		existing.Barcode = *req.Barcode
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("product name is required")
		}
		existing.Name = *req.Name
	}
	if req.Brand != nil {
		existing.Brand = *req.Brand
	}
	if req.Specification != nil {
		existing.Specification = *req.Specification
	}
	if req.PriceCents != nil {
		if *req.PriceCents < 0 {
			return nil, apperrors.NewValidationError("price_cents must be non-negative")
		}
		existing.PriceCents = *req.PriceCents
	}
	if req.CostCents != nil {
		existing.CostCents = *req.CostCents
	}
	if req.Stock != nil {
		if *req.Stock < 0 {
			return nil, apperrors.NewValidationError("stock must be non-negative")
		}
		existing.Stock = *req.Stock
	}
	if req.AlertStock != nil {
		existing.AlertStock = *req.AlertStock
	}
	if req.ExpiryDate != nil {
		existing.ExpiryDate = req.ExpiryDate
	}
	if req.CategoryID != nil {
		existing.CategoryID = req.CategoryID
	}

	p, err := scanRow(s.db.QueryRowContext(ctx,
		`UPDATE products SET
		 barcode = $1, name = $2, brand = $3, specification = $4,
		 price_cents = $5, cost_cents = $6, stock = $7, alert_stock = $8,
		 expiry_date = $9, category_id = $10, updated_at = NOW()
		 WHERE id = $11 AND merchant_id = $12 AND deleted_at IS NULL
		 RETURNING `+productColumns,
		existing.Barcode, existing.Name, existing.Brand, existing.Specification,
		existing.PriceCents, existing.CostCents, existing.Stock, existing.AlertStock,
		existing.ExpiryDate, existing.CategoryID, productID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update product", err)
	}
	return p, nil
}

// Delete soft-deletes a product after checking for stock and pending orders.
func (s *Service) Delete(ctx context.Context, productID, merchantID int64) error {
	existing, err := s.GetByID(ctx, productID, merchantID)
	if err != nil {
		return err
	}

	if existing.Stock > 0 {
		return apperrors.NewValidationError("cannot delete product with remaining stock, please clear inventory first")
	}

	var orderCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 WHERE oi.product_id = $1 AND o.merchant_id = $2 AND o.status = 'completed'`,
		productID, merchantID,
	).Scan(&orderCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check pending orders", err)
	}
	if orderCount > 0 {
		return apperrors.NewValidationError("cannot delete product with pending orders, please resolve them first")
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE products SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		productID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete product", err)
	}
	return nil
}

// List returns products for a merchant with optional filtering, search, and pagination.
func (s *Service) List(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	conditions = append(conditions, "merchant_id = $1")
	args = append(args, merchantID)
	conditions = append(conditions, "deleted_at IS NULL")
	argIdx := 2

	if params.Status != "" {
		conditions = append(conditions, "status = $"+strconv.Itoa(argIdx))
		args = append(args, params.Status)
		argIdx++
	}
	if params.Keyword != "" {
		kw := "%" + params.Keyword + "%"
		conditions = append(conditions, "(barcode ILIKE $"+strconv.Itoa(argIdx)+" OR name ILIKE $"+strconv.Itoa(argIdx+1)+")")
		args = append(args, kw, kw)
		argIdx += 2
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count products", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+productColumns+` FROM products WHERE `+whereClause+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list products", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		p, err := scanRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan product", err)
		}
		products = append(products, *p)
	}
	if products == nil {
		products = []Product{}
	}

	return &ListResult{
		Products: products,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// ToggleStatus toggles a product between active and inactive.
func (s *Service) ToggleStatus(ctx context.Context, productID, merchantID int64) (*Product, error) {
	p, err := scanRow(s.db.QueryRowContext(ctx,
		`UPDATE products SET status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING `+productColumns,
		productID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("product not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle product status", err)
	}
	return p, nil
}

// --- SKU (multi-spec variant) management ---

// ProductSku represents a single SKU variant of a product.
type ProductSku struct {
	ID         int64                  `json:"id"`
	ProductID  int64                  `json:"product_id"`
	SkuCode    string                 `json:"sku_code"`
	SpecInfo   map[string]string      `json:"spec_info"`
	PriceCents int                    `json:"price_cents"`
	CostCents  int                    `json:"cost_cents"`
	Stock      int                    `json:"stock"`
	AlertStock int                    `json:"alert_stock"`
	Status     string                 `json:"status"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// CreateSkuRequest is the request body for creating a SKU.
type CreateSkuRequest struct {
	SkuCode    string            `json:"sku_code"`
	SpecInfo   map[string]string `json:"spec_info"`
	PriceCents int               `json:"price_cents"`
	CostCents  int               `json:"cost_cents"`
	Stock      int               `json:"stock"`
	AlertStock int               `json:"alert_stock"`
}

// UpdateSkuRequest is the request body for updating a SKU.
type UpdateSkuRequest struct {
	SkuCode    *string            `json:"sku_code"`
	SpecInfo   map[string]string  `json:"spec_info"`
	PriceCents *int               `json:"price_cents"`
	CostCents  *int               `json:"cost_cents"`
	Stock      *int               `json:"stock"`
	AlertStock *int               `json:"alert_stock"`
}

const skuColumns = `id, product_id, sku_code, spec_info, price_cents, cost_cents, stock, alert_stock, status, created_at, updated_at`

func scanSkuRow(row *sql.Row) (*ProductSku, error) {
	s := &ProductSku{}
	var specJSON []byte
	err := row.Scan(&s.ID, &s.ProductID, &s.SkuCode, &specJSON, &s.PriceCents, &s.CostCents, &s.Stock, &s.AlertStock, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if specJSON != nil {
		json.Unmarshal(specJSON, &s.SpecInfo)
	}
	if s.SpecInfo == nil {
		s.SpecInfo = map[string]string{}
	}
	return s, nil
}

func scanSkuRows(rows *sql.Rows) (*ProductSku, error) {
	s := &ProductSku{}
	var specJSON []byte
	err := rows.Scan(&s.ID, &s.ProductID, &s.SkuCode, &specJSON, &s.PriceCents, &s.CostCents, &s.Stock, &s.AlertStock, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if specJSON != nil {
		json.Unmarshal(specJSON, &s.SpecInfo)
	}
	if s.SpecInfo == nil {
		s.SpecInfo = map[string]string{}
	}
	return s, nil
}

// CreateSKU creates a SKU variant for a product.
func (s *Service) CreateSKU(ctx context.Context, productID, merchantID int64, req CreateSkuRequest) (*ProductSku, error) {
	// Verify product belongs to merchant.
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		productID, merchantID,
	).Scan(&exists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify product", err)
	}
	if !exists {
		return nil, apperrors.NewNotFoundError("product not found")
	}

	if req.PriceCents < 0 {
		return nil, apperrors.NewValidationError("price_cents must be non-negative")
	}
	if req.Stock < 0 {
		return nil, apperrors.NewValidationError("stock must be non-negative")
	}

	specJSON, _ := json.Marshal(req.SpecInfo)
	sku, err := scanSkuRow(s.db.QueryRowContext(ctx,
		`INSERT INTO product_skus (product_id, sku_code, spec_info, price_cents, cost_cents, stock, alert_stock)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+skuColumns,
		productID, req.SkuCode, specJSON, req.PriceCents, req.CostCents, req.Stock, req.AlertStock,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create SKU", err)
	}
	return sku, nil
}

// ListSKUs returns all SKUs for a product.
func (s *Service) ListSKUs(ctx context.Context, productID, merchantID int64) ([]ProductSku, error) {
	// Verify product belongs to merchant.
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		productID, merchantID,
	).Scan(&exists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify product", err)
	}
	if !exists {
		return nil, apperrors.NewNotFoundError("product not found")
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+skuColumns+` FROM product_skus
		 WHERE product_id = $1 AND deleted_at IS NULL ORDER BY id ASC`,
		productID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list SKUs", err)
	}
	defer rows.Close()

	var skus []ProductSku
	for rows.Next() {
		sku, err := scanSkuRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan SKU", err)
		}
		skus = append(skus, *sku)
	}
	if skus == nil {
		skus = []ProductSku{}
	}
	return skus, rows.Err()
}

// GetSKU returns a single SKU by ID.
func (s *Service) GetSKU(ctx context.Context, skuID, merchantID int64) (*ProductSku, error) {
	sku, err := scanSkuRow(s.db.QueryRowContext(ctx,
		`SELECT `+skuColumns+` FROM product_skus
		 WHERE id = $1 AND deleted_at IS NULL`,
		skuID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("SKU not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get SKU", err)
	}

	// Verify product belongs to merchant.
	var ownerID int64
	err = s.db.QueryRowContext(ctx,
		`SELECT merchant_id FROM products WHERE id = $1 AND deleted_at IS NULL`, sku.ProductID,
	).Scan(&ownerID)
	if err != nil || ownerID != merchantID {
		return nil, apperrors.NewNotFoundError("SKU not found")
	}

	return sku, nil
}

// UpdateSKU updates a SKU's fields. Only non-nil fields are applied.
func (s *Service) UpdateSKU(ctx context.Context, skuID, merchantID int64, req UpdateSkuRequest) (*ProductSku, error) {
	existing, err := s.GetSKU(ctx, skuID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.SkuCode != nil {
		existing.SkuCode = *req.SkuCode
	}
	if req.SpecInfo != nil {
		existing.SpecInfo = req.SpecInfo
	}
	if req.PriceCents != nil {
		if *req.PriceCents < 0 {
			return nil, apperrors.NewValidationError("price_cents must be non-negative")
		}
		existing.PriceCents = *req.PriceCents
	}
	if req.CostCents != nil {
		existing.CostCents = *req.CostCents
	}
	if req.Stock != nil {
		if *req.Stock < 0 {
			return nil, apperrors.NewValidationError("stock must be non-negative")
		}
		existing.Stock = *req.Stock
	}
	if req.AlertStock != nil {
		existing.AlertStock = *req.AlertStock
	}

	specJSON, _ := json.Marshal(existing.SpecInfo)
	sku, err := scanSkuRow(s.db.QueryRowContext(ctx,
		`UPDATE product_skus SET sku_code = $1, spec_info = $2, price_cents = $3, cost_cents = $4,
		 stock = $5, alert_stock = $6, updated_at = NOW()
		 WHERE id = $7 AND deleted_at IS NULL
		 RETURNING `+skuColumns,
		existing.SkuCode, specJSON, existing.PriceCents, existing.CostCents,
		existing.Stock, existing.AlertStock, skuID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update SKU", err)
	}
	return sku, nil
}

// DeleteSKU soft-deletes a SKU after checking stock.
func (s *Service) DeleteSKU(ctx context.Context, skuID, merchantID int64) error {
	sku, err := s.GetSKU(ctx, skuID, merchantID)
	if err != nil {
		return err
	}

	if sku.Stock > 0 {
		return apperrors.NewValidationError("cannot delete SKU with remaining stock, please clear inventory first")
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE product_skus SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		skuID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete SKU", err)
	}
	return nil
}

// ProductDetail wraps a product with its SKUs.
type ProductDetail struct {
	Product
	Skus []ProductSku `json:"skus"`
}

// GetByIDWithSKUs returns a product with all its SKUs.
func (s *Service) GetByIDWithSKUs(ctx context.Context, productID, merchantID int64) (*ProductDetail, error) {
	p, err := s.GetByID(ctx, productID, merchantID)
	if err != nil {
		return nil, err
	}
	skus, err := s.ListSKUs(ctx, productID, merchantID)
	if err != nil {
		return nil, err
	}
	return &ProductDetail{Product: *p, Skus: skus}, nil
}

// SKUInfoForCheckout returns price, stock, and name info for a SKU used in checkout.
func (s *Service) SKUInfoForCheckout(ctx context.Context, skuID int64) (priceCents int, stock int, specInfo map[string]string, err error) {
	var specJSON []byte
	err = s.db.QueryRowContext(ctx,
		`SELECT price_cents, stock, spec_info FROM product_skus
		 WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`,
		skuID,
	).Scan(&priceCents, &stock, &specJSON)
	if err != nil {
		return 0, 0, nil, err
	}
	if specJSON != nil {
		json.Unmarshal(specJSON, &specInfo)
	}
	return priceCents, stock, specInfo, nil
}

// ToggleSKUStatus toggles a SKU between active and inactive.
func (s *Service) ToggleSKUStatus(ctx context.Context, skuID, merchantID int64) (*ProductSku, error) {
	if _, err := s.GetSKU(ctx, skuID, merchantID); err != nil {
		return nil, err
	}

	sku, err := scanSkuRow(s.db.QueryRowContext(ctx,
		`UPDATE product_skus SET status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING `+skuColumns,
		skuID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle SKU status", err)
	}
	return sku, nil
}
