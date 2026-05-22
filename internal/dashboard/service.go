package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service provides platform dashboard metrics.
type Service struct {
	db *sql.DB
}

// NewService creates a new dashboard Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// MetricItem represents a single metric with its value.
type MetricItem struct {
	Value int64  `json:"value"`
	Label string `json:"label"`
}

// OverviewResponse is the dashboard overview response.
type OverviewResponse struct {
	TotalMerchants     int64       `json:"total_merchants"`
	ActiveMerchants    int64       `json:"active_merchants"`
	NewMerchantsPeriod int64       `json:"new_merchants_period"`
	TotalOrders        int64       `json:"total_orders"`
	TotalTransaction   int64       `json:"total_transaction"`
	NewMembers         int64       `json:"new_members"`
	ServiceCompletions int64       `json:"service_completions"`
	Period             string      `json:"period"`
	Metrics            []MetricItem `json:"metrics"`
}

// GetOverview returns platform overview metrics filtered by time period.
func (s *Service) GetOverview(ctx context.Context, period string) (*OverviewResponse, error) {
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

	resp := &OverviewResponse{Period: period}

	// Total merchants (non-deleted).
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM merchants WHERE deleted_at IS NULL`).Scan(&resp.TotalMerchants)

	// Active merchants (approved status).
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM merchants WHERE deleted_at IS NULL AND status = 'approved'`).Scan(&resp.ActiveMerchants)

	// New merchants in period.
	if period != "all" {
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM merchants WHERE deleted_at IS NULL AND created_at >= $1`, since).Scan(&resp.NewMerchantsPeriod)
	}

	// New platform users with merchant association (merchant admin accounts) in period.
	if period != "all" {
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM platform_users WHERE deleted_at IS NULL AND merchant_id IS NOT NULL AND created_at >= $1`, since).Scan(&resp.NewMembers)
	} else {
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM platform_users WHERE deleted_at IS NULL AND merchant_id IS NOT NULL`).Scan(&resp.NewMembers)
	}

	// Orders and transactions: not yet available, return 0.
	// Service completions: not yet available, return 0.

	resp.Metrics = []MetricItem{
		{Value: resp.TotalMerchants, Label: "商户总数"},
		{Value: resp.ActiveMerchants, Label: "活跃商户"},
		{Value: resp.NewMerchantsPeriod, Label: "新增商户"},
		{Value: resp.TotalTransaction, Label: "累计交易额(元)"},
		{Value: resp.TotalOrders, Label: "订单总量"},
		{Value: resp.NewMembers, Label: "新增会员"},
		{Value: resp.ServiceCompletions, Label: "服务完成量"},
	}

	return resp, nil
}

// ProductSalesRank represents a product sales ranking item.
type ProductSalesRank struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int64  `json:"quantity"`
	Revenue     int64  `json:"revenue"`
	Rank        int    `json:"rank"`
}

// ServicePopularity represents a service popularity ranking item.
type ServicePopularity struct {
	ServiceID   int64  `json:"service_id"`
	ServiceName string `json:"service_name"`
	OrderCount  int64  `json:"order_count"`
	Revenue     int64  `json:"revenue"`
	Rank        int    `json:"rank"`
}

// MerchantRevenueRank represents a merchant revenue ranking item.
type MerchantRevenueRank struct {
	MerchantID   int64  `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
	TotalRevenue int64  `json:"total_revenue"`
	Rank         int    `json:"rank"`
}

// MerchantAnalysisResponse holds comprehensive analysis for a single merchant.
type MerchantAnalysisResponse struct {
	MerchantID        int64               `json:"merchant_id"`
	MerchantName      string              `json:"merchant_name"`
	Period            string              `json:"period"`
	TodayRevenue      int64               `json:"today_revenue"`
	TodayOrders       int64               `json:"today_orders"`
	TodayNewMembers   int64               `json:"today_new_members"`
	TotalRevenue      int64               `json:"total_revenue"`
	TotalOrders       int64               `json:"total_orders"`
	RevenueRank       int                 `json:"revenue_rank"`
	ProductSalesRank  []ProductSalesRank  `json:"product_sales_rank"`
	ServicePopularity []ServicePopularity `json:"service_popularity"`
}

// computeSince returns the start time for a given period.
func computeSince(period string) (time.Time, string) {
	now := time.Now()
	switch period {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), "today"
	case "week":
		weekday := now.Weekday()
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()-int(weekday)+1, 0, 0, 0, 0, now.Location()), "week"
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), "month"
	case "year":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()), "year"
	default:
		return time.Time{}, "all"
	}
}

