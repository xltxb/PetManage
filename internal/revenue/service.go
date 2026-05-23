package revenue

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service provides revenue statistics and transaction detail queries.
type Service struct {
	db *sql.DB
}

// NewService creates a new revenue Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

// SummaryItem holds revenue breakdown for a single period.
type SummaryItem struct {
	Period         string `json:"period"`
	ProductRevenue int64  `json:"product_revenue_cents"`
	ServiceRevenue int64  `json:"service_revenue_cents"`
	RechargeAmount int64  `json:"recharge_amount_cents"`
	RefundAmount   int64  `json:"refund_amount_cents"`
}

// SummaryResult holds the revenue summary response.
type SummaryResult struct {
	GroupBy string        `json:"group_by"`
	Items   []SummaryItem `json:"items"`
	Total   SummaryItem   `json:"total"`
}

// GetSummary returns revenue summary for a merchant grouped by day/week/month.
// groupBy: "day", "week", "month" (default "day").
func (s *Service) GetSummary(ctx context.Context, merchantID int64, startDate, endDate string, groupBy string) (*SummaryResult, error) {
	if groupBy == "" {
		groupBy = "day"
	}
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		return nil, apperrors.NewValidationError("group_by must be one of: day, week, month")
	}

	trunc := "day"
	switch groupBy {
	case "week":
		trunc = "week"
	case "month":
		trunc = "month"
	}

	query := fmt.Sprintf(`
SELECT
	period,
	COALESCE(SUM(product_revenue), 0) AS product_revenue,
	COALESCE(SUM(service_revenue), 0) AS service_revenue,
	COALESCE(SUM(recharge_amount), 0) AS recharge_amount,
	COALESCE(SUM(refund_amount), 0) AS refund_amount
FROM (
	-- Product revenue
	SELECT
		date_trunc('%s', o.created_at)::date::text AS period,
		COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS product_revenue,
		0 AS service_revenue,
		0 AS recharge_amount,
		0 AS refund_amount
	FROM orders o
	JOIN order_items oi ON oi.order_id = o.id
	WHERE o.merchant_id = $1
	  AND o.status IN ('completed', 'partially_refunded', 'refunded')
	  AND oi.service_item_id IS NULL
	  AND o.created_at::date >= $2::date
	  AND o.created_at::date <= $3::date
	GROUP BY date_trunc('%s', o.created_at)::date::text

	UNION ALL

	-- Service revenue
	SELECT
		date_trunc('%s', o.created_at)::date::text AS period,
		0 AS product_revenue,
		COALESCE(SUM(oi.price_cents * oi.quantity), 0) AS service_revenue,
		0 AS recharge_amount,
		0 AS refund_amount
	FROM orders o
	JOIN order_items oi ON oi.order_id = o.id
	WHERE o.merchant_id = $1
	  AND o.status IN ('completed', 'partially_refunded', 'refunded')
	  AND oi.service_item_id IS NOT NULL
	  AND o.created_at::date >= $2::date
	  AND o.created_at::date <= $3::date
	GROUP BY date_trunc('%s', o.created_at)::date::text

	UNION ALL

	-- Recharge (stored value)
	SELECT
		date_trunc('%s', bt.created_at)::date::text AS period,
		0 AS product_revenue,
		0 AS service_revenue,
		COALESCE(SUM(bt.amount_cents), 0) AS recharge_amount,
		0 AS refund_amount
	FROM balance_transactions bt
	WHERE bt.merchant_id = $1
	  AND bt.type = 'recharge'
	  AND bt.created_at::date >= $2::date
	  AND bt.created_at::date <= $3::date
	GROUP BY date_trunc('%s', bt.created_at)::date::text

	UNION ALL

	-- Refunds
	SELECT
		date_trunc('%s', r.created_at)::date::text AS period,
		0 AS product_revenue,
		0 AS service_revenue,
		0 AS recharge_amount,
		COALESCE(SUM(r.amount_cents), 0) AS refund_amount
	FROM refunds r
	WHERE r.merchant_id = $1
	  AND r.status = 'completed'
	  AND r.created_at::date >= $2::date
	  AND r.created_at::date <= $3::date
	GROUP BY date_trunc('%s', r.created_at)::date::text
) sub
GROUP BY period
ORDER BY period
`, trunc, trunc, trunc, trunc, trunc, trunc, trunc, trunc)

	rows, err := s.db.QueryContext(ctx, query, merchantID, startDate, endDate)
	if err != nil {
		return nil, apperrors.NewInternalError("querying revenue summary", err)
	}
	defer rows.Close()

	var items []SummaryItem
	var total SummaryItem
	total.Period = "合计"

	for rows.Next() {
		var item SummaryItem
		if err := rows.Scan(&item.Period, &item.ProductRevenue, &item.ServiceRevenue, &item.RechargeAmount, &item.RefundAmount); err != nil {
			return nil, apperrors.NewInternalError("scanning revenue summary", err)
		}
		items = append(items, item)
		total.ProductRevenue += item.ProductRevenue
		total.ServiceRevenue += item.ServiceRevenue
		total.RechargeAmount += item.RechargeAmount
		total.RefundAmount += item.RefundAmount
	}

	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternalError("iterating revenue summary", err)
	}

	if items == nil {
		items = []SummaryItem{}
	}

	return &SummaryResult{
		GroupBy: groupBy,
		Items:   items,
		Total:   total,
	}, nil
}

