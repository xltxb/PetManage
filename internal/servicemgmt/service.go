package servicemgmt

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// ServiceCategory represents a service category node.
type ServiceCategory struct {
	ID         int64              `json:"id"`
	MerchantID int64              `json:"merchant_id"`
	ParentID   *int64             `json:"parent_id"`
	Name       string             `json:"name"`
	SortOrder  int                `json:"sort_order"`
	Children   []*ServiceCategory `json:"children,omitempty"`
	CreatedAt  string             `json:"created_at"`
	UpdatedAt  string             `json:"updated_at"`
}

// CreateServiceCategoryRequest is the request body for creating a service category.
type CreateServiceCategoryRequest struct {
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// UpdateServiceCategoryRequest is the request body for updating a service category.
type UpdateServiceCategoryRequest struct {
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// ServiceItem represents a service item.
type ServiceItem struct {
	ID               int64   `json:"id"`
	MerchantID       int64   `json:"merchant_id"`
	CategoryID       int64   `json:"category_id"`
	Name             string  `json:"name"`
	DurationMinutes  int     `json:"duration_minutes"`
	PriceCents       int     `json:"price_cents"`
	MemberPriceCents int     `json:"member_price_cents"`
	PetType          string  `json:"pet_type"`
	MinWeightKg      float64 `json:"min_weight_kg"`
	MaxWeightKg      float64 `json:"max_weight_kg"`
	Materials        string  `json:"materials"`
	CostCents        int     `json:"cost_cents"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// CreateServiceItemRequest is the request body for creating a service item.
type CreateServiceItemRequest struct {
	CategoryID       int64   `json:"category_id"`
	Name             string  `json:"name"`
	DurationMinutes  int     `json:"duration_minutes"`
	PriceCents       int     `json:"price_cents"`
	MemberPriceCents int     `json:"member_price_cents"`
	PetType          string  `json:"pet_type"`
	MinWeightKg      float64 `json:"min_weight_kg"`
	MaxWeightKg      float64 `json:"max_weight_kg"`
	Materials        string  `json:"materials"`
	CostCents        int     `json:"cost_cents"`
}

// UpdateServiceItemRequest is the request body for updating a service item.
type UpdateServiceItemRequest struct {
	Name             *string  `json:"name"`
	DurationMinutes  *int     `json:"duration_minutes"`
	PriceCents       *int     `json:"price_cents"`
	MemberPriceCents *int     `json:"member_price_cents"`
	PetType          *string  `json:"pet_type"`
	MinWeightKg      *float64 `json:"min_weight_kg"`
	MaxWeightKg      *float64 `json:"max_weight_kg"`
	Materials        *string  `json:"materials"`
	CostCents        *int     `json:"cost_cents"`
	CategoryID       *int64   `json:"category_id"`
}

// ListItemsParams holds optional filters for listing service items.
type ListItemsParams struct {
	CategoryID *int64
	Status     string
	Keyword    string
	Page       int
	PageSize   int
}

// ListItemsResult wraps the service items list with pagination.
type ListItemsResult struct {
	Items    []ServiceItem `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// Service provides service category and item management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// --- Service Categories ---

// CreateCategory creates a new service category for a merchant.
func (s *Service) CreateCategory(ctx context.Context, merchantID int64, req CreateServiceCategoryRequest) (*ServiceCategory, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("category name is required")
	}
	if req.ParentID != nil && *req.ParentID > 0 {
		var exists bool
		err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM service_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
			*req.ParentID, merchantID,
		).Scan(&exists)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check parent category", err)
		}
		if !exists {
			return nil, apperrors.NewNotFoundError("parent category not found")
		}
	}

	var c ServiceCategory
	var parentID sql.NullInt64
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO service_categories (merchant_id, parent_id, name, sort_order)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, merchant_id, parent_id, name, sort_order, created_at, updated_at`,
		merchantID, req.ParentID, req.Name, req.SortOrder,
	).Scan(&c.ID, &c.MerchantID, &parentID, &c.Name, &c.SortOrder, &createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create category", err)
	}
	if parentID.Valid {
		c.ParentID = &parentID.Int64
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &c, nil
}

// ListCategories returns all service categories for a merchant as a tree.
func (s *Service) ListCategories(ctx context.Context, merchantID int64) ([]*ServiceCategory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, parent_id, name, sort_order, created_at, updated_at
		 FROM service_categories
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY sort_order ASC, id ASC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list categories", err)
	}
	defer rows.Close()

	var flat []*ServiceCategory
	for rows.Next() {
		var c ServiceCategory
		var parentID sql.NullInt64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&c.ID, &c.MerchantID, &parentID, &c.Name, &c.SortOrder, &createdAt, &updatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan category", err)
		}
		if parentID.Valid {
			c.ParentID = &parentID.Int64
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)
		flat = append(flat, &c)
	}
	if flat == nil {
		return []*ServiceCategory{}, rows.Err()
	}
	return buildCategoryTree(flat), rows.Err()
}

// UpdateCategory updates a service category.
func (s *Service) UpdateCategory(ctx context.Context, categoryID, merchantID int64, req UpdateServiceCategoryRequest) (*ServiceCategory, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("category name is required")
	}

	var c ServiceCategory
	var parentID, newParentID sql.NullInt64
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, parent_id, name, sort_order, created_at, updated_at
		 FROM service_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		categoryID, merchantID,
	).Scan(&c.ID, &c.MerchantID, &parentID, &c.Name, &c.SortOrder, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("category not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to find category", err)
	}

	if req.ParentID != nil && *req.ParentID > 0 {
		if *req.ParentID == categoryID {
			return nil, apperrors.NewValidationError("category cannot be its own parent")
		}
		var exists bool
		err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM service_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
			*req.ParentID, merchantID,
		).Scan(&exists)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check parent category", err)
		}
		if !exists {
			return nil, apperrors.NewNotFoundError("parent category not found")
		}
	}

	err = s.db.QueryRowContext(ctx,
		`UPDATE service_categories SET name = $1, parent_id = $2, sort_order = $3, updated_at = NOW()
		 WHERE id = $4 AND merchant_id = $5 AND deleted_at IS NULL
		 RETURNING id, merchant_id, parent_id, name, sort_order, created_at, updated_at`,
		req.Name, req.ParentID, req.SortOrder, categoryID, merchantID,
	).Scan(&c.ID, &c.MerchantID, &newParentID, &c.Name, &c.SortOrder, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("category not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update category", err)
	}

	if newParentID.Valid {
		c.ParentID = &newParentID.Int64
	} else {
		c.ParentID = nil
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &c, nil
}

// DeleteCategory soft-deletes a category after checking for items and children.
func (s *Service) DeleteCategory(ctx context.Context, categoryID, merchantID int64) error {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM service_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		categoryID, merchantID,
	).Scan(&exists)
	if err != nil {
		return apperrors.NewInternalError("failed to check category", err)
	}
	if !exists {
		return apperrors.NewNotFoundError("category not found")
	}

	var itemCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_items WHERE category_id = $1 AND deleted_at IS NULL`,
		categoryID,
	).Scan(&itemCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check associated items", err)
	}
	if itemCount > 0 {
		return apperrors.NewValidationError("cannot delete category: there are service items associated with this category")
	}

	var childCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_categories WHERE parent_id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		categoryID, merchantID,
	).Scan(&childCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check child categories", err)
	}
	if childCount > 0 {
		return apperrors.NewValidationError("cannot delete category: there are sub-categories under this category")
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE service_categories SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2`,
		categoryID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete category", err)
	}
	return nil
}

func buildCategoryTree(flat []*ServiceCategory) []*ServiceCategory {
	byID := make(map[int64]*ServiceCategory)
	var roots []*ServiceCategory

	for _, c := range flat {
		byID[c.ID] = c
	}
	for _, c := range flat {
		if c.ParentID != nil {
			if parent, ok := byID[*c.ParentID]; ok {
				parent.Children = append(parent.Children, c)
			} else {
				roots = append(roots, c)
			}
		} else {
			roots = append(roots, c)
		}
	}
	return roots
}

// --- Service Items ---

const serviceItemColumns = `id, merchant_id, category_id, name, duration_minutes, price_cents, member_price_cents, pet_type, min_weight_kg, max_weight_kg, materials, cost_cents, status, created_at, updated_at`

func scanItemRow(row *sql.Row) (*ServiceItem, error) {
	it := &ServiceItem{}
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&it.ID, &it.MerchantID, &it.CategoryID, &it.Name, &it.DurationMinutes,
		&it.PriceCents, &it.MemberPriceCents, &it.PetType,
		&it.MinWeightKg, &it.MaxWeightKg, &it.Materials, &it.CostCents, &it.Status,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = createdAt.Format(time.RFC3339)
	it.UpdatedAt = updatedAt.Format(time.RFC3339)
	return it, nil
}

func scanItemRows(rows *sql.Rows) (*ServiceItem, error) {
	it := &ServiceItem{}
	var createdAt, updatedAt time.Time
	err := rows.Scan(
		&it.ID, &it.MerchantID, &it.CategoryID, &it.Name, &it.DurationMinutes,
		&it.PriceCents, &it.MemberPriceCents, &it.PetType,
		&it.MinWeightKg, &it.MaxWeightKg, &it.Materials, &it.CostCents, &it.Status,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = createdAt.Format(time.RFC3339)
	it.UpdatedAt = updatedAt.Format(time.RFC3339)
	return it, nil
}

// CreateItem creates a new service item.
func (s *Service) CreateItem(ctx context.Context, merchantID int64, req CreateServiceItemRequest) (*ServiceItem, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("service item name is required")
	}
	if req.CategoryID <= 0 {
		return nil, apperrors.NewValidationError("category_id is required")
	}
	// Verify category exists.
	var catExists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM service_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		req.CategoryID, merchantID,
	).Scan(&catExists)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to verify category", err)
	}
	if !catExists {
		return nil, apperrors.NewNotFoundError("category not found")
	}

	if req.DurationMinutes <= 0 {
		return nil, apperrors.NewValidationError("duration_minutes must be positive")
	}
	if req.PriceCents < 0 {
		return nil, apperrors.NewValidationError("price_cents must be non-negative")
	}
	if req.MinWeightKg < 0 || req.MaxWeightKg < 0 {
		return nil, apperrors.NewValidationError("weight range must be non-negative")
	}

	it, err := scanItemRow(s.db.QueryRowContext(ctx,
		`INSERT INTO service_items (merchant_id, category_id, name, duration_minutes, price_cents, member_price_cents, pet_type, min_weight_kg, max_weight_kg, materials, cost_cents, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active')
		 RETURNING `+serviceItemColumns,
		merchantID, req.CategoryID, req.Name, req.DurationMinutes, req.PriceCents, req.MemberPriceCents,
		req.PetType, req.MinWeightKg, req.MaxWeightKg, req.Materials, req.CostCents,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create service item", err)
	}
	return it, nil
}

// GetItem returns a single service item by ID.
func (s *Service) GetItem(ctx context.Context, itemID, merchantID int64) (*ServiceItem, error) {
	it, err := scanItemRow(s.db.QueryRowContext(ctx,
		`SELECT `+serviceItemColumns+` FROM service_items
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		itemID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("service item not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get service item", err)
	}
	return it, nil
}

