package dictionary

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/cache"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

var dictCacheTTL = 10 * time.Minute

// Service manages system data dictionaries (categories, breeds).
type Service struct {
	db    *sql.DB
	cache *cache.RedisClient
}

// NewService creates a new dictionary Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SetCache injects a Redis cache client for read-through caching.
func (s *Service) SetCache(c *cache.RedisClient) {
	s.cache = c
}

func (s *Service) categoriesCacheKey(merchantID int64) string {
	return fmt.Sprintf("cache:dict_categories:%d", merchantID)
}

func (s *Service) breedsCacheKey(petType string, merchantID int64) string {
	return fmt.Sprintf("cache:dict_breeds:%s:%d", petType, merchantID)
}

func (s *Service) invalidateDictCache(ctx context.Context) {
	if s.cache != nil {
		_ = s.cache.InvalidatePattern(ctx, "cache:dict_*")
	}
}

// --- Category types ---

// Category represents a system category node.
type Category struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ParentID   *int64 `json:"parent_id"`
	Level      int    `json:"level"`
	SortOrder  int    `json:"sort_order"`
	Status     string `json:"status"`
	IsPlatform bool   `json:"is_platform"`
	MerchantID *int64 `json:"merchant_id,omitempty"`
	Children   []*Category `json:"children,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// CreateCategoryRequest is the request body for creating a category.
type CreateCategoryRequest struct {
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// CreateCategory creates a new category.
// If merchantID is non-nil and non-zero, the category is a merchant-level custom category.
// Platform admins pass 0 for merchantID to create platform-level categories.
func (s *Service) CreateCategory(ctx context.Context, req CreateCategoryRequest, merchantID int64) (*Category, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("category name is required")
	}

	level := 1
	var parentID *int64
	if req.ParentID != nil && *req.ParentID > 0 {
		parentID = req.ParentID
		// Verify parent exists and get its level.
		var parentLevel int
		var parentPlatform bool
		var parentMerchantID sql.NullInt64
		err := s.db.QueryRowContext(ctx,
			`SELECT level, is_platform, merchant_id FROM system_categories WHERE id = $1 AND deleted_at IS NULL`,
			*parentID,
		).Scan(&parentLevel, &parentPlatform, &parentMerchantID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NewNotFoundError("parent category not found")
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to check parent category", err)
		}
		if parentLevel >= 2 {
			return nil, apperrors.NewValidationError("cannot create child under a level-2 category; max depth is 2")
		}
		level = 2

		// Merchant-created categories can only be under platform-level categories
		// or under their own merchant categories.
		if merchantID > 0 && parentPlatform && parentMerchantID.Valid && parentMerchantID.Int64 > 0 && parentMerchantID.Int64 != merchantID {
			return nil, apperrors.NewForbiddenError("cannot create child under another merchant's category")
		}
	}

	isPlatform := merchantID == 0
	var catMerchantID sql.NullInt64
	if !isPlatform {
		catMerchantID = sql.NullInt64{Int64: merchantID, Valid: true}
	}

	var cat Category
	var createdAt, updatedAt time.Time
	var catParentID, catMerchID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO system_categories (name, parent_id, level, sort_order, status, is_platform, merchant_id)
		 VALUES ($1, $2, $3, $4, 'enabled', $5, $6)
		 RETURNING id, name, parent_id, level, sort_order, status, is_platform, merchant_id, created_at, updated_at`,
		req.Name, parentID, level, req.SortOrder, isPlatform, catMerchantID,
	).Scan(&cat.ID, &cat.Name, &catParentID, &cat.Level, &cat.SortOrder,
		&cat.Status, &cat.IsPlatform, &catMerchID, &createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create category", err)
	}

	if catParentID.Valid {
		cat.ParentID = &catParentID.Int64
	}
	if catMerchID.Valid {
		cat.MerchantID = &catMerchID.Int64
	}
	cat.CreatedAt = createdAt.Format(time.RFC3339)
	cat.UpdatedAt = updatedAt.Format(time.RFC3339)
	cat.Children = []*Category{}

	s.invalidateDictCache(ctx)
	return &cat, nil
}

