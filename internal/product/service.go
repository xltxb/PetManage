package product

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Product represents a product record.
type Product struct {
	ID         int64      `json:"id"`
	MerchantID int64      `json:"merchant_id"`
	Barcode    string     `json:"barcode"`
	Name       string     `json:"name"`
	PriceCents int        `json:"price_cents"`
	CostCents  int        `json:"cost_cents"`
	Stock      int        `json:"stock"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// CreateProductRequest is the request body for creating a product.
type CreateProductRequest struct {
	Barcode    string `json:"barcode"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	CostCents  int    `json:"cost_cents"`
	Stock      int    `json:"stock"`
}

// Service provides product management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new product Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
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

	p := &Product{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO products (merchant_id, barcode, name, price_cents, cost_cents, stock, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active')
		 RETURNING id, merchant_id, barcode, name, price_cents, cost_cents, stock, status, created_at, updated_at`,
		merchantID, req.Barcode, req.Name, req.PriceCents, req.CostCents, req.Stock,
	).Scan(&p.ID, &p.MerchantID, &p.Barcode, &p.Name, &p.PriceCents, &p.CostCents, &p.Stock, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create product", err)
	}
	return p, nil
}

// List returns products for a merchant, optionally filtered by status.
func (s *Service) List(ctx context.Context, merchantID int64, status string) ([]Product, error) {
	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, merchant_id, barcode, name, price_cents, cost_cents, stock, status, created_at, updated_at
			 FROM products WHERE merchant_id = $1 AND status = $2 AND deleted_at IS NULL
			 ORDER BY created_at DESC`, merchantID, status)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, merchant_id, barcode, name, price_cents, cost_cents, stock, status, created_at, updated_at
			 FROM products WHERE merchant_id = $1 AND deleted_at IS NULL
			 ORDER BY created_at DESC`, merchantID)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list products", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.MerchantID, &p.Barcode, &p.Name, &p.PriceCents, &p.CostCents, &p.Stock, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan product", err)
		}
		products = append(products, p)
	}
	if products == nil {
		products = []Product{}
	}
	return products, rows.Err()
}

// ToggleStatus toggles a product between active and inactive.
func (s *Service) ToggleStatus(ctx context.Context, productID, merchantID int64) (*Product, error) {
	p := &Product{}
	err := s.db.QueryRowContext(ctx,
		`UPDATE products SET status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING id, merchant_id, barcode, name, price_cents, cost_cents, stock, status, created_at, updated_at`,
		productID, merchantID,
	).Scan(&p.ID, &p.MerchantID, &p.Barcode, &p.Name, &p.PriceCents, &p.CostCents, &p.Stock, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("product not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle product status", err)
	}
	return p, nil
}