// GetMerchantAnalysis returns comprehensive business analysis for a single merchant.
func (s *Service) GetMerchantAnalysis(ctx context.Context, merchantID int64, period string) (*MerchantAnalysisResponse, error) {
	since, effectivePeriod := computeSince(period)

	resp := &MerchantAnalysisResponse{
		MerchantID:   merchantID,
		Period:       effectivePeriod,
	}

	// Get merchant name.
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM merchants WHERE id = $1 AND deleted_at IS NULL`, merchantID).Scan(&name)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: fmt.Sprintf("merchant not found: %d", merchantID),
			Err:     err,
		}
	}
	resp.MerchantName = name

	// Today's revenue (always today, not period-based).
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
	s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_cents), 0) FROM orders WHERE merchant_id = $1 AND status = 'completed' AND created_at >= $2`,
		merchantID, today).Scan(&resp.TodayRevenue)

	// Today's orders.
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders WHERE merchant_id = $1 AND status = 'completed' AND created_at >= $2`,
		merchantID, today).Scan(&resp.TodayOrders)

	// Today's new members.
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM platform_users WHERE merchant_id = $1 AND deleted_at IS NULL AND created_at >= $2`,
		merchantID, today).Scan(&resp.TodayNewMembers)

	// Total revenue in period.
	if effectivePeriod != "all" {
		s.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(total_cents), 0) FROM orders WHERE merchant_id = $1 AND status = 'completed' AND created_at >= $2`,
			merchantID, since).Scan(&resp.TotalRevenue)
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM orders WHERE merchant_id = $1 AND status = 'completed' AND created_at >= $2`,
			merchantID, since).Scan(&resp.TotalOrders)
	} else {
		s.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(total_cents), 0) FROM orders WHERE merchant_id = $1 AND status = 'completed'`,
			merchantID).Scan(&resp.TotalRevenue)
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM orders WHERE merchant_id = $1 AND status = 'completed'`,
			merchantID).Scan(&resp.TotalOrders)
	}

	// Revenue rank: compute total revenue per merchant and find position.
	resp.RevenueRank = computeRevenueRank(ctx, s.db, merchantID, since, effectivePeriod)

	// Product sales ranking: top 10 by quantity sold.
	resp.ProductSalesRank = queryProductSalesRank(ctx, s.db, merchantID, since, effectivePeriod)

	// Service popularity: top 10 by order count from order_items (same data source for now).
	resp.ServicePopularity = queryServicePopularity(ctx, s.db, merchantID, since, effectivePeriod)

	return resp, nil
}

// GetMerchantsRevenueRanking returns all approved merchants ranked by total revenue.
func (s *Service) GetMerchantsRevenueRanking(ctx context.Context, period string) ([]MerchantRevenueRank, error) {
	since, effectivePeriod := computeSince(period)

	var rows *sql.Rows
	var err error

	if effectivePeriod != "all" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT m.id, m.name, COALESCE(SUM(o.total_cents), 0) AS total_revenue
			 FROM merchants m
			 LEFT JOIN orders o ON o.merchant_id = m.id AND o.status = 'completed' AND o.created_at >= $1
			 WHERE m.deleted_at IS NULL AND m.status = 'approved'
			 GROUP BY m.id, m.name
			 ORDER BY total_revenue DESC`, since)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT m.id, m.name, COALESCE(SUM(o.total_cents), 0) AS total_revenue
			 FROM merchants m
			 LEFT JOIN orders o ON o.merchant_id = m.id AND o.status = 'completed'
			 WHERE m.deleted_at IS NULL AND m.status = 'approved'
			 GROUP BY m.id, m.name
			 ORDER BY total_revenue DESC`)
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query revenue ranking",
			Err:     err,
		}
	}
	defer rows.Close()

	var ranking []MerchantRevenueRank
	rank := 0
	for rows.Next() {
		rank++
		var item MerchantRevenueRank
		if err := rows.Scan(&item.MerchantID, &item.MerchantName, &item.TotalRevenue); err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan ranking row",
				Err:     err,
			}
		}
		item.Rank = rank
		ranking = append(ranking, item)
	}
	if ranking == nil {
		ranking = []MerchantRevenueRank{}
	}
	return ranking, nil
}

