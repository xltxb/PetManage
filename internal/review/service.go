package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/internal/middleware"
	apperrors "github.com/xltxb/PetManage/pkg/apperrors"
)

// Review combines service evaluations and product reviews into a unified view.
type Review struct {
	ID              int64           `json:"id"`
	MerchantID      int64           `json:"merchant_id"`
	MemberID        int64           `json:"member_id"`
	MemberName      string          `json:"member_name,omitempty"`
	EmployeeID      *int64          `json:"employee_id"`
	EmployeeName    string          `json:"employee_name,omitempty"`
	ProductID       *int64          `json:"product_id"`
	ProductName     string          `json:"product_name,omitempty"`
	ServiceItemID   *int64          `json:"service_item_id"`
	ServiceItemName string          `json:"service_item_name,omitempty"`
	ReviewType      string          `json:"review_type"` // "service" or "product"
	Rating          int             `json:"rating"`
	Content         string          `json:"content"`
	Images          json.RawMessage `json:"images"`
	Reply           string          `json:"reply"`
	RepliedAt       *time.Time      `json:"replied_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ListParams holds filters for listing reviews.
type ListParams struct {
	RatingMin int    // minimum rating filter (e.g., 4 for good reviews)
	RatingMax int    // maximum rating filter (e.g., 2 for bad reviews)
	Replied   *bool  // filter by reply status
	MemberID  int64  // filter by member
	EmployeeID int64 // filter by employee
	DateFrom  string // YYYY-MM-DD
	DateTo    string // YYYY-MM-DD
	Page      int
	PageSize  int
}

// ListResponse is a paginated review list.
type ListResponse struct {
	Reviews  []Review `json:"reviews"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// ReplyRequest is the input for replying to a review.
type ReplyRequest struct {
	Reply string `json:"reply"`
}

// ReviewStats holds review statistics.
type ReviewStats struct {
	TotalReviews   int     `json:"total_reviews"`
	GoodCount      int     `json:"good_count"`      // rating >= 4
	BadCount       int     `json:"bad_count"`       // rating <= 2
	NeutralCount   int     `json:"neutral_count"`   // rating == 3
	GoodRate       float64 `json:"good_rate"`        // good_count / total_reviews * 100
	AverageRating  float64 `json:"average_rating"`
	RepliedCount   int     `json:"replied_count"`
	UnrepliedCount int     `json:"unreplied_count"`
	Period         string  `json:"period"`
}

// EmployeeReviewStats holds per-employee review statistics.
type EmployeeReviewStats struct {
	EmployeeID   int64   `json:"employee_id"`
	EmployeeName string  `json:"employee_name"`
	TotalReviews int     `json:"total_reviews"`
	AverageRating float64 `json:"average_rating"`
	GoodCount    int     `json:"good_count"`
	BadCount     int     `json:"bad_count"`
}

// Service handles review management.
type Service struct {
	db *sql.DB
}

// NewService creates a new review Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ListReviews returns a unified list of service evaluations and product reviews.
func (s *Service) ListReviews(ctx context.Context, merchantID int64, params ListParams) (*ListResponse, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	// Build the unified query using UNION ALL.
	serviceCTE := `SELECT se.id, se.merchant_id, se.member_id,
		COALESCE(m.name, '') AS member_name,
		se.employee_id, COALESCE(e.name, '') AS employee_name,
		NULL::bigint AS product_id, '' AS product_name,
		si.id AS service_item_id, COALESCE(si.name, '') AS service_item_name,
		'service' AS review_type,
		se.rating, se.content,
		CASE WHEN se.images = '' OR se.images IS NULL THEN '[]'::jsonb ELSE se.images::jsonb END AS images,
		se.reply, se.replied_at, se.created_at, se.updated_at
		FROM service_evaluations se
		LEFT JOIN members m ON m.id = se.member_id
		LEFT JOIN employees e ON e.id = se.employee_id
		LEFT JOIN pet_service_records psr ON psr.id = se.service_record_id
		LEFT JOIN service_items si ON si.id = COALESCE(psr.service_item_id, 0)
		WHERE se.merchant_id = $1`

	productCTE := `SELECT pr.id, pr.merchant_id, pr.member_id,
		COALESCE(m.name, '') AS member_name,
		NULL::bigint AS employee_id, '' AS employee_name,
		pr.product_id, COALESCE(p.name, '') AS product_name,
		NULL::bigint AS service_item_id, '' AS service_item_name,
		'product' AS review_type,
		pr.rating, pr.content,
		CASE WHEN pr.images = '' OR pr.images IS NULL THEN '[]'::jsonb ELSE pr.images::jsonb END AS images,
		pr.reply, pr.replied_at, pr.created_at, pr.updated_at
		FROM product_reviews pr
		LEFT JOIN members m ON m.id = pr.member_id
		LEFT JOIN products p ON p.id = pr.product_id
		WHERE pr.merchant_id = $1`

	// Apply common filters to both sides.
	var args []interface{}
	args = append(args, merchantID)
	argIdx := 2

	var conditions []string

	if params.RatingMin > 0 {
		conditions = append(conditions, fmt.Sprintf("rating >= $%d", argIdx))
		args = append(args, params.RatingMin)
		argIdx++
	}
	if params.RatingMax > 0 {
		conditions = append(conditions, fmt.Sprintf("rating <= $%d", argIdx))
		args = append(args, params.RatingMax)
		argIdx++
	}
	if params.Replied != nil {
		if *params.Replied {
			conditions = append(conditions, "reply != ''")
		} else {
			conditions = append(conditions, "(reply = '' OR reply IS NULL)")
		}
	}
	if params.MemberID > 0 {
		conditions = append(conditions, fmt.Sprintf("member_id = $%d", argIdx))
		args = append(args, params.MemberID)
		argIdx++
	}
	if params.EmployeeID > 0 {
		conditions = append(conditions, fmt.Sprintf("employee_id = $%d", argIdx))
		args = append(args, params.EmployeeID)
		argIdx++
	}
	if params.DateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d::date", argIdx))
		args = append(args, params.DateFrom)
		argIdx++
	}
	if params.DateTo != "" {
		conditions = append(conditions, fmt.Sprintf("created_at < ($%d::date + interval '1 day')", argIdx))
		args = append(args, params.DateTo)
		argIdx++
	}

	filterClause := ""
	if len(conditions) > 0 {
		for _, c := range conditions {
			filterClause += " AND " + c
		}
	}

	// Additional filter: only include product reviews that are not deleted
	serviceDeletedFilter := ""
	productDeletedFilter := " AND pr.deleted_at IS NULL"

	unionQuery := fmt.Sprintf(
		`SELECT * FROM (
			(%s%s%s)
			UNION ALL
			(%s%s%s)
		) AS reviews`, serviceCTE, filterClause, serviceDeletedFilter, productCTE, filterClause, productDeletedFilter)

	// Count query.
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM (%s) AS cnt`, unionQuery)
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count reviews", err)
	}

	// Paginated query.
	offset := (params.Page - 1) * params.PageSize
	dataQuery := fmt.Sprintf(`%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		unionQuery, argIdx, argIdx+1)
	dataArgs := append(args, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query reviews", err)
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var r Review
		var imagesRaw []byte
		if err := rows.Scan(
			&r.ID, &r.MerchantID, &r.MemberID, &r.MemberName,
			&r.EmployeeID, &r.EmployeeName,
			&r.ProductID, &r.ProductName,
			&r.ServiceItemID, &r.ServiceItemName,
			&r.ReviewType, &r.Rating, &r.Content,
			&imagesRaw, &r.Reply, &r.RepliedAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, apperrors.NewInternalError("failed to scan review", err)
		}
		r.Images = json.RawMessage(imagesRaw)
		reviews = append(reviews, r)
	}
	if reviews == nil {
		reviews = []Review{}
	}

	return &ListResponse{
		Reviews:  reviews,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetReview returns a single review by type and id.
func (s *Service) GetReview(ctx context.Context, merchantID int64, reviewType string, reviewID int64) (*Review, error) {
	var r Review
	var imagesRaw []byte

	if reviewType == "service" {
		err := s.db.QueryRowContext(ctx,
			`SELECT se.id, se.merchant_id, se.member_id,
				COALESCE(m.name, '') AS member_name,
				se.employee_id, COALESCE(e.name, '') AS employee_name,
				NULL::bigint, '',
				psr.service_item_id, COALESCE(si.name, '') AS service_item_name,
				'service', se.rating, se.content,
				CASE WHEN se.images = '' OR se.images IS NULL THEN '[]'::jsonb ELSE se.images::jsonb END,
				se.reply, se.replied_at, se.created_at, se.updated_at
			FROM service_evaluations se
			LEFT JOIN members m ON m.id = se.member_id
			LEFT JOIN employees e ON e.id = se.employee_id
			LEFT JOIN pet_service_records psr ON psr.id = se.service_record_id
			LEFT JOIN service_items si ON si.id = COALESCE(psr.service_item_id, 0)
			WHERE se.id = $1 AND se.merchant_id = $2`, reviewID, merchantID,
		).Scan(
			&r.ID, &r.MerchantID, &r.MemberID, &r.MemberName,
			&r.EmployeeID, &r.EmployeeName,
			&r.ProductID, &r.ProductName,
			&r.ServiceItemID, &r.ServiceItemName,
			&r.ReviewType, &r.Rating, &r.Content,
			&imagesRaw, &r.Reply, &r.RepliedAt, &r.CreatedAt, &r.UpdatedAt,
		)
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFoundError("review not found")
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to get review", err)
		}
	} else {
		err := s.db.QueryRowContext(ctx,
			`SELECT pr.id, pr.merchant_id, pr.member_id,
				COALESCE(m.name, '') AS member_name,
				NULL::bigint, '',
				pr.product_id, COALESCE(p.name, '') AS product_name,
				NULL::bigint, '',
				'product', pr.rating, pr.content,
				CASE WHEN pr.images = '' OR pr.images IS NULL THEN '[]'::jsonb ELSE pr.images::jsonb END,
				pr.reply, pr.replied_at, pr.created_at, pr.updated_at
			FROM product_reviews pr
			LEFT JOIN members m ON m.id = pr.member_id
			LEFT JOIN products p ON p.id = pr.product_id
			WHERE pr.id = $1 AND pr.merchant_id = $2 AND pr.deleted_at IS NULL`, reviewID, merchantID,
		).Scan(
			&r.ID, &r.MerchantID, &r.MemberID, &r.MemberName,
			&r.EmployeeID, &r.EmployeeName,
			&r.ProductID, &r.ProductName,
			&r.ServiceItemID, &r.ServiceItemName,
			&r.ReviewType, &r.Rating, &r.Content,
			&imagesRaw, &r.Reply, &r.RepliedAt, &r.CreatedAt, &r.UpdatedAt,
		)
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFoundError("review not found")
		}
		if err != nil {
			return nil, apperrors.NewInternalError("failed to get review", err)
		}
	}

	r.Images = json.RawMessage(imagesRaw)
	return &r, nil
}