// UpdateItem updates a service item's fields.
func (s *Service) UpdateItem(ctx context.Context, itemID, merchantID int64, req UpdateServiceItemRequest) (*ServiceItem, error) {
	existing, err := s.GetItem(ctx, itemID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("service item name is required")
		}
		existing.Name = *req.Name
	}
	if req.DurationMinutes != nil {
		if *req.DurationMinutes <= 0 {
			return nil, apperrors.NewValidationError("duration_minutes must be positive")
		}
		existing.DurationMinutes = *req.DurationMinutes
	}
	if req.PriceCents != nil {
		if *req.PriceCents < 0 {
			return nil, apperrors.NewValidationError("price_cents must be non-negative")
		}
		existing.PriceCents = *req.PriceCents
	}
	if req.MemberPriceCents != nil {
		existing.MemberPriceCents = *req.MemberPriceCents
	}
	if req.PetType != nil {
		existing.PetType = *req.PetType
	}
	if req.MinWeightKg != nil {
		if *req.MinWeightKg < 0 {
			return nil, apperrors.NewValidationError("min_weight_kg must be non-negative")
		}
		existing.MinWeightKg = *req.MinWeightKg
	}
	if req.MaxWeightKg != nil {
		if *req.MaxWeightKg < 0 {
			return nil, apperrors.NewValidationError("max_weight_kg must be non-negative")
		}
		existing.MaxWeightKg = *req.MaxWeightKg
	}
	if req.Materials != nil {
		existing.Materials = *req.Materials
	}
	if req.CostCents != nil {
		existing.CostCents = *req.CostCents
	}
	if req.CategoryID != nil {
		var exists bool
		err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM service_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
			*req.CategoryID, merchantID,
		).Scan(&exists)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to verify category", err)
		}
		if !exists {
			return nil, apperrors.NewNotFoundError("category not found")
		}
		existing.CategoryID = *req.CategoryID
	}

	it, err := scanItemRow(s.db.QueryRowContext(ctx,
		`UPDATE service_items SET
		 name = $1, duration_minutes = $2, price_cents = $3, member_price_cents = $4,
		 pet_type = $5, min_weight_kg = $6, max_weight_kg = $7, materials = $8,
		 cost_cents = $9, category_id = $10, updated_at = NOW()
		 WHERE id = $11 AND merchant_id = $12 AND deleted_at IS NULL
		 RETURNING `+serviceItemColumns,
		existing.Name, existing.DurationMinutes, existing.PriceCents, existing.MemberPriceCents,
		existing.PetType, existing.MinWeightKg, existing.MaxWeightKg, existing.Materials,
		existing.CostCents, existing.CategoryID, itemID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update service item", err)
	}
	return it, nil
}

