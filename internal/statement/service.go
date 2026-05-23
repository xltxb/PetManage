package statement

import (
	"context"
	"database/sql"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service provides automated financial statement generation.
type Service struct {
	db *sql.DB
}

// NewService creates a new statement Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ---------------------------------------------------------------------------
// 1. Profit & Loss (利润表)
// ---------------------------------------------------------------------------

// ProfitLossItem holds one row of the profit & loss statement.
type ProfitLossItem struct {
	Label       string `json:"label"`
	AmountCents int64  `json:"amount_cents"`
}

// ProfitLossResult holds the full profit & loss statement.
type ProfitLossResult struct {
	Year  int              `json:"year"`
	Month int              `json:"month"`
	Items []ProfitLossItem `json:"items"`
}

// GetProfitLoss generates a profit & loss statement for the given merchant and month.
func (s *Service) GetProfitLoss(ctx context.Context, merchantID int64, year, month int) (*ProfitLossResult, error) {
	start, end, err := monthRange(year, month)
	if err != nil {
		return nil, err
	}

	// Revenue: sum of order totals minus refunds (status = completed / partially_refunded / refunded)
	var revenue int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(o.total_cents), 0) -
		        COALESCE((SELECT SUM(r.amount_cents) FROM refunds r
		                  WHERE r.merchant_id = $1 AND r.status = 'completed'
		                    AND r.created_at >= $2 AND r.created_at <= $3), 0)
		 FROM orders o
		 WHERE o.merchant_id = $1
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND o.created_at >= $2 AND o.created_at <= $3`,
		merchantID, start, end).Scan(&revenue)
	if err != nil {
		return nil, apperrors.NewInternalError("querying revenue", err)
	}

	// Cost: frozen order item costs (product + service consumables)
	var productCost, serviceCost int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN oi.product_id IS NOT NULL THEN oi.cost_cents ELSE 0 END), 0),
		        COALESCE(SUM(CASE WHEN oi.service_item_id IS NOT NULL THEN oi.cost_cents ELSE 0 END), 0)
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 WHERE o.merchant_id = $1
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND o.created_at >= $2 AND o.created_at <= $3`,
		merchantID, start, end).Scan(&productCost, &serviceCost)
	if err != nil {
		return nil, apperrors.NewInternalError("querying cost", err)
	}
	totalCost := productCost + serviceCost

	// Commission expense: confirmed commission records
	var commissionExpense int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(commission_cents), 0)
		 FROM commission_records
		 WHERE merchant_id = $1
		   AND status = 'confirmed'
		   AND created_at >= $2 AND created_at <= $3`,
		merchantID, start, end).Scan(&commissionExpense)
	if err != nil {
		return nil, apperrors.NewInternalError("querying commission expenses", err)
	}

	// Fixed expenses: rent, utilities, salary, other
	var fixedExpenses int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0)
		 FROM fixed_expenses
		 WHERE merchant_id = $1 AND deleted_at IS NULL`,
		merchantID).Scan(&fixedExpenses)
	if err != nil {
		return nil, apperrors.NewInternalError("querying fixed expenses", err)
	}

	totalExpense := commissionExpense + fixedExpenses
	grossProfit := revenue - totalCost
	netProfit := grossProfit - totalExpense

	// Margin rates (percentage, avoid division by zero)
	grossMargin := 0.0
	netMargin := 0.0
	if revenue > 0 {
		grossMargin = float64(grossProfit) / float64(revenue) * 100
		netMargin = float64(netProfit) / float64(revenue) * 100
	}

	items := []ProfitLossItem{
		{Label: "营收", AmountCents: revenue},
		{Label: "商品成本", AmountCents: productCost},
		{Label: "服务耗材成本", AmountCents: serviceCost},
		{Label: "毛利", AmountCents: grossProfit},
		{Label: "毛利率", AmountCents: int64(grossMargin * 100)},
		{Label: "员工提成", AmountCents: commissionExpense},
		{Label: "固定费用", AmountCents: fixedExpenses},
		{Label: "净利", AmountCents: netProfit},
		{Label: "净利率", AmountCents: int64(netMargin * 100)},
	}

	return &ProfitLossResult{
		Year:  year,
		Month: month,
		Items: items,
	}, nil
}

// ---------------------------------------------------------------------------
// 2. Revenue Detail (营收明细表)
// ---------------------------------------------------------------------------

// RevenueDetailItem holds a single revenue source breakdown.
type RevenueDetailItem struct {
	Source      string `json:"source"`
	AmountCents int64  `json:"amount_cents"`
	OrderCount  int64  `json:"order_count"`
}

// RevenueDetailResult holds the revenue detail report.
type RevenueDetailResult struct {
	Year  int                `json:"year"`
	Month int                `json:"month"`
	Items []RevenueDetailItem `json:"items"`
}