// ListCategories returns categories as a tree.
// When merchantID > 0, returns platform enabled categories + the merchant's own categories.
// When merchantID == 0 (platform admin), returns all non-deleted categories.
func (s *Service) ListCategories(ctx context.Context, merchantID int64) ([]*Category, error) {
	if s.cache != nil {
		var cached []*Category
		if s.cache.GetJSON(ctx, s.categoriesCacheKey(merchantID), &cached) {
			return cached, nil
		}
	}

	var rows *sql.Rows
	var err error

	if merchantID > 0 {
		// Merchant view: platform enabled + own categories.
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, parent_id, level, sort_order, status, is_platform, merchant_id, created_at, updated_at
			 FROM system_categories
			 WHERE deleted_at IS NULL
			   AND (
			     (is_platform = true AND status = 'enabled')
			     OR (is_platform = false AND merchant_id = $1)
			   )
			 ORDER BY sort_order, id`,
			merchantID,
		)
	} else {
		// Platform admin view: all non-deleted.
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, parent_id, level, sort_order, status, is_platform, merchant_id, created_at, updated_at
			 FROM system_categories
			 WHERE deleted_at IS NULL
			 ORDER BY sort_order, id`,
		)
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list categories", err)
	}
	defer rows.Close()

	nodes := map[int64]*Category{}
	var roots []*Category

	for rows.Next() {
		var cat Category
		var parentID, merchID sql.NullInt64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&cat.ID, &cat.Name, &parentID, &cat.Level, &cat.SortOrder,
			&cat.Status, &cat.IsPlatform, &merchID, &createdAt, &updatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan category", err)
		}
		if parentID.Valid {
			cat.ParentID = &parentID.Int64
		}
		if merchID.Valid {
			cat.MerchantID = &merchID.Int64
		}
		cat.CreatedAt = createdAt.Format(time.RFC3339)
		cat.UpdatedAt = updatedAt.Format(time.RFC3339)
		cat.Children = []*Category{}
		nodes[cat.ID] = &cat
	}

	// Build tree.
	for _, cat := range nodes {
		if cat.ParentID != nil {
			if parent, ok := nodes[*cat.ParentID]; ok {
				parent.Children = append(parent.Children, cat)
				continue
			}
		}
		roots = append(roots, cat)
	}

	if roots == nil {
		roots = []*Category{}
	}
	if s.cache != nil {
		_ = s.cache.SetJSON(ctx, s.categoriesCacheKey(merchantID), roots, dictCacheTTL)
	}
	return roots, nil
}

// UpdateCategoryRequest is the request body for updating a category.
type UpdateCategoryRequest struct {
	Name      string `json:"name"`
	SortOrder *int   `json:"sort_order"`
}

// UpdateCategory updates a category's name and/or sort order.
// Merchants can only update their own custom categories (is_platform=false, merchant_id=own).
func (s *Service) UpdateCategory(ctx context.Context, id int64, req UpdateCategoryRequest, merchantID int64) (*Category, error) {
	if strings.TrimSpace(req.Name) == "" && req.SortOrder == nil {
		return nil, apperrors.NewValidationError("at least one field to update is required")
	}

	var cat Category
	var parentID, catMerchID sql.NullInt64
	var createdAt, updatedAt time.Time

	// Check ownership.
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, level, sort_order, status, is_platform, merchant_id, created_at, updated_at
		 FROM system_categories WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&cat.ID, &cat.Name, &parentID, &cat.Level, &cat.SortOrder,
		&cat.Status, &cat.IsPlatform, &catMerchID, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFoundError("category not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to look up category", err)
	}

	if cat.IsPlatform {
		// Merchants cannot modify platform categories.
		if merchantID > 0 {
			return nil, apperrors.NewForbiddenError("cannot modify platform-level category")
		}
	} else {
		// Merchants can only modify their own custom categories.
		if merchantID > 0 && (!catMerchID.Valid || catMerchID.Int64 != merchantID) {
			return nil, apperrors.NewForbiddenError("can only modify your own custom categories")
		}
	}

	name := cat.Name
	sortOrder := cat.SortOrder
	if strings.TrimSpace(req.Name) != "" {
		name = req.Name
	}
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	err = s.db.QueryRowContext(ctx,
		`UPDATE system_categories SET name = $1, sort_order = $2, updated_at = NOW()
		 WHERE id = $3 AND deleted_at IS NULL
		 RETURNING id, name, parent_id, level, sort_order, status, is_platform, merchant_id, created_at, updated_at`,
		name, sortOrder, id,
	).Scan(&cat.ID, &cat.Name, &parentID, &cat.Level, &cat.SortOrder,
		&cat.Status, &cat.IsPlatform, &catMerchID, &createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update category", err)
	}

	if parentID.Valid {
		cat.ParentID = &parentID.Int64
	}
	if catMerchID.Valid {
		cat.MerchantID = &catMerchID.Int64
	}
	cat.CreatedAt = createdAt.Format(time.RFC3339)
	cat.UpdatedAt = updatedAt.Format(time.RFC3339)
	cat.Children = []*Category{}

	s.invalidateDictCache(ctx)
	return &cat, nil
}

