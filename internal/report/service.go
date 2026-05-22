package report

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service provides report export capabilities.
type Service struct {
	db *sql.DB
}

// NewService creates a new report Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ExportOperatingReport generates an Excel file with platform-wide operating data.
func (s *Service) ExportOperatingReport(ctx context.Context, startTime, endTime string) ([]byte, string, error) {
	f := excelize.NewFile()
	sheet := "商户经营概况"
	f.SetSheetName("Sheet1", sheet)

	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	headers := []string{"商户名称", "营业执照号", "状态", "合同状态", "入驻时间", "总营收(元)", "总订单数", "新增会员数", "商品数"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#CBD5E0", Style: 1},
		},
	})
	f.SetCellStyle(sheet, "A1", cellName(len(headers), 1), headerStyle)

	rows, err := s.db.QueryContext(ctx,
		`SELECT m.name, m.license_number, m.status, m.created_at,
		        COALESCE(ct.status, '') AS contract_status,
		        COALESCE(SUM(o.total_cents), 0) AS total_revenue,
		        COUNT(o.id) AS total_orders,
		        COALESCE((SELECT COUNT(*) FROM platform_users pu
		                  WHERE pu.merchant_id = m.id AND pu.deleted_at IS NULL
		                  AND pu.created_at >= $1 AND pu.created_at <= $2), 0) AS new_members,
		        COALESCE((SELECT COUNT(*) FROM products p
		                  WHERE p.merchant_id = m.id AND p.deleted_at IS NULL), 0) AS product_count
		 FROM merchants m
		 LEFT JOIN merchant_contracts ct ON ct.merchant_id = m.id AND ct.deleted_at IS NULL AND ct.is_current = true
		 LEFT JOIN orders o ON o.merchant_id = m.id AND o.status = 'completed'
		      AND o.created_at >= $1 AND o.created_at <= $2
		 WHERE m.deleted_at IS NULL
		 GROUP BY m.id, m.name, m.license_number, m.status, m.created_at, ct.status
		 ORDER BY total_revenue DESC`, start, end)
	if err != nil {
		return nil, "", &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query operating report data",
			Err:     err,
		}
	}
	defer rows.Close()

	statusLabels := map[string]string{
		"pending": "待审核", "approved": "已通过", "rejected": "已驳回",
		"frozen": "已冻结", "closed": "已关停",
	}

	rowNum := 2
	for rows.Next() {
		var name, licenseNumber, status, contractStatus string
		var createdAt time.Time
		var totalRevenue, totalOrders, newMembers, productCount int64

		if err := rows.Scan(&name, &licenseNumber, &status, &createdAt,
			&contractStatus, &totalRevenue, &totalOrders, &newMembers, &productCount); err != nil {
			continue
		}

		statusDisplay := statusLabels[status]
		if statusDisplay == "" {
			statusDisplay = status
		}
		contractDisplay := "无"
		if contractStatus == "active" {
			contractDisplay = "有效"
		} else if contractStatus == "expired" {
			contractDisplay = "已过期"
		}

		revenueYuan := float64(totalRevenue) / 100.0

		f.SetCellValue(sheet, cellName(1, rowNum), name)
		f.SetCellValue(sheet, cellName(2, rowNum), licenseNumber)
		f.SetCellValue(sheet, cellName(3, rowNum), statusDisplay)
		f.SetCellValue(sheet, cellName(4, rowNum), contractDisplay)
		f.SetCellValue(sheet, cellName(5, rowNum), createdAt.Format("2006-01-02"))
		f.SetCellValue(sheet, cellName(6, rowNum), revenueYuan)
		f.SetCellValue(sheet, cellName(7, rowNum), totalOrders)
		f.SetCellValue(sheet, cellName(8, rowNum), newMembers)
		f.SetCellValue(sheet, cellName(9, rowNum), productCount)
		rowNum++
	}

	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 18)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to generate Excel file",
			Err:     err,
		}
	}

	filename := fmt.Sprintf("operating_report_%s_%s.xlsx",
		start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

// ExportTransactionReport generates an Excel file with transaction data.
func (s *Service) ExportTransactionReport(ctx context.Context, startTime, endTime string) ([]byte, string, error) {
	f := excelize.NewFile()
	sheet := "交易明细"
	f.SetSheetName("Sheet1", sheet)

	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	headers := []string{"订单编号", "商户名称", "下单时间", "商品明细", "数量", "订单金额(元)", "已付金额(元)", "支付方式", "订单状态"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#CBD5E0", Style: 1},
		},
	})
	f.SetCellStyle(sheet, "A1", cellName(len(headers), 1), headerStyle)

	rows, err := s.db.QueryContext(ctx,
		`SELECT o.id, m.name, o.created_at,
		        COALESCE(
		            (SELECT STRING_AGG(oi.product_name || ' x' || oi.quantity, '; ' ORDER BY oi.id)
		             FROM order_items oi WHERE oi.order_id = o.id), '') AS items_json,
		        o.total_cents, o.paid_cents,
		        COALESCE(
		            (SELECT STRING_AGG(p.method || ' ¥' || (p.amount_cents::numeric / 100)::text, '; ' ORDER BY p.id)
		             FROM payments p WHERE p.order_id = o.id), '') AS payment_summary,
		        o.status
		 FROM orders o
		 JOIN merchants m ON m.id = o.merchant_id AND m.deleted_at IS NULL
		 WHERE o.created_at >= $1 AND o.created_at <= $2
		 ORDER BY o.created_at DESC`, start, end)
	if err != nil {
		return nil, "", &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query transaction report data",
			Err:     err,
		}
	}
	defer rows.Close()

	statusLabels := map[string]string{
		"pending": "待支付", "paid": "已支付", "completed": "已完成", "cancelled": "已取消", "refunded": "已退款",
	}

	rowNum := 2
	for rows.Next() {
		var id int64
		var merchantName string
		var createdAt time.Time
		var itemsJSON string
		var totalCents, paidCents int64
		var paymentSummary string
		var status string

		if err := rows.Scan(&id, &merchantName, &createdAt, &itemsJSON,
			&totalCents, &paidCents, &paymentSummary, &status); err != nil {
			continue
		}

		statusDisplay := statusLabels[status]
		if statusDisplay == "" {
			statusDisplay = status
		}

		totalYuan := float64(totalCents) / 100.0
		paidYuan := float64(paidCents) / 100.0

		f.SetCellValue(sheet, cellName(1, rowNum), fmt.Sprintf("ORD-%d", id))
		f.SetCellValue(sheet, cellName(2, rowNum), merchantName)
		f.SetCellValue(sheet, cellName(3, rowNum), createdAt.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheet, cellName(4, rowNum), itemsJSON)
		f.SetCellValue(sheet, cellName(5, rowNum), computeTotalQty(itemsJSON))
		f.SetCellValue(sheet, cellName(6, rowNum), totalYuan)
		f.SetCellValue(sheet, cellName(7, rowNum), paidYuan)
		f.SetCellValue(sheet, cellName(8, rowNum), formatPaymentSummary(paymentSummary))
		f.SetCellValue(sheet, cellName(9, rowNum), statusDisplay)
		rowNum++
	}

	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		width := 18.0
		if i == 4 || i == 8 {
			width = 30.0
		}
		f.SetColWidth(sheet, col, col, width)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to generate Excel file",
			Err:     err,
		}
	}

	filename := fmt.Sprintf("transaction_report_%s_%s.xlsx",
		start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

func computeTotalQty(itemsJSON string) int {
	if itemsJSON == "" {
		return 0
	}
	total := 0
	parts := splitItems(itemsJSON)
	for _, part := range parts {
		for i := len(part) - 1; i >= 0; i-- {
			if part[i] == 'x' {
				num := 0
				for j := i + 1; j < len(part) && part[j] >= '0' && part[j] <= '9'; j++ {
					num = num*10 + int(part[j]-'0')
				}
				total += num
				break
			}
		}
	}
	return total
}

func splitItems(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' && i+1 < len(s) && s[i+1] == ' ' {
			parts = append(parts, s[start:i])
			start = i + 2
			i++
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func parseTimeRange(startTime, endTime string) (time.Time, time.Time, error) {
	if startTime == "" || endTime == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start_time and end_time are required")
	}

	layouts := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	var start, end time.Time
	var err error

	for _, layout := range layouts {
		start, err = time.Parse(layout, startTime)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot parse start_time: %s", startTime)
	}

	for _, layout := range layouts {
		end, err = time.Parse(layout, endTime)
		if err == nil {
			break
		}
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot parse end_time: %s", endTime)
	}

	if len(endTime) <= 10 {
		end = end.Add(24*time.Hour - time.Second)
	}

	return start, end, nil
}

func formatPaymentSummary(s string) string {
	if s == "" {
		return ""
	}
	parts := splitItems(s)
	var cleaned []string
	for _, part := range parts {
		idx := strings.Index(part, "¥")
		if idx >= 0 {
			method := part[:idx]
			amountStr := part[idx+len("¥"):]
			var amount float64
			if _, err := fmt.Sscanf(amountStr, "%f", &amount); err == nil {
				cleaned = append(cleaned, fmt.Sprintf("%s¥%.2f", method, amount))
			} else {
				cleaned = append(cleaned, part)
			}
		} else {
			cleaned = append(cleaned, part)
		}
	}
	return joinItems(cleaned)
}

func joinItems(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "; " + parts[i]
	}
	return result
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
