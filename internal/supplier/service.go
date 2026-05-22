package supplier

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Supplier represents a supplier record.
type Supplier struct {
	ID              int64      `json:"id"`
	MerchantID      int64      `json:"merchant_id"`
	Name            string     `json:"name"`
	ContactPerson   string     `json:"contact_person"`
	ContactPhone    string     `json:"contact_phone"`
	ContactEmail    string     `json:"contact_email"`
	Address         string     `json:"address"`
	SettlementCycle string     `json:"settlement_cycle"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

// LinkedProduct represents a product linked to a supplier.
type LinkedProduct struct {
	ProductID   int64  `json:"product_id"`
	Barcode     string `json:"barcode"`
	Name        string `json:"name"`
	PriceCents  int    `json:"price_cents"`
	Stock       int    `json:"stock"`
	Status      string `json:"status"`
	LinkedAt    string `json:"linked_at"`
}

// CreateSupplierRequest is the request body for creating a supplier.
type CreateSupplierRequest struct {
	Name            string `json:"name"`
	ContactPerson   string `json:"contact_person"`
	ContactPhone    string `json:"contact_phone"`
	ContactEmail    string `json:"contact_email"`
	Address         string `json:"address"`
	SettlementCycle string `json:"settlement_cycle"`
	Notes           string `json:"notes"`
}

// UpdateSupplierRequest is the request body for updating a supplier (partial).
type UpdateSupplierRequest struct {
	Name            *string `json:"name"`
	ContactPerson   *string `json:"contact_person"`
	ContactPhone    *string `json:"contact_phone"`
	ContactEmail    *string `json:"contact_email"`
	Address         *string `json:"address"`
	SettlementCycle *string `json:"settlement_cycle"`
	Notes           *string `json:"notes"`
}

// LinkProductRequest is the request body for linking a product to a supplier.
type LinkProductRequest struct {
	ProductID int64 `json:"product_id"`
}

// ListParams holds optional filters and pagination for listing suppliers.
type ListParams struct {
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// ListResult wraps the suppliers list with pagination info.
type ListResult struct {
	Suppliers []Supplier `json:"suppliers"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// SupplierDetail includes supplier info plus linked products.
type SupplierDetail struct {
	Supplier Supplier        `json:"supplier"`
	Products []LinkedProduct `json:"products"`
}

// Service provides supplier management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new supplier Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const supplierColumns = `id, merchant_id, name, contact_person, contact_phone, contact_email, address, settlement_cycle, status, notes, created_at, updated_at`

func scanSupplierRow(row *sql.Row) (*Supplier, error) {
	s := &Supplier{}
	err := row.Scan(
		&s.ID, &s.MerchantID, &s.Name, &s.ContactPerson, &s.ContactPhone,
		&s.ContactEmail, &s.Address, &s.SettlementCycle, &s.Status, &s.Notes,
		&s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

func scanSupplierRows(rows *sql.Rows) (*Supplier, error) {
	s := &Supplier{}
	err := rows.Scan(
		&s.ID, &s.MerchantID, &s.Name, &s.ContactPerson, &s.ContactPhone,
		&s.ContactEmail, &s.Address, &s.SettlementCycle, &s.Status, &s.Notes,
		&s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

// Create creates a new supplier for a merchant.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateSupplierRequest) (*Supplier, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("supplier name is required")
	}
	if strings.TrimSpace(req.ContactPerson) == "" {
		return nil, apperrors.NewValidationError("contact person is required")
	}
	if strings.TrimSpace(req.ContactPhone) == "" {
		return nil, apperrors.NewValidationError("contact phone is required")
	}

	sup, err := scanSupplierRow(s.db.QueryRowContext(ctx,
		`INSERT INTO suppliers (merchant_id, name, contact_person, contact_phone, contact_email, address, settlement_cycle, status, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', $8)
		 RETURNING `+supplierColumns,
		merchantID, strings.TrimSpace(req.Name), strings.TrimSpace(req.ContactPerson),
		strings.TrimSpace(req.ContactPhone), req.ContactEmail, req.Address,
		req.SettlementCycle, req.Notes,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create supplier", err)
	}
	return sup, nil
}

// GetByID returns a single supplier by ID, scoped to a merchant.
func (s *Service) GetByID(ctx context.Context, supplierID, merchantID int64) (*Supplier, error) {
	sup, err := scanSupplierRow(s.db.QueryRowContext(ctx,
		`SELECT `+supplierColumns+` FROM suppliers
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		supplierID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("supplier not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get supplier", err)
	}
	return sup, nil
}

// GetDetail returns supplier with linked products.
func (s *Service) GetDetail(ctx context.Context, supplierID, merchantID int64) (*SupplierDetail, error) {
	sup, err := s.GetByID(ctx, supplierID, merchantID)
	if err != nil {
		return nil, err
	}

	products, err := s.GetLinkedProducts(ctx, supplierID)
	if err != nil {
		return nil, err
	}

	return &SupplierDetail{
		Supplier: *sup,
		Products: products,
	}, nil
}

// List returns suppliers for a merchant with optional filters and pagination.
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

	where := "WHERE merchant_id = $1 AND deleted_at IS NULL"
	if params.Status != "" {
		where += " AND status = $" + strconv.Itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Keyword != "" {
		where += " AND name ILIKE $" + strconv.Itoa(argIdx)
		args = append(args, "%"+params.Keyword+"%")
		argIdx++
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM suppliers `+where,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count suppliers", err)
	}

	offset := (page - 1) * pageSize
	queryArgs := append(args, pageSize, offset)
	queryIdx := len(args) + 1
	limitIdx := queryIdx
	offsetIdx := queryIdx + 1

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+supplierColumns+` FROM suppliers `+where+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx),
		append(queryArgs[:len(queryArgs)-2], pageSize, offset)...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list suppliers", err)
	}
	defer rows.Close()

	suppliers := make([]Supplier, 0)
	for rows.Next() {
		sup, err := scanSupplierRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan supplier", err)
		}
		suppliers = append(suppliers, *sup)
	}

	if suppliers == nil {
		suppliers = make([]Supplier, 0)
	}

	return &ListResult{
		Suppliers: suppliers,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

// Update updates supplier fields. Only non-nil fields in the request are applied.
func (s *Service) Update(ctx context.Context, supplierID, merchantID int64, req UpdateSupplierRequest) (*Supplier, error) {
	existing, err := s.GetByID(ctx, supplierID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("supplier name is required")
		}
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if req.ContactPerson != nil {
		if strings.TrimSpace(*req.ContactPerson) == "" {
			return nil, apperrors.NewValidationError("contact person is required")
		}
		existing.ContactPerson = strings.TrimSpace(*req.ContactPerson)
	}
	if req.ContactPhone != nil {
		if strings.TrimSpace(*req.ContactPhone) == "" {
			return nil, apperrors.NewValidationError("contact phone is required")
		}
		existing.ContactPhone = strings.TrimSpace(*req.ContactPhone)
	}
	if req.ContactEmail != nil {
		existing.ContactEmail = *req.ContactEmail
	}
	if req.Address != nil {
		existing.Address = *req.Address
	}
	if req.SettlementCycle != nil {
		existing.SettlementCycle = *req.SettlementCycle
	}
	if req.Notes != nil {
		existing.Notes = *req.Notes
	}

	sup, err := scanSupplierRow(s.db.QueryRowContext(ctx,
		`UPDATE suppliers SET name=$1, contact_person=$2, contact_phone=$3, contact_email=$4, address=$5, settlement_cycle=$6, notes=$7, updated_at=NOW()
		 WHERE id=$8 AND merchant_id=$9 AND deleted_at IS NULL
		 RETURNING `+supplierColumns,
		existing.Name, existing.ContactPerson, existing.ContactPhone, existing.ContactEmail,
		existing.Address, existing.SettlementCycle, existing.Notes,
		supplierID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update supplier", err)
	}
	return sup, nil
}

// ToggleStatus toggles a supplier between active and inactive.
func (s *Service) ToggleStatus(ctx context.Context, supplierID, merchantID int64) (*Supplier, error) {
	existing, err := s.GetByID(ctx, supplierID, merchantID)
	if err != nil {
		return nil, err
	}

	newStatus := "inactive"
	if existing.Status == "inactive" {
		newStatus = "active"
	}

	sup, err := scanSupplierRow(s.db.QueryRowContext(ctx,
		`UPDATE suppliers SET status=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL
		 RETURNING `+supplierColumns,
		newStatus, supplierID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle supplier status", err)
	}
	return sup, nil
}

// LinkProduct links a product to a supplier.
func (s *Service) LinkProduct(ctx context.Context, supplierID, merchantID int64, req LinkProductRequest) error {
	// Verify supplier belongs to this merchant.
	_, err := s.GetByID(ctx, supplierID, merchantID)
	if err != nil {
		return err
	}

	// Verify the product belongs to the same merchant.
	var productMerchantID int64
	err = s.db.QueryRowContext(ctx,
		`SELECT merchant_id FROM products WHERE id=$1 AND deleted_at IS NULL`,
		req.ProductID,
	).Scan(&productMerchantID)
	if err == sql.ErrNoRows {
		return apperrors.NewNotFoundError("product not found")
	}
	if err != nil {
		return apperrors.NewInternalError("failed to verify product", err)
	}
	if productMerchantID != merchantID {
		return apperrors.NewNotFoundError("product not found")
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO supplier_products (supplier_id, product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		supplierID, req.ProductID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to link product", err)
	}

	return nil
}

// UnlinkProduct removes a product from a supplier.
func (s *Service) UnlinkProduct(ctx context.Context, supplierID, productID, merchantID int64) error {
	// Verify supplier belongs to this merchant.
	_, err := s.GetByID(ctx, supplierID, merchantID)
	if err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx,
		`DELETE FROM supplier_products WHERE supplier_id=$1 AND product_id=$2`,
		supplierID, productID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to unlink product", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return apperrors.NewNotFoundError("product link not found")
	}

	return nil
}

// GetLinkedProducts returns products linked to a supplier.
func (s *Service) GetLinkedProducts(ctx context.Context, supplierID int64) ([]LinkedProduct, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.barcode, p.name, p.price_cents, p.stock, p.status, sp.created_at
		 FROM supplier_products sp
		 JOIN products p ON p.id = sp.product_id
		 WHERE sp.supplier_id = $1 AND p.deleted_at IS NULL
		 ORDER BY sp.created_at DESC`,
		supplierID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get linked products", err)
	}
	defer rows.Close()

	products := make([]LinkedProduct, 0)
	for rows.Next() {
		var lp LinkedProduct
		var linkedAt time.Time
		if err := rows.Scan(&lp.ProductID, &lp.Barcode, &lp.Name, &lp.PriceCents, &lp.Stock, &lp.Status, &linkedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan linked product", err)
		}
		lp.LinkedAt = linkedAt.Format(time.RFC3339)
		products = append(products, lp)
	}

	if products == nil {
		products = make([]LinkedProduct, 0)
	}

	return products, nil
}
