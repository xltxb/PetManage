package announcement

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/xltxb/PetManage/internal/middleware"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Announcement represents a platform announcement.
type Announcement struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Scope     string     `json:"scope"`
	IsPinned  bool       `json:"is_pinned"`
	PublishAt time.Time  `json:"publish_at"`
	CreatedBy int64      `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	// Stats fields (populated in list/detail queries)
	CreatedByName string  `json:"created_by_name,omitempty"`
	ReadCount     int     `json:"read_count"`
	TargetCount   int     `json:"target_count"`
	IsRead        bool    `json:"is_read,omitempty"`
	MerchantIDs   []int64 `json:"merchant_ids,omitempty"`
}

// AnnouncementDetail includes announcement with read/unread merchant lists.
type AnnouncementDetail struct {
	Announcement
	ReadMerchants   []MerchantInfo `json:"read_merchants"`
	UnreadMerchants []MerchantInfo `json:"unread_merchants"`
}

// MerchantInfo is a compact merchant reference for read/unread lists.
type MerchantInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CreateAnnouncementRequest is the input for creating an announcement.
type CreateAnnouncementRequest struct {
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Scope       string  `json:"scope"`
	MerchantIDs []int64 `json:"merchant_ids,omitempty"`
	IsPinned    bool    `json:"is_pinned"`
	PublishAt   string  `json:"publish_at,omitempty"`
}

// UpdateAnnouncementRequest is the input for updating an announcement.
type UpdateAnnouncementRequest struct {
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Scope       string  `json:"scope"`
	MerchantIDs []int64 `json:"merchant_ids,omitempty"`
	IsPinned    bool    `json:"is_pinned"`
	PublishAt   string  `json:"publish_at,omitempty"`
}

// ListParams holds filter parameters for listing announcements.
type ListParams struct {
	Scope string
	Page  int
	PageSize int
}

// ListResponse wraps the announcement list response.
type ListResponse struct {
	Announcements []Announcement `json:"announcements"`
	Total         int            `json:"total"`
	Page          int            `json:"page"`
	PageSize      int            `json:"page_size"`
}

// MerchantAnnouncement is announcement data for the merchant side.
type MerchantAnnouncement struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	IsPinned    bool      `json:"is_pinned"`
	PublishAt   time.Time `json:"publish_at"`
	IsRead      bool      `json:"is_read"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Service handles announcement operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new announcement Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateAnnouncement creates a new announcement.
func (s *Service) CreateAnnouncement(ctx context.Context, req CreateAnnouncementRequest, createdBy int64) (*Announcement, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Scope = strings.TrimSpace(req.Scope)

	if req.Title == "" {
		return nil, apperrors.NewValidationError("title is required")
	}
	if req.Scope == "" {
		req.Scope = "all"
	}
	if req.Scope != "all" && req.Scope != "merchants" {
		return nil, apperrors.NewValidationError("scope must be 'all' or 'merchants'")
	}
	if req.Scope == "merchants" && len(req.MerchantIDs) == 0 {
		return nil, apperrors.NewValidationError("merchant_ids is required when scope is 'merchants'")
	}

	var publishAt time.Time
	if req.PublishAt != "" {
		var err error
		publishAt, err = time.Parse(time.RFC3339, req.PublishAt)
		if err != nil {
			return nil, apperrors.NewValidationError("invalid publish_at format, expected RFC3339")
		}
	} else {
		publishAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("begin tx failed", err)
	}
	defer tx.Rollback()

	var a Announcement
	err = tx.QueryRowContext(ctx,
		`INSERT INTO platform_announcements (title, content, scope, is_pinned, publish_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, title, content, scope, is_pinned, publish_at, created_by, created_at, updated_at`,
		req.Title, req.Content, req.Scope, req.IsPinned, publishAt, createdBy,
	).Scan(&a.ID, &a.Title, &a.Content, &a.Scope, &a.IsPinned, &a.PublishAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("insert announcement failed", err)
	}

	// Insert merchant associations if scope is 'merchants'.
	if req.Scope == "merchants" {
		for _, mid := range req.MerchantIDs {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO announcement_merchants (announcement_id, merchant_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				a.ID, mid)
			if err != nil {
				return nil, apperrors.NewInternalError("insert announcement_merchants failed", err)
			}
		}
		a.MerchantIDs = req.MerchantIDs
	}

	// Record operation log.
	detail, _ := json.Marshal(map[string]interface{}{
		"title":  req.Title,
		"scope":  req.Scope,
		"pinned": req.IsPinned,
	})
		ip := middleware.ClientIPFromContext(ctx)
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail, ip_address, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		createdBy, "create_announcement", "announcement", a.ID, string(detail), ip, time.Now())

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("commit failed", err)
	}

	return &a, nil
}