// SubmitReply adds a merchant reply to a review.
func (s *Service) SubmitReply(ctx context.Context, merchantID int64, reviewType string, reviewID int64, req ReplyRequest) (*Review, error) {
	if req.Reply == "" {
		return nil, apperrors.NewValidationError("reply is required")
	}

	now := time.Now()

	if reviewType == "service" {
		result, err := s.db.ExecContext(ctx,
			`UPDATE service_evaluations SET reply = $1, replied_at = $2, updated_at = NOW()
			 WHERE id = $3 AND merchant_id = $4 AND (reply = '' OR reply IS NULL)`,
			req.Reply, now, reviewID, merchantID)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to submit reply", err)
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			// Check if the review exists but already has a reply.
			var existingReply string
			err := s.db.QueryRowContext(ctx,
				`SELECT reply FROM service_evaluations WHERE id = $1 AND merchant_id = $2`,
				reviewID, merchantID,
			).Scan(&existingReply)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("review not found")
			}
			if existingReply != "" {
				return nil, apperrors.NewValidationError("review already has a reply")
			}
			return nil, apperrors.NewInternalError("failed to submit reply", nil)
		}
	} else {
		result, err := s.db.ExecContext(ctx,
			`UPDATE product_reviews SET reply = $1, replied_at = $2, updated_at = NOW()
			 WHERE id = $3 AND merchant_id = $4 AND (reply = '' OR reply IS NULL) AND deleted_at IS NULL`,
			req.Reply, now, reviewID, merchantID)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to submit reply", err)
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			var existingReply string
			err := s.db.QueryRowContext(ctx,
				`SELECT reply FROM product_reviews WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
				reviewID, merchantID,
			).Scan(&existingReply)
			if err == sql.ErrNoRows {
				return nil, apperrors.NewNotFoundError("review not found")
			}
			if existingReply != "" {
				return nil, apperrors.NewValidationError("review already has a reply")
			}
			return nil, apperrors.NewInternalError("failed to submit reply", nil)
		}
	}

	s.recordLog(ctx, 0, "reply_review", reviewType+"_review", reviewID, map[string]interface{}{
		"reply": req.Reply,
	})

	return s.GetReview(ctx, merchantID, reviewType, reviewID)
}

// GetStats returns review statistics for a merchant.
func (s *Service) GetStats(ctx context.Context, merchantID int64, period string) (*ReviewStats, error) {
	var since time.Time
	now := time.Now()
	switch period {
	case "today":
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		weekday := now.Weekday()
		if weekday == 0 {
			weekday = 7
		}
		since = time.Date(now.Year(), now.Month(), now.Day()-int(weekday)+1, 0, 0, 0, 0, now.Location())
	case "month":
		since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "year":
		since = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default:
		period = "all"
	}

	stats := &ReviewStats{Period: period}

	// Unified stats from both service evaluations and product reviews.
	var serviceWhere string
	var productWhere string
	var args []interface{}
	args = append(args, merchantID)
	argIdx := 2

	if period != "all" {
		serviceWhere = fmt.Sprintf(" AND created_at >= $%d", argIdx)
		productWhere = fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, since)
		argIdx++
	}
	productWhere += " AND deleted_at IS NULL"

	// Total reviews.
	query := fmt.Sprintf(
		`SELECT COALESCE((SELECT COUNT(*) FROM service_evaluations WHERE merchant_id = $1%s), 0) +
		        COALESCE((SELECT COUNT(*) FROM product_reviews WHERE merchant_id = $1%s), 0)`,
		serviceWhere, productWhere)
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&stats.TotalReviews)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count reviews", err)
	}

	if stats.TotalReviews == 0 {
		return stats, nil
	}

	// Good reviews (rating >= 4).
	query = fmt.Sprintf(
		`SELECT COALESCE((SELECT COUNT(*) FROM service_evaluations WHERE merchant_id = $1 AND rating >= 4%s), 0) +
		        COALESCE((SELECT COUNT(*) FROM product_reviews WHERE merchant_id = $1 AND rating >= 4%s), 0)`,
		serviceWhere, productWhere)
	s.db.QueryRowContext(ctx, query, args...).Scan(&stats.GoodCount)

	// Bad reviews (rating <= 2).
	query = fmt.Sprintf(
		`SELECT COALESCE((SELECT COUNT(*) FROM service_evaluations WHERE merchant_id = $1 AND rating <= 2%s), 0) +
		        COALESCE((SELECT COUNT(*) FROM product_reviews WHERE merchant_id = $1 AND rating <= 2%s), 0)`,
		serviceWhere, productWhere)
	s.db.QueryRowContext(ctx, query, args...).Scan(&stats.BadCount)

	// Neutral (rating == 3).
	query = fmt.Sprintf(
		`SELECT COALESCE((SELECT COUNT(*) FROM service_evaluations WHERE merchant_id = $1 AND rating = 3%s), 0) +
		        COALESCE((SELECT COUNT(*) FROM product_reviews WHERE merchant_id = $1 AND rating = 3%s), 0)`,
		serviceWhere, productWhere)
	s.db.QueryRowContext(ctx, query, args...).Scan(&stats.NeutralCount)

	// Good rate.
	stats.GoodRate = float64(stats.GoodCount) / float64(stats.TotalReviews) * 100

	// Average rating.
	query = fmt.Sprintf(
		`SELECT COALESCE(
			(SELECT AVG(rating)::numeric(3,1) FROM (
				SELECT rating FROM service_evaluations WHERE merchant_id = $1%s
				UNION ALL
				SELECT rating FROM product_reviews WHERE merchant_id = $1%s
			) AS all_ratings), 0)`,
		serviceWhere, productWhere)
	var avgRating sql.NullFloat64
	s.db.QueryRowContext(ctx, query, args...).Scan(&avgRating)
	if avgRating.Valid {
		stats.AverageRating = avgRating.Float64
	}

	// Replied count.
	query = fmt.Sprintf(
		`SELECT COALESCE((SELECT COUNT(*) FROM service_evaluations WHERE merchant_id = $1 AND reply != '' %s), 0) +
		        COALESCE((SELECT COUNT(*) FROM product_reviews WHERE merchant_id = $1 AND reply != '' %s), 0)`,
		serviceWhere, productWhere)
	s.db.QueryRowContext(ctx, query, args...).Scan(&stats.RepliedCount)

	stats.UnrepliedCount = stats.TotalReviews - stats.RepliedCount

	return stats, nil
}

// GetEmployeeStats returns per-employee review statistics.
func (s *Service) GetEmployeeStats(ctx context.Context, merchantID int64) ([]EmployeeReviewStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, COALESCE(e.name, ''),
			COUNT(se.id) AS total_reviews,
			COALESCE(AVG(se.rating)::numeric(3,1), 0) AS avg_rating,
			COUNT(CASE WHEN se.rating >= 4 THEN 1 END) AS good_count,
			COUNT(CASE WHEN se.rating <= 2 THEN 1 END) AS bad_count
		FROM employees e
		LEFT JOIN service_evaluations se ON se.employee_id = e.id AND se.merchant_id = $1
		WHERE e.merchant_id = $1 AND e.deleted_at IS NULL
		GROUP BY e.id, e.name
		ORDER BY avg_rating DESC NULLS LAST, total_reviews DESC`,
		merchantID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to query employee stats", err)
	}
	defer rows.Close()

	var stats []EmployeeReviewStats
	for rows.Next() {
		var es EmployeeReviewStats
		if err := rows.Scan(&es.EmployeeID, &es.EmployeeName, &es.TotalReviews, &es.AverageRating, &es.GoodCount, &es.BadCount); err != nil {
			return nil, apperrors.NewInternalError("failed to scan employee stats", err)
		}
		stats = append(stats, es)
	}
	if stats == nil {
		stats = []EmployeeReviewStats{}
	}

	return stats, nil
}

func (s *Service) recordLog(ctx context.Context, userID int64, action, targetType string, targetID int64, detail map[string]interface{}) {
	detailJSON, _ := json.Marshal(detail)
	ip := middleware.ClientIPFromContext(ctx)
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail, ip_address)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
		userID, action, targetType, targetID, string(detailJSON), ip,
	)
}
