package servicepackage

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// ServicePackage represents a combo package bundling multiple service items.
type ServicePackage struct {
	ID                int64              `json:"id"`
	MerchantID        int64              `json:"merchant_id"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	TotalPriceCents   int                `json:"total_price_cents"`
	OriginalPriceCents int               `json:"original_price_cents"`
	ValidDays         int                `json:"valid_days"`
	UsageLimit        int                `json:"usage_limit"`
	Status            string             `json:"status"`
	Items             []PackageItem      `json:"items,omitempty"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
}

// PackageItem represents a service item within a package.
type PackageItem struct {
	ID            int64  `json:"id"`
	PackageID     int64  `json:"package_id"`
	ServiceItemID int64  `json:"service_item_id"`
	ServiceName   string `json:"service_name"`
	PriceCents    int    `json:"price_cents"`
	SortOrder     int    `json:"sort_order"`
}

// CreatePackageRequest is the request body for creating a service package.
type CreatePackageRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TotalPriceCents int `json:"total_price_cents"`
	ValidDays   int    `json:"valid_days"`
	UsageLimit  int    `json:"usage_limit"`
	ItemIDs     []int64 `json:"item_ids"`
}

// UpdatePackageRequest is the request body for updating a service package.
type UpdatePackageRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	TotalPriceCents *int `json:"total_price_cents"`
	ValidDays   *int   `json:"valid_days"`
	UsageLimit  *int   `json:"usage_limit"`
	ItemIDs     *[]int64 `json:"item_ids"`
}

// ListParams holds optional filters for listing packages.
type ListParams struct {
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// ListResult wraps the packages list with pagination.
type ListResult struct {
	Items    []ServicePackage `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// Service provides service package management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const packageColumns = `id, merchant_id, name, description, total_price_cents, original_price_cents, valid_days, usage_limit, status, created_at, updated_at`

func scanPackage(row *sql.Row) (*ServicePackage, error) {
	p := &ServicePackage{}
	var createdAt, updatedAt time.Time
	err := row.Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description,
		&p.TotalPriceCents, &p.OriginalPriceCents, &p.ValidDays, &p.UsageLimit, &p.Status,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return p, nil
}

func scanPackageRows(rows *sql.Rows) (*ServicePackage, error) {
	p := &ServicePackage{}
	var createdAt, updatedAt time.Time
	err := rows.Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description,
		&p.TotalPriceCents, &p.OriginalPriceCents, &p.ValidDays, &p.UsageLimit, &p.Status,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return p, nil
}

// CreatePackage creates a new service package with its items.
func (s *Service) CreatePackage(ctx context.Context, merchantID int64, req CreatePackageRequest) (*ServicePackage, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("package name is required")
	}
	if len(req.ItemIDs) == 0 {
		return nil, apperrors.NewValidationError("at least one service item is required")
	}
	if req.TotalPriceCents <= 0 {
		return nil, apperrors.NewValidationError("total_price_cents must be positive")
	}
	if req.ValidDays < 0 {
		return nil, apperrors.NewValidationError("valid_days must be non-negative")
	}
	if req.UsageLimit < 0 {
		return nil, apperrors.NewValidationError("usage_limit must be non-negative")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// Calculate original price = sum of all service item prices.
	var originalPrice int
	for _, itemID := range req.ItemIDs {
		var price int
		err := tx.QueryRowContext(ctx,
			`SELECT price_cents FROM service_items WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL AND status = 'active'`,
			itemID, merchantID,
		).Scan(&price)
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFoundError("service item not found or inactive: " + strconv.FormatInt(itemID, 10))
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to verify service item", err)
		}
		originalPrice += price
	}

	var p ServicePackage
	var createdAt, updatedAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO service_packages (merchant_id, name, description, total_price_cents, original_price_cents, valid_days, usage_limit, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		 RETURNING `+packageColumns,
		merchantID, req.Name, req.Description, req.TotalPriceCents, originalPrice, req.ValidDays, req.UsageLimit,
	).Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description,
		&p.TotalPriceCents, &p.OriginalPriceCents, &p.ValidDays, &p.UsageLimit, &p.Status,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create package", err)
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	// Insert package items.
	for i, itemID := range req.ItemIDs {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO service_package_items (package_id, service_item_id, sort_order) VALUES ($1, $2, $3)`,
			p.ID, itemID, i+1,
		)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to insert package items", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	// Load items for response.
	p.Items, _ = s.getPackageItems(ctx, p.ID)
	return &p, nil
}

func (s *Service) getPackageItems(ctx context.Context, packageID int64) ([]PackageItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pi.id, pi.package_id, pi.service_item_id, si.name, si.price_cents, pi.sort_order
		 FROM service_package_items pi
		 JOIN service_items si ON si.id = pi.service_item_id AND si.deleted_at IS NULL
		 WHERE pi.package_id = $1
		 ORDER BY pi.sort_order ASC`,
		packageID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query package items", err)
	}
	defer rows.Close()

	var items []PackageItem
	for rows.Next() {
		var it PackageItem
		if err := rows.Scan(&it.ID, &it.PackageID, &it.ServiceItemID, &it.ServiceName, &it.PriceCents, &it.SortOrder); err != nil {
			return nil, apperrors.NewInternalError("failed to scan package item", err)
		}
		items = append(items, it)
	}
	if items == nil {
		items = []PackageItem{}
	}
	return items, rows.Err()
}