// ---------------------------------------------------------------------------
// Transaction list
// ---------------------------------------------------------------------------

// TransactionItem represents a single financial transaction in the unified ledger.
type TransactionItem struct {
	ID              int64     `json:"id"`
	TransactionType string    `json:"transaction_type"`
	AmountCents     int64     `json:"amount_cents"`
	PaymentMethod   string    `json:"payment_method"`
	TransactionTime time.Time `json:"transaction_time"`
	OperatorName    string    `json:"operator_name"`
	SourceType      string    `json:"source_type"`
	SourceID        int64     `json:"source_id"`
	MemberName      string    `json:"member_name"`
}

// ListTransactionsParams holds filters for the transaction list.
type ListTransactionsParams struct {
	StartDate     string
	EndDate       string
	Type          string // "all", "sale", "recharge", "refund"
	PaymentMethod string // "all", "cash", "wechat", "alipay", "balance"
	Page          int
	PageSize      int
}

// ListTransactionsResult holds the paginated transaction list response.
type ListTransactionsResult struct {
	Transactions []TransactionItem `json:"transactions"`
	Total        int               `json:"total"`
	Page         int               `json:"page"`
	PageSize     int               `json:"page_size"`
	TotalPages   int               `json:"total_pages"`
}

// ListTransactions returns a unified list of all financial transactions for a merchant.
// All filters (date range, type, payment method) are applied as outer WHERE on the UNION result.
func (s *Service) ListTransactions(ctx context.Context, merchantID int64, params ListTransactionsParams) (*ListTransactionsResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	if params.Type == "" {
		params.Type = "all"
	}
	if params.PaymentMethod == "" {
		params.PaymentMethod = "all"
	}

	// Build UNION ALL subqueries — $1 is merchant_id in every part.
	unionParts := []string{
		`SELECT
			o.id AS source_id,
			'sale' AS transaction_type,
			o.total_cents AS amount_cents,
			COALESCE(p.method, '') AS payment_method,
			o.created_at AS transaction_time,
			'order' AS source_type,
			COALESCE(m.name, '') AS member_name
		FROM orders o
		LEFT JOIN LATERAL (
			SELECT method FROM payments WHERE order_id = o.id LIMIT 1
		) p ON true
		LEFT JOIN members m ON m.id = o.member_id
		WHERE o.merchant_id = $1
		  AND o.status IN ('completed', 'partially_refunded', 'refunded')`,

		`SELECT
			bt.id AS source_id,
			'recharge' AS transaction_type,
			bt.amount_cents,
			COALESCE(bt.payment_method, '') AS payment_method,
			bt.created_at AS transaction_time,
			'balance' AS source_type,
			COALESCE(m.name, '') AS member_name
		FROM balance_transactions bt
		LEFT JOIN members m ON m.id = bt.member_id
		WHERE bt.merchant_id = $1
		  AND bt.type = 'recharge'`,

		`SELECT
			r.id AS source_id,
			'refund' AS transaction_type,
			r.amount_cents,
			'' AS payment_method,
			r.created_at AS transaction_time,
			'refund' AS source_type,
			COALESCE(m.name, '') AS member_name
		FROM refunds r
		LEFT JOIN orders o2 ON o2.id = r.order_id
		LEFT JOIN members m ON m.id = o2.member_id
		WHERE r.merchant_id = $1
		  AND r.status = 'completed'`,
	}

	// Filter by type (keep only matching UNION parts).
	if params.Type != "all" {
		filtered := []string{}
		for _, part := range unionParts {
			if strings.Contains(part, fmt.Sprintf("'%s' AS transaction_type", params.Type)) {
				filtered = append(filtered, part)
			}
		}
		if len(filtered) == 0 {
			return &ListTransactionsResult{
				Transactions: []TransactionItem{},
				Total:        0,
				Page:         params.Page,
				PageSize:     params.PageSize,
			}, nil
		}
		unionParts = filtered
	}

	innerQuery := strings.Join(unionParts, " UNION ALL ")

	// Build outer WHERE conditions with parameterised placeholders.
	// $1 is merchantID (used inside subqueries).
	// Outer placeholders start from $2.
	var outerConds []string
	outerArgs := []interface{}{merchantID}
	argIdx := 2

	if params.PaymentMethod != "all" {
		outerConds = append(outerConds, fmt.Sprintf("payment_method = $%d", argIdx))
		outerArgs = append(outerArgs, params.PaymentMethod)
		argIdx++
	}
	if params.StartDate != "" {
		outerConds = append(outerConds, fmt.Sprintf("transaction_time::date >= $%d::date", argIdx))
		outerArgs = append(outerArgs, params.StartDate)
		argIdx++
	}
	if params.EndDate != "" {
		outerConds = append(outerConds, fmt.Sprintf("transaction_time::date <= $%d::date", argIdx))
		outerArgs = append(outerArgs, params.EndDate)
		argIdx++
	}

	outerWhere := ""
	if len(outerConds) > 0 {
		outerWhere = " WHERE " + strings.Join(outerConds, " AND ")
	}

	// Count query.
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub%s", innerQuery, outerWhere)
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, outerArgs...).Scan(&total); err != nil {
		return nil, apperrors.NewInternalError("counting transactions", err)
	}

	// Page query.
	offset := (params.Page - 1) * params.PageSize
	dataQuery := fmt.Sprintf(`
SELECT source_id, transaction_type, amount_cents, payment_method,
       transaction_time, source_type, member_name
FROM (%s) sub
%s
ORDER BY transaction_time DESC
LIMIT $%d OFFSET $%d
`, innerQuery, outerWhere, argIdx, argIdx+1)

	allArgs := append(outerArgs, params.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, allArgs...)
	if err != nil {
		return nil, apperrors.NewInternalError("querying transactions", err)
	}
	defer rows.Close()

	var txs []TransactionItem
	for rows.Next() {
		var tx TransactionItem
		if err := rows.Scan(&tx.SourceID, &tx.TransactionType, &tx.AmountCents,
			&tx.PaymentMethod, &tx.TransactionTime, &tx.SourceType, &tx.MemberName); err != nil {
			return nil, apperrors.NewInternalError("scanning transaction", err)
		}
		tx.ID = tx.SourceID
		txs = append(txs, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, apperrors.NewInternalError("iterating transactions", err)
	}

	if txs == nil {
		txs = []TransactionItem{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &ListTransactionsResult{
		Transactions: txs,
		Total:        total,
		Page:         params.Page,
		PageSize:     params.PageSize,
		TotalPages:   totalPages,
	}, nil
}