// GetRevenueDetail generates a revenue source breakdown for the given merchant and month.
func (s *Service) GetRevenueDetail(ctx context.Context, merchantID int64, year, month int) (*RevenueDetailResult, error) {
	start, end, err := monthRange(year, month)
	if err != nil {
		return nil, err
	}

	var items []RevenueDetailItem

	// Product sales revenue
	var productRevenue int64
	var productCount int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(oi.price_cents * oi.quantity), 0),
		        COUNT(DISTINCT o.id)
		 FROM orders o
		 JOIN order_items oi ON oi.order_id = o.id
		 WHERE o.merchant_id = $1
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND oi.product_id IS NOT NULL
		   AND o.created_at >= $2 AND o.created_at <= $3`,
		merchantID, start, end).Scan(&productRevenue, &productCount)
	if err != nil {
		return nil, apperrors.NewInternalError("querying product revenue", err)
	}
	if productRevenue > 0 {
		items = append(items, RevenueDetailItem{
			Source:      "商品销售",
			AmountCents: productRevenue,
			OrderCount:  productCount,
		})
	}

	// Service revenue
	var serviceRevenue int64
	var serviceCount int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(oi.price_cents * oi.quantity), 0),
		        COUNT(DISTINCT o.id)
		 FROM orders o
		 JOIN order_items oi ON oi.order_id = o.id
		 WHERE o.merchant_id = $1
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND oi.service_item_id IS NOT NULL
		   AND o.created_at >= $2 AND o.created_at <= $3`,
		merchantID, start, end).Scan(&serviceRevenue, &serviceCount)
	if err != nil {
		return nil, apperrors.NewInternalError("querying service revenue", err)
	}
	if serviceRevenue > 0 {
		items = append(items, RevenueDetailItem{
			Source:      "服务收入",
			AmountCents: serviceRevenue,
			OrderCount:  serviceCount,
		})
	}

	// Recharge revenue
	var rechargeRevenue int64
	var rechargeCount int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0), COUNT(*)
		 FROM balance_transactions
		 WHERE merchant_id = $1
		   AND type = 'recharge'
		   AND created_at >= $2 AND created_at <= $3`,
		merchantID, start, end).Scan(&rechargeRevenue, &rechargeCount)
	if err != nil {
		return nil, apperrors.NewInternalError("querying recharge revenue", err)
	}
	if rechargeRevenue > 0 {
		items = append(items, RevenueDetailItem{
			Source:      "充值收入",
			AmountCents: rechargeRevenue,
			OrderCount:  rechargeCount,
		})
	}

	// Refund (deduction)
	var refundAmount int64
	var refundCount int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0), COUNT(*)
		 FROM refunds
		 WHERE merchant_id = $1
		   AND status = 'completed'
		   AND created_at >= $2 AND created_at <= $3`,
		merchantID, start, end).Scan(&refundAmount, &refundCount)
	if err != nil {
		return nil, apperrors.NewInternalError("querying refunds", err)
	}
	if refundAmount > 0 {
		items = append(items, RevenueDetailItem{
			Source:      "退款支出",
			AmountCents: -refundAmount,
			OrderCount:  refundCount,
		})
	}

	if items == nil {
		items = []RevenueDetailItem{}
	}

	return &RevenueDetailResult{
		Year:  year,
		Month: month,
		Items: items,
	}, nil
}

// ---------------------------------------------------------------------------
// 3. Product Sales Report (商品销售报表)
// ---------------------------------------------------------------------------

// ProductSalesItem holds sales data for one product category.
type ProductSalesItem struct {
	CategoryName string `json:"category_name"`
	SalesCount   int64  `json:"sales_count"`
	AmountCents  int64  `json:"amount_cents"`
	CostCents    int64  `json:"cost_cents"`
	ProfitCents  int64  `json:"profit_cents"`
}

// ProductSalesResult holds the product sales report.
type ProductSalesResult struct {
	Year  int                `json:"year"`
	Month int                `json:"month"`
	Items []ProductSalesItem `json:"items"`
}