// ListAnnouncements lists announcements with read stats (platform side).
func (s *Service) ListAnnouncements(ctx context.Context, params ListParams) (*ListResponse, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, "a.deleted_at IS NULL")
	if params.Scope != "" {
		conditions = append(conditions, "a.scope = $"+itoa(argIdx))
		args = append(args, params.Scope)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			whereClause += " AND " + c
		}
	}

	// Count total.
	var total int
	countQuery := "SELECT COUNT(*) FROM platform_announcements a " + whereClause
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("count announcements failed", err)
	}

	// Query with stats.
	offset := (params.Page - 1) * params.PageSize
	query := `SELECT a.id, a.title, a.content, a.scope, a.is_pinned, a.publish_at, a.created_by, a.created_at, a.updated_at,
		COALESCE(u.username, '') AS created_by_name,
		COALESCE(rs.read_count, 0) AS read_count,
		CASE WHEN a.scope = 'all'
			THEN (SELECT COUNT(*) FROM merchants WHERE deleted_at IS NULL)
			ELSE COALESCE(amc.target_count, 0)
		END AS target_count
	FROM platform_announcements a
	LEFT JOIN platform_users u ON u.id = a.created_by
	LEFT JOIN LATERAL (
		SELECT COUNT(*) AS read_count FROM announcement_reads WHERE announcement_id = a.id
	) rs ON true
	LEFT JOIN LATERAL (
		SELECT COUNT(*) AS target_count FROM announcement_merchants WHERE announcement_id = a.id
	) amc ON true
	` + whereClause + `
	ORDER BY a.is_pinned DESC, a.publish_at DESC
	LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	args = append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, apperrors.NewInternalError("query announcements failed", err)
	}
	defer rows.Close()

	var announcements []Announcement
	for rows.Next() {
		var a Announcement
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &a.Scope, &a.IsPinned, &a.PublishAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
			&a.CreatedByName, &a.ReadCount, &a.TargetCount); err != nil {
			return nil, apperrors.NewInternalError("scan announcement failed", err)
		}
		announcements = append(announcements, a)
	}
	if rows.Err() != nil {
		return nil, apperrors.NewInternalError("iterate announcements failed", rows.Err())
	}

	if announcements == nil {
		announcements = []Announcement{}
	}

	return &ListResponse{
		Announcements: announcements,
		Total:         total,
		Page:          params.Page,
		PageSize:      params.PageSize,
	}, nil
}

// GetAnnouncement returns announcement detail with read/unread merchant lists.
func (s *Service) GetAnnouncement(ctx context.Context, id int64) (*AnnouncementDetail, error) {
	var a Announcement
	err := s.db.QueryRowContext(ctx,
		`SELECT a.id, a.title, a.content, a.scope, a.is_pinned, a.publish_at, a.created_by, a.created_at, a.updated_at,
			COALESCE(u.username, '') AS created_by_name
		 FROM platform_announcements a
		 LEFT JOIN platform_users u ON u.id = a.created_by
		 WHERE a.id = $1 AND a.deleted_at IS NULL`, id,
	).Scan(&a.ID, &a.Title, &a.Content, &a.Scope, &a.IsPinned, &a.PublishAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt, &a.CreatedByName)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("announcement not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("query announcement failed", err)
	}

	detail := &AnnouncementDetail{Announcement: a}

	// Get read merchants.
	readRows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT m.id, m.name FROM announcement_reads ar
		 JOIN platform_users pu ON pu.id = ar.user_id
		 JOIN merchants m ON m.id = pu.merchant_id
		 WHERE ar.announcement_id = $1
		 ORDER BY m.name`, id)
	if err != nil {
		return nil, apperrors.NewInternalError("query read merchants failed", err)
	}
	defer readRows.Close()
	for readRows.Next() {
		var m MerchantInfo
		if err := readRows.Scan(&m.ID, &m.Name); err != nil {
			return nil, apperrors.NewInternalError("scan read merchant failed", err)
		}
		detail.ReadMerchants = append(detail.ReadMerchants, m)
	}
	if detail.ReadMerchants == nil {
		detail.ReadMerchants = []MerchantInfo{}
	}

	// Get unread merchants (targeted by this announcement but haven't read it).
	var unreadQuery string
	var unreadArgs []interface{}
	if a.Scope == "all" {
		unreadQuery = `SELECT m.id, m.name FROM merchants m
		 WHERE m.deleted_at IS NULL AND m.id NOT IN (
			SELECT pu.merchant_id FROM announcement_reads ar
			JOIN platform_users pu ON pu.id = ar.user_id
			WHERE ar.announcement_id = $1 AND pu.merchant_id IS NOT NULL
		 ) ORDER BY m.name`
		unreadArgs = []interface{}{id}
	} else {
		unreadQuery = `SELECT m.id, m.name FROM merchants m
		 JOIN announcement_merchants am ON am.merchant_id = m.id
		 WHERE am.announcement_id = $1 AND m.deleted_at IS NULL AND m.id NOT IN (
			SELECT pu.merchant_id FROM announcement_reads ar
			JOIN platform_users pu ON pu.id = ar.user_id
			WHERE ar.announcement_id = $1 AND pu.merchant_id IS NOT NULL
		 ) ORDER BY m.name`
		unreadArgs = []interface{}{id}
	}

	unreadRows, err := s.db.QueryContext(ctx, unreadQuery, unreadArgs...)
	if err != nil {
		return nil, apperrors.NewInternalError("query unread merchants failed", err)
	}
	defer unreadRows.Close()
	for unreadRows.Next() {
		var m MerchantInfo
		if err := unreadRows.Scan(&m.ID, &m.Name); err != nil {
			return nil, apperrors.NewInternalError("scan unread merchant failed", err)
		}
		detail.UnreadMerchants = append(detail.UnreadMerchants, m)
	}
	if detail.UnreadMerchants == nil {
		detail.UnreadMerchants = []MerchantInfo{}
	}

	// Populate target counts.
	detail.ReadCount = len(detail.ReadMerchants)
	detail.TargetCount = detail.ReadCount + len(detail.UnreadMerchants)

	// Get merchant IDs if scope is 'merchants'.
	if a.Scope == "merchants" {
		mRows, err := s.db.QueryContext(ctx,
			`SELECT merchant_id FROM announcement_merchants WHERE announcement_id = $1 ORDER BY merchant_id`, id)
		if err == nil {
			defer mRows.Close()
			for mRows.Next() {
				var mid int64
				if err := mRows.Scan(&mid); err == nil {
					detail.MerchantIDs = append(detail.MerchantIDs, mid)
				}
			}
		}
	}

	return detail, nil
}