// GetPackage returns a single package with its items.
func (s *Service) GetPackage(ctx context.Context, packageID, merchantID int64) (*ServicePackage, error) {
	p, err := scanPackage(s.db.QueryRowContext(ctx,
		`SELECT `+packageColumns+` FROM service_packages
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		packageID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("package not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get package", err)
	}

	items, err := s.getPackageItems(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Items = items
	return p, nil
}

// ListPackages returns packages with optional filtering and pagination.
func (s *Service) ListPackages(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
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
		conditions = append(conditions, "name ILIKE $"+strconv.Itoa(argIdx))
		args = append(args, kw)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_packages WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count packages", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+packageColumns+` FROM service_packages WHERE `+whereClause+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list packages", err)
	}
	defer rows.Close()

	var items []ServicePackage
	for rows.Next() {
		p, err := scanPackageRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan package", err)
		}
		// Load items for each package.
		p.Items, _ = s.getPackageItems(ctx, p.ID)
		items = append(items, *p)
	}
	if items == nil {
		items = []ServicePackage{}
	}

	return &ListResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// UpdatePackage updates a package and its items.
func (s *Service) UpdatePackage(ctx context.Context, packageID, merchantID int64, req UpdatePackageRequest) (*ServicePackage, error) {
	existing, err := s.GetPackage(ctx, packageID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("package name is required")
		}
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.TotalPriceCents != nil {
		if *req.TotalPriceCents <= 0 {
			return nil, apperrors.NewValidationError("total_price_cents must be positive")
		}
		existing.TotalPriceCents = *req.TotalPriceCents
	}
	if req.ValidDays != nil {
		if *req.ValidDays < 0 {
			return nil, apperrors.NewValidationError("valid_days must be non-negative")
		}
		existing.ValidDays = *req.ValidDays
	}
	if req.UsageLimit != nil {
		if *req.UsageLimit < 0 {
			return nil, apperrors.NewValidationError("usage_limit must be non-negative")
		}
		existing.UsageLimit = *req.UsageLimit
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// If items are being updated, recalculate original price.
	if req.ItemIDs != nil {
		if len(*req.ItemIDs) == 0 {
			return nil, apperrors.NewValidationError("at least one service item is required")
		}

		var originalPrice int
		for _, itemID := range *req.ItemIDs {
			var price int
			err := tx.QueryRowContext(ctx,
				`SELECT price_cents FROM service_items WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL AND status = 'active'`,
				itemID, merchantID,
			).Scan(&price)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("service item not found or inactive: " + strconv.FormatInt(itemID, 10))
			}
			if err != nil {
				return nil, apperrors.NewInternalError("failed to verify service item", err)
			}
			originalPrice += price
		}
		existing.OriginalPriceCents = originalPrice

		// Remove old items and insert new ones.
		_, err = tx.ExecContext(ctx, `DELETE FROM service_package_items WHERE package_id = $1`, packageID)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to remove old items", err)
		}
		for i, itemID := range *req.ItemIDs {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO service_package_items (package_id, service_item_id, sort_order) VALUES ($1, $2, $3)`,
				packageID, itemID, i+1,
			)
			if err != nil {
				return nil, apperrors.NewInternalError("failed to insert package items", err)
			}
		}
	}

	var createdAt, updatedAt time.Time
	var p ServicePackage
	err = tx.QueryRowContext(ctx,
		`UPDATE service_packages SET
		 name = $1, description = $2, total_price_cents = $3, original_price_cents = $4,
		 valid_days = $5, usage_limit = $6, updated_at = NOW()
		 WHERE id = $7 AND merchant_id = $8 AND deleted_at IS NULL
		 RETURNING `+packageColumns,
		existing.Name, existing.Description, existing.TotalPriceCents, existing.OriginalPriceCents,
		existing.ValidDays, existing.UsageLimit, packageID, merchantID,
	).Scan(&p.ID, &p.MerchantID, &p.Name, &p.Description,
		&p.TotalPriceCents, &p.OriginalPriceCents, &p.ValidDays, &p.UsageLimit, &p.Status,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update package", err)
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("failed to commit transaction", err)
	}

	// Load items for response.
	p.Items, _ = s.getPackageItems(ctx, p.ID)
	return &p, nil
}

// DeletePackage soft-deletes a package.
func (s *Service) DeletePackage(ctx context.Context, packageID, merchantID int64) error {
	_, err := s.GetPackage(ctx, packageID, merchantID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE service_packages SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		packageID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete package", err)
	}
	return nil
}

// TogglePackageStatus toggles a package between active and inactive.
func (s *Service) TogglePackageStatus(ctx context.Context, packageID, merchantID int64) (*ServicePackage, error) {
	p, err := scanPackage(s.db.QueryRowContext(ctx,
		`UPDATE service_packages SET status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING `+packageColumns,
		packageID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("package not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle package status", err)
	}

	p.Items, _ = s.getPackageItems(ctx, p.ID)
	return p, nil
}

// GetPackageItems returns the individual service items for a package.
func (s *Service) GetPackageItems(ctx context.Context, packageID, merchantID int64) ([]PackageItem, error) {
	// Verify package exists and belongs to merchant.
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM service_packages WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		packageID, merchantID,
	).Scan(&exists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify package", err)
	}
	if !exists {
		return nil, apperrors.NewNotFoundError("package not found")
	}

	return s.getPackageItems(ctx, packageID)
}