// GetProductSales generates a product sales report grouped by category.
func (s *Service) GetProductSales(ctx context.Context, merchantID int64, year, month int) (*ProductSalesResult, error) {
	start, end, err := monthRange(year, month)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT
			COALESCE(pc.name, '未分类') AS category_name,
			SUM(oi.quantity) AS sales_count,
			SUM(oi.price_cents * oi.quantity) AS amount_cents,
			SUM(oi.cost_cents) AS cost_cents,
			SUM(oi.price_cents * oi.quantity) - SUM(oi.cost_cents) AS profit_cents
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 LEFT JOIN products p ON p.id = oi.product_id
		 LEFT JOIN product_categories pc ON pc.id = p.category_id
		 WHERE o.merchant_id = $1
		   AND oi.product_id IS NOT NULL
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND o.created_at >= $2 AND o.created_at <= $3
		 GROUP BY pc.id, pc.name
		 ORDER BY amount_cents DESC`,
		merchantID, start, end)
	if err != nil {
		return nil, apperrors.NewInternalError("querying product sales", err)
	}
	defer rows.Close()

	var items []ProductSalesItem
	for rows.Next() {
		var item ProductSalesItem
		if err := rows.Scan(&item.CategoryName, &item.SalesCount, &item.AmountCents, &item.CostCents, &item.ProfitCents); err != nil {
			return nil, apperrors.NewInternalError("scanning product sales", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternalError("iterating product sales", err)
	}
	if items == nil {
		items = []ProductSalesItem{}
	}

	return &ProductSalesResult{
		Year:  year,
		Month: month,
		Items: items,
	}, nil
}

// ---------------------------------------------------------------------------
// 4. Service Performance (服务业绩报表)
// ---------------------------------------------------------------------------

// ServicePerfItem holds performance data for one service item.
type ServicePerfItem struct {
	ServiceName    string `json:"service_name"`
	CompletionCount int64 `json:"completion_count"`
	AmountCents    int64  `json:"amount_cents"`
}

// TechnicianRanking holds one technician's ranking data.
type TechnicianRanking struct {
	EmployeeName   string `json:"employee_name"`
	CompletionCount int64 `json:"completion_count"`
	AmountCents    int64  `json:"amount_cents"`
}

// ServicePerformanceResult holds the service performance report.
type ServicePerformanceResult struct {
	Year              int                  `json:"year"`
	Month             int                  `json:"month"`
	ServiceItems      []ServicePerfItem    `json:"service_items"`
	TechnicianRanking []TechnicianRanking  `json:"technician_ranking"`
}

// GetServicePerformance generates a service performance report.
func (s *Service) GetServicePerformance(ctx context.Context, merchantID int64, year, month int) (*ServicePerformanceResult, error) {
	start, end, err := monthRange(year, month)
	if err != nil {
		return nil, err
	}

	// Service item performance from completed orders
	rows, err := s.db.QueryContext(ctx,
		`SELECT
			COALESCE(si.name, oi.product_name, '未知服务') AS service_name,
			SUM(oi.quantity) AS completion_count,
			SUM(oi.price_cents * oi.quantity) AS amount_cents
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 LEFT JOIN service_items si ON si.id = oi.service_item_id
		 WHERE o.merchant_id = $1
		   AND oi.service_item_id IS NOT NULL
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND o.created_at >= $2 AND o.created_at <= $3
		 GROUP BY si.id, si.name, oi.product_name
		 ORDER BY amount_cents DESC`,
		merchantID, start, end)
	if err != nil {
		return nil, apperrors.NewInternalError("querying service performance", err)
	}
	defer rows.Close()

	var svcItems []ServicePerfItem
	for rows.Next() {
		var item ServicePerfItem
		if err := rows.Scan(&item.ServiceName, &item.CompletionCount, &item.AmountCents); err != nil {
			return nil, apperrors.NewInternalError("scanning service performance", err)
		}
		svcItems = append(svcItems, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternalError("iterating service performance", err)
	}
	if svcItems == nil {
		svcItems = []ServicePerfItem{}
	}

	// Technician ranking from appointments + commission records
	techRows, err := s.db.QueryContext(ctx,
		`SELECT
			e.name AS employee_name,
			COUNT(DISTINCT a.id) AS completion_count,
			COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS amount_cents
		 FROM appointments a
		 JOIN employees e ON e.id = a.employee_id
		 JOIN orders o ON o.merchant_id = a.merchant_id
		      AND o.created_at >= $2 AND o.created_at <= $3
		      AND o.status IN ('completed', 'partially_refunded', 'refunded')
		 JOIN order_items oi ON oi.order_id = o.id AND oi.service_item_id = a.service_item_id
		 WHERE a.merchant_id = $1
		   AND a.status IN ('arrived', 'in_progress', 'completed', 'picked_up')
		 GROUP BY e.id, e.name
		 ORDER BY amount_cents DESC`,
		merchantID, start, end)
	if err != nil {
		return nil, apperrors.NewInternalError("querying technician ranking", err)
	}
	defer techRows.Close()

	var techRankings []TechnicianRanking
	for techRows.Next() {
		var rank TechnicianRanking
		if err := techRows.Scan(&rank.EmployeeName, &rank.CompletionCount, &rank.AmountCents); err != nil {
			return nil, apperrors.NewInternalError("scanning technician ranking", err)
		}
		techRankings = append(techRankings, rank)
	}
	if err := techRows.Err(); err != nil {
		return nil, apperrors.NewInternalError("iterating technician ranking", err)
	}
	if techRankings == nil {
		techRankings = []TechnicianRanking{}
	}

	return &ServicePerformanceResult{
		Year:              year,
		Month:             month,
		ServiceItems:      svcItems,
		TechnicianRanking: techRankings,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func monthRange(year, month int) (time.Time, time.Time, error) {
	if year <= 0 {
		return time.Time{}, time.Time{}, apperrors.NewValidationError("year is required")
	}

	m := time.Month(month)
	if month < 1 || month > 12 {
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(year, time.December, 31, 23, 59, 59, 0, time.UTC)
		return start, end, nil
	}

	start := time.Date(year, m, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.UTC)
	return start, end, nil
}
