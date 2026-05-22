package product

import (
	"context"
	"database/sql"
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