// DeleteCategory soft-deletes a category (set deleted_at).
func (s *Service) DeleteCategory(ctx context.Context, id int64, merchantID int64) error {
	var isPlatform bool
	var catMerchID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT is_platform, merchant_id FROM system_categories WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&isPlatform, &catMerchID)
	if errors.Is(err, sql.ErrNoRows) {
		return apperrors.NewNotFoundError("category not found")
	}
	if err != nil {
		return apperrors.NewInternalError("failed to look up category", err)
	}

	if isPlatform {
		if merchantID > 0 {
			return apperrors.NewForbiddenError("cannot delete platform-level category")
		}
	} else {
		if merchantID > 0 && (!catMerchID.Valid || catMerchID.Int64 != merchantID) {
			return apperrors.NewForbiddenError("can only delete your own custom categories")
		}
	}

	// Check for children.
	var childCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM system_categories WHERE parent_id = $1 AND deleted_at IS NULL`, id,
	).Scan(&childCount)
	if err != nil {
		return apperrors.NewInternalError("failed to check child categories", err)
	}
	if childCount > 0 {
		return apperrors.NewValidationError("cannot delete category with sub-categories; remove children first")
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE system_categories SET deleted_at = NOW() WHERE id = $1`, id,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete category", err)
	}
	s.invalidateDictCache(ctx)
	return nil
}