func computeRevenueRank(ctx context.Context, db *sql.DB, merchantID int64, since time.Time, period string) int {
	var rows *sql.Rows
	var err error
	if period != "all" {
		rows, err = db.QueryContext(ctx,
			`SELECT m.id, COALESCE(SUM(o.total_cents), 0) AS total_revenue
			 FROM merchants m
			 LEFT JOIN orders o ON o.merchant_id = m.id AND o.status = 'completed' AND o.created_at >= $1
			 WHERE m.deleted_at IS NULL AND m.status = 'approved'
			 GROUP BY m.id
			 ORDER BY total_revenue DESC`, since)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT m.id, COALESCE(SUM(o.total_cents), 0) AS total_revenue
			 FROM merchants m
			 LEFT JOIN orders o ON o.merchant_id = m.id AND o.status = 'completed'
			 WHERE m.deleted_at IS NULL AND m.status = 'approved'
			 GROUP BY m.id
			 ORDER BY total_revenue DESC`)
	}
	if err != nil {
		return 0
	}
	defer rows.Close()

	rank := 0
	for rows.Next() {
		rank++
		var id int64
		var revenue int64
		if err := rows.Scan(&id, &revenue); err != nil {
			return 0
		}
		if id == merchantID {
			return rank
		}
	}
	return 0
}

func queryProductSalesRank(ctx context.Context, db *sql.DB, merchantID int64, since time.Time, period string) []ProductSalesRank {
	var rows *sql.Rows
	var err error
	if period != "all" {
		rows, err = db.QueryContext(ctx,
			`SELECT oi.product_id, oi.product_name, COALESCE(SUM(oi.quantity), 0) AS total_qty,
			        COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS total_rev
			 FROM order_items oi
			 JOIN orders o ON o.id = oi.order_id
			 WHERE o.merchant_id = $1 AND o.status = 'completed' AND o.created_at >= $2
			 GROUP BY oi.product_id, oi.product_name
			 ORDER BY total_qty DESC
			 LIMIT 10`, merchantID, since)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT oi.product_id, oi.product_name, COALESCE(SUM(oi.quantity), 0) AS total_qty,
			        COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS total_rev
			 FROM order_items oi
			 JOIN orders o ON o.id = oi.order_id
			 WHERE o.merchant_id = $1 AND o.status = 'completed'
			 GROUP BY oi.product_id, oi.product_name
			 ORDER BY total_qty DESC
			 LIMIT 10`, merchantID)
	}
	if err != nil {
		return []ProductSalesRank{}
	}
	defer rows.Close()

	var result []ProductSalesRank
	rank := 0
	for rows.Next() {
		rank++
		var item ProductSalesRank
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.Quantity, &item.Revenue); err != nil {
			continue
		}
		item.Rank = rank
		result = append(result, item)
	}
	if result == nil {
		result = []ProductSalesRank{}
	}
	return result
}

func queryServicePopularity(ctx context.Context, db *sql.DB, merchantID int64, since time.Time, period string) []ServicePopularity {
	var rows *sql.Rows
	var err error
	if period != "all" {
		rows, err = db.QueryContext(ctx,
			`SELECT p.id, p.name, COUNT(*) AS order_cnt,
			        COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS total_rev
			 FROM order_items oi
			 JOIN orders o ON o.id = oi.order_id
			 JOIN products p ON p.id = oi.product_id
			 WHERE o.merchant_id = $1 AND o.status = 'completed' AND o.created_at >= $2
			 GROUP BY p.id, p.name
			 ORDER BY order_cnt DESC
			 LIMIT 10`, merchantID, since)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT p.id, p.name, COUNT(*) AS order_cnt,
			        COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS total_rev
			 FROM order_items oi
			 JOIN orders o ON o.id = oi.order_id
			 JOIN products p ON p.id = oi.product_id
			 WHERE o.merchant_id = $1 AND o.status = 'completed'
			 GROUP BY p.id, p.name
			 ORDER BY order_cnt DESC
			 LIMIT 10`, merchantID)
	}
	if err != nil {
		return []ServicePopularity{}
	}
	defer rows.Close()

	var result []ServicePopularity
	rank := 0
	for rows.Next() {
		rank++
		var item ServicePopularity
		if err := rows.Scan(&item.ServiceID, &item.ServiceName, &item.OrderCount, &item.Revenue); err != nil {
			continue
		}
		item.Rank = rank
		result = append(result, item)
	}
	if result == nil {
		result = []ServicePopularity{}
	}
	return result
}