// UpdateAnnouncement updates an existing announcement.
func (s *Service) UpdateAnnouncement(ctx context.Context, id int64, req UpdateAnnouncementRequest) (*Announcement, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Scope = strings.TrimSpace(req.Scope)

	if req.Title == "" {
		return nil, apperrors.NewValidationError("title is required")
	}
	if req.Scope != "all" && req.Scope != "merchants" {
		return nil, apperrors.NewValidationError("scope must be 'all' or 'merchants'")
	}
	if req.Scope == "merchants" && len(req.MerchantIDs) == 0 {
		return nil, apperrors.NewValidationError("merchant_ids is required when scope is 'merchants'")
	}

	var publishAt time.Time
	if req.PublishAt != "" {
		var err error
		publishAt, err = time.Parse(time.RFC3339, req.PublishAt)
		if err != nil {
			return nil, apperrors.NewValidationError("invalid publish_at format, expected RFC3339")
		}
	} else {
		publishAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperrors.NewInternalError("begin tx failed", err)
	}
	defer tx.Rollback()

	// Verify exists and not deleted.
	var existingScope string
	err = tx.QueryRowContext(ctx,
		`SELECT scope FROM platform_announcements WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&existingScope)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("announcement not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("check announcement failed", err)
	}

	var a Announcement
	err = tx.QueryRowContext(ctx,
		`UPDATE platform_announcements
		 SET title = $2, content = $3, scope = $4, is_pinned = $5, publish_at = $6, updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, title, content, scope, is_pinned, publish_at, created_by, created_at, updated_at`,
		id, req.Title, req.Content, req.Scope, req.IsPinned, publishAt,
	).Scan(&a.ID, &a.Title, &a.Content, &a.Scope, &a.IsPinned, &a.PublishAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, apperrors.NewInternalError("update announcement failed", err)
	}

	// Update merchant associations if scope changed or merchants.
	if req.Scope == "merchants" {
		_, _ = tx.ExecContext(ctx, `DELETE FROM announcement_merchants WHERE announcement_id = $1`, id)
		for _, mid := range req.MerchantIDs {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO announcement_merchants (announcement_id, merchant_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, mid)
			if err != nil {
				return nil, apperrors.NewInternalError("update announcement_merchants failed", err)
			}
		}
		a.MerchantIDs = req.MerchantIDs
	} else if existingScope == "merchants" && req.Scope == "all" {
		_, _ = tx.ExecContext(ctx, `DELETE FROM announcement_merchants WHERE announcement_id = $1`, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, apperrors.NewInternalError("commit failed", err)
	}

	return &a, nil
}

// DeleteAnnouncement soft-deletes an announcement.
func (s *Service) DeleteAnnouncement(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE platform_announcements SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`,
		id, time.Now())
	if err != nil {
		return apperrors.NewInternalError("delete announcement failed", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.NewNotFoundError("announcement not found")
	}
	return nil
}

// PinAnnouncement toggles the pinned status of an announcement.
func (s *Service) PinAnnouncement(ctx context.Context, id int64) (*Announcement, error) {
	var a Announcement
	err := s.db.QueryRowContext(ctx,
		`UPDATE platform_announcements
		 SET is_pinned = NOT is_pinned, updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, title, content, scope, is_pinned, publish_at, created_by, created_at, updated_at`,
		id,
	).Scan(&a.ID, &a.Title, &a.Content, &a.Scope, &a.IsPinned, &a.PublishAt, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("announcement not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("toggle pin failed", err)
	}
	return &a, nil
}

// GetMerchantAnnouncements returns announcements for a specific merchant user.
func (s *Service) GetMerchantAnnouncements(ctx context.Context, merchantID int64, userID int64) ([]MerchantAnnouncement, error) {
	query := `SELECT a.id, a.title, a.content, a.is_pinned, a.publish_at, a.created_at,
		ar.read_at IS NOT NULL AS is_read, ar.read_at
	FROM platform_announcements a
	LEFT JOIN announcement_reads ar ON ar.announcement_id = a.id AND ar.user_id = $2
	WHERE a.deleted_at IS NULL AND a.publish_at <= NOW()
	AND (
		a.scope = 'all'
		OR (a.scope = 'merchants' AND EXISTS (
			SELECT 1 FROM announcement_merchants am WHERE am.announcement_id = a.id AND am.merchant_id = $1
		))
	)
	ORDER BY a.is_pinned DESC, a.publish_at DESC`

	rows, err := s.db.QueryContext(ctx, query, merchantID, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("query merchant announcements failed", err)
	}
	defer rows.Close()

	var result []MerchantAnnouncement
	for rows.Next() {
		var ma MerchantAnnouncement
		var readAt sql.NullTime
		if err := rows.Scan(&ma.ID, &ma.Title, &ma.Content, &ma.IsPinned, &ma.PublishAt, &ma.CreatedAt, &ma.IsRead, &readAt); err != nil {
			return nil, apperrors.NewInternalError("scan merchant announcement failed", err)
		}
		if readAt.Valid {
			ma.ReadAt = &readAt.Time
		}
		result = append(result, ma)
	}
	if rows.Err() != nil {
		return nil, apperrors.NewInternalError("iterate merchant announcements failed", rows.Err())
	}
	if result == nil {
		result = []MerchantAnnouncement{}
	}
	return result, nil
}

// GetUnreadCount returns the number of unread announcements for a merchant user.
func (s *Service) GetUnreadCount(ctx context.Context, merchantID int64, userID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM platform_announcements a
		 WHERE a.deleted_at IS NULL AND a.publish_at <= NOW()
		 AND (
			a.scope = 'all'
			OR (a.scope = 'merchants' AND EXISTS (
				SELECT 1 FROM announcement_merchants am WHERE am.announcement_id = a.id AND am.merchant_id = $1
			))
		 )
		 AND NOT EXISTS (
			SELECT 1 FROM announcement_reads ar WHERE ar.announcement_id = a.id AND ar.user_id = $2
		 )`,
		merchantID, userID,
	).Scan(&count)
	if err != nil {
		return 0, apperrors.NewInternalError("count unread failed", err)
	}
	return count, nil
}

// MarkAsRead marks an announcement as read by the given user.
func (s *Service) MarkAsRead(ctx context.Context, announcementID int64, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO announcement_reads (announcement_id, user_id, read_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (announcement_id, user_id) DO NOTHING`,
		announcementID, userID)
	if err != nil {
		return apperrors.NewInternalError("mark as read failed", err)
	}
	return nil
}

// itoa converts int to string for building SQL placeholders.
func itoa(i int) string {
	return strconv.Itoa(i)
}
