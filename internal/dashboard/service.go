package dashboard

import (
	"context"
	"database/sql"
	"time"
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