// DeleteItem soft-deletes a service item.
func (s *Service) DeleteItem(ctx context.Context, itemID, merchantID int64) error {
	_, err := s.GetItem(ctx, itemID, merchantID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE service_items SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		itemID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete service item", err)
	}
	return nil
}

// ListItems returns service items with optional filtering and pagination.
func (s *Service) ListItems(ctx context.Context, merchantID int64, params ListItemsParams) (*ListItemsResult, error) {
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

	if params.CategoryID != nil && *params.CategoryID > 0 {
		conditions = append(conditions, "category_id = $"+strconv.Itoa(argIdx))
		args = append(args, *params.CategoryID)
		argIdx++
	}
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
		`SELECT COUNT(*) FROM service_items WHERE `+whereClause,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count items", err)
	}

	offset := (params.Page - 1) * params.PageSize
	queryArgs := append(args, params.PageSize, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+serviceItemColumns+` FROM service_items WHERE `+whereClause+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list items", err)
	}
	defer rows.Close()

	var items []ServiceItem
	for rows.Next() {
		it, err := scanItemRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan item", err)
		}
		items = append(items, *it)
	}
	if items == nil {
		items = []ServiceItem{}
	}

	return &ListItemsResult{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, rows.Err()
}

// ToggleItemStatus toggles a service item between active and inactive.
func (s *Service) ToggleItemStatus(ctx context.Context, itemID, merchantID int64) (*ServiceItem, error) {
	it, err := scanItemRow(s.db.QueryRowContext(ctx,
		`UPDATE service_items SET status = CASE WHEN status = 'active' THEN 'inactive' ELSE 'active' END,
		 updated_at = NOW()
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL
		 RETURNING `+serviceItemColumns,
		itemID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("service item not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle status", err)
	}
	return it, nil
}
