package category

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Category represents a product category node.
type Category struct {
	ID         int64       `json:"id"`
	MerchantID int64       `json:"merchant_id"`
	ParentID   *int64      `json:"parent_id"`
	Name       string      `json:"name"`
	SortOrder  int         `json:"sort_order"`
	Children   []*Category `json:"children,omitempty"`
	CreatedAt  string      `json:"created_at"`
	UpdatedAt  string      `json:"updated_at"`
}

// CreateCategoryRequest is the request body for creating a category.
type CreateCategoryRequest struct {
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// UpdateCategoryRequest is the request body for updating a category.
type UpdateCategoryRequest struct {
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// Service provides product category management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new category Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create creates a new product category for a merchant.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateCategoryRequest) (*Category, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("category name is required")
	}

	if req.ParentID != nil && *req.ParentID > 0 {
		// Verify parent exists and belongs to this merchant.
		var exists bool
		err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM product_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
			*req.ParentID, merchantID,
		).Scan(&exists)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check parent category", err)
		}
		if !exists {
			return nil, apperrors.NewNotFoundError("parent category not found")
		}
	}

	var c Category
	var parentID sql.NullInt64
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO product_categories (merchant_id, parent_id, name, sort_order)
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

// List returns all categories for a merchant as a tree.
func (s *Service) List(ctx context.Context, merchantID int64) ([]*Category, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, parent_id, name, sort_order, created_at, updated_at
		 FROM product_categories
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY sort_order ASC, id ASC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list categories", err)
	}
	defer rows.Close()

	var flat []*Category
	for rows.Next() {
		var c Category
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
		return []*Category{}, rows.Err()
	}

	return buildTree(flat), rows.Err()
}

// Update updates a category's name, parent, and sort order.
func (s *Service) Update(ctx context.Context, categoryID, merchantID int64, req UpdateCategoryRequest) (*Category, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("category name is required")
	}

	// Verify category exists and belongs to this merchant.
	var c Category
	var parentID, newParentID sql.NullInt64
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, parent_id, name, sort_order, created_at, updated_at
		 FROM product_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
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
			`SELECT EXISTS(SELECT 1 FROM product_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
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
		`UPDATE product_categories SET name = $1, parent_id = $2, sort_order = $3, updated_at = NOW()
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

// Delete soft-deletes a category after checking for associated products and children.
func (s *Service) Delete(ctx context.Context, categoryID, merchantID int64) error {
	// Verify category exists.
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM product_categories WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL)`,
		categoryID, merchantID,
	).Scan(&exists)
	if err != nil {
		return apperrors.NewInternalError("failed to check category", err)
	}
	if !exists {
		return apperrors.NewNotFoundError("category not found")
	}

	// Check for associated products.
	var productCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM products WHERE category_id = $1 AND deleted_at IS NULL`,
		categoryID,
	).Scan(&productCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check associated products", err)
	}
	if productCount > 0 {
		return apperrors.NewValidationError("cannot delete category: there are products associated with this category")
	}

	// Check for child categories.
	var childCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM product_categories WHERE parent_id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		categoryID, merchantID,
	).Scan(&childCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check child categories", err)
	}
	if childCount > 0 {
		return apperrors.NewValidationError("cannot delete category: there are sub-categories under this category")
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE product_categories SET deleted_at = NOW() WHERE id = $1 AND merchant_id = $2`,
		categoryID, merchantID,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete category", err)
	}
	return nil
}

// buildTree constructs a tree of categories from a flat list.
func buildTree(flat []*Category) []*Category {
	byID := make(map[int64]*Category)
	var roots []*Category

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