// ToggleCategoryResponse is the response for toggling a category.
type ToggleCategoryResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// ToggleCategory toggles a category between enabled and disabled.
// Only platform admins can toggle platform categories. Merchants can toggle their own.
func (s *Service) ToggleCategory(ctx context.Context, id int64, merchantID int64) (*ToggleCategoryResponse, error) {
	var status string
	var isPlatform bool
	var catMerchID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT status, is_platform, merchant_id FROM system_categories WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&status, &isPlatform, &catMerchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFoundError("category not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to look up category", err)
	}

	if isPlatform {
		if merchantID > 0 {
			return nil, apperrors.NewForbiddenError("cannot modify platform-level category")
		}
	} else {
		if merchantID > 0 && (!catMerchID.Valid || catMerchID.Int64 != merchantID) {
			return nil, apperrors.NewForbiddenError("can only toggle your own custom categories")
		}
	}

	newStatus := "enabled"
	message := "category enabled"
	if status == "enabled" {
		newStatus = "disabled"
		message = "category disabled"
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE system_categories SET status = $1, updated_at = NOW() WHERE id = $2`,
		newStatus, id,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle category", err)
	}

	s.invalidateDictCache(ctx)
	return &ToggleCategoryResponse{Message: message, Status: newStatus}, nil
}

// --- Breed types ---

// Breed represents a pet breed.
type Breed struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PetType   string `json:"pet_type"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateBreedRequest is the request body for creating a breed.
type CreateBreedRequest struct {
	Name      string `json:"name"`
	PetType   string `json:"pet_type"`
	SortOrder int    `json:"sort_order"`
}

// CreateBreed creates a new pet breed.
func (s *Service) CreateBreed(ctx context.Context, req CreateBreedRequest) (*Breed, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("breed name is required")
	}
	petType := strings.TrimSpace(req.PetType)
	if petType == "" {
		petType = "dog"
	}

	var breed Breed
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO system_breeds (name, pet_type, sort_order, status)
		 VALUES ($1, $2, $3, 'enabled')
		 RETURNING id, name, pet_type, sort_order, status, created_at, updated_at`,
		req.Name, petType, req.SortOrder,
	).Scan(&breed.ID, &breed.Name, &breed.PetType, &breed.SortOrder,
		&breed.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create breed", err)
	}
	breed.CreatedAt = createdAt.Format(time.RFC3339)
	breed.UpdatedAt = updatedAt.Format(time.RFC3339)
	s.invalidateDictCache(ctx)
	return &breed, nil
}

// BreedListResponse is the response for listing breeds.
type BreedListResponse struct {
	Breeds []Breed `json:"breeds"`
	Total  int     `json:"total"`
}

// ListBreeds returns pet breeds, optionally filtered by pet_type.
// When merchantID > 0, only enabled breeds are returned (merchant view).
func (s *Service) ListBreeds(ctx context.Context, petType string, merchantID int64) (*BreedListResponse, error) {
	if s.cache != nil {
		var cached BreedListResponse
		if s.cache.GetJSON(ctx, s.breedsCacheKey(petType, merchantID), &cached) {
			return &cached, nil
		}
	}

	var rows *sql.Rows
	var err error

	if merchantID > 0 {
		// Merchant view: only enabled breeds.
		if petType != "" {
			rows, err = s.db.QueryContext(ctx,
				`SELECT id, name, pet_type, sort_order, status, created_at, updated_at
				 FROM system_breeds
				 WHERE deleted_at IS NULL AND status = 'enabled' AND pet_type = $1
				 ORDER BY sort_order, id`,
				petType,
			)
		} else {
			rows, err = s.db.QueryContext(ctx,
				`SELECT id, name, pet_type, sort_order, status, created_at, updated_at
				 FROM system_breeds
				 WHERE deleted_at IS NULL AND status = 'enabled'
				 ORDER BY sort_order, id`,
			)
		}
	} else {
		// Platform admin view: all non-deleted.
		if petType != "" {
			rows, err = s.db.QueryContext(ctx,
				`SELECT id, name, pet_type, sort_order, status, created_at, updated_at
				 FROM system_breeds
				 WHERE deleted_at IS NULL AND pet_type = $1
				 ORDER BY sort_order, id`,
				petType,
			)
		} else {
			rows, err = s.db.QueryContext(ctx,
				`SELECT id, name, pet_type, sort_order, status, created_at, updated_at
				 FROM system_breeds
				 WHERE deleted_at IS NULL
				 ORDER BY sort_order, id`,
			)
		}
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list breeds", err)
	}
	defer rows.Close()

	var breeds []Breed
	for rows.Next() {
		var b Breed
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&b.ID, &b.Name, &b.PetType, &b.SortOrder,
			&b.Status, &createdAt, &updatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan breed", err)
		}
		b.CreatedAt = createdAt.Format(time.RFC3339)
		b.UpdatedAt = updatedAt.Format(time.RFC3339)
		breeds = append(breeds, b)
	}

	if breeds == nil {
		breeds = []Breed{}
	}

	resp := &BreedListResponse{Breeds: breeds, Total: len(breeds)}
	if s.cache != nil {
		_ = s.cache.SetJSON(ctx, s.breedsCacheKey(petType, merchantID), resp, dictCacheTTL)
	}
	return resp, nil
}

// UpdateBreedRequest is the request body for updating a breed.
type UpdateBreedRequest struct {
	Name      string `json:"name"`
	PetType   string `json:"pet_type"`
	SortOrder *int   `json:"sort_order"`
}

// UpdateBreed updates a breed's fields.
func (s *Service) UpdateBreed(ctx context.Context, id int64, req UpdateBreedRequest) (*Breed, error) {
	var breed Breed
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, pet_type, sort_order, status, created_at, updated_at
		 FROM system_breeds WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&breed.ID, &breed.Name, &breed.PetType, &breed.SortOrder,
		&breed.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFoundError("breed not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to look up breed", err)
	}

	name := breed.Name
	petType := breed.PetType
	sortOrder := breed.SortOrder

	if strings.TrimSpace(req.Name) != "" {
		name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.PetType) != "" {
		petType = strings.TrimSpace(req.PetType)
	}
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	err = s.db.QueryRowContext(ctx,
		`UPDATE system_breeds SET name = $1, pet_type = $2, sort_order = $3, updated_at = NOW()
		 WHERE id = $4 AND deleted_at IS NULL
		 RETURNING id, name, pet_type, sort_order, status, created_at, updated_at`,
		name, petType, sortOrder, id,
	).Scan(&breed.ID, &breed.Name, &breed.PetType, &breed.SortOrder,
		&breed.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update breed", err)
	}
	breed.CreatedAt = createdAt.Format(time.RFC3339)
	breed.UpdatedAt = updatedAt.Format(time.RFC3339)
	s.invalidateDictCache(ctx)
	return &breed, nil
}

// DeleteBreed soft-deletes a breed.
func (s *Service) DeleteBreed(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE system_breeds SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id,
	)
	if err != nil {
		return apperrors.NewInternalError("failed to delete breed", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperrors.NewNotFoundError("breed not found")
	}
	s.invalidateDictCache(ctx)
	return nil
}

// ToggleBreedResponse is the response for toggling a breed.
type ToggleBreedResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// ToggleBreed toggles a breed between enabled and disabled.
func (s *Service) ToggleBreed(ctx context.Context, id int64) (*ToggleBreedResponse, error) {
	var status string
	err := s.db.QueryRowContext(ctx,
		`SELECT status FROM system_breeds WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperrors.NewNotFoundError("breed not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to look up breed", err)
	}

	newStatus := "enabled"
	message := "breed enabled"
	if status == "enabled" {
		newStatus = "disabled"
		message = "breed disabled"
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE system_breeds SET status = $1, updated_at = NOW() WHERE id = $2`,
		newStatus, id,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle breed", err)
	}

	s.invalidateDictCache(ctx)
	return &ToggleBreedResponse{Message: message, Status: newStatus}, nil
}

// DetailOnly is used for operation log details.
type DetailOnly struct {
	Detail json.RawMessage `json:"detail"`
}
