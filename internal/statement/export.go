package statement

import (
	"context"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
	"github.com/xltxb/PetManage/pkg/apperrors"
)

// ---------------------------------------------------------------------------
// Time range parsing
// ---------------------------------------------------------------------------

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

func centsToYuan(cents int64) float64 {
	return float64(cents) / 100.0
}

func excelCell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

func excelHeaderStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "#1E293B"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#CBD5E0", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	return style
}

func excelTitleStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: "#0F172A"},
	})
	return style
}

func excelDataStyle(f *excelize.File) int {
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	return style
}

// ---------------------------------------------------------------------------
// 1. Profit & Loss Excel & PDF
// ---------------------------------------------------------------------------

func (s *Service) ExportProfitLossExcel(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	revenue, cost, expense, err := s.queryProfitLossData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", err
	}

	grossProfit := revenue - cost
	netProfit := grossProfit - expense

	f := excelize.NewFile()
	sheet := "利润表"
	f.SetSheetName("Sheet1", sheet)

	// Title
	f.SetCellValue(sheet, "A1", "利润表 (Profit & Loss)")
	f.MergeCell(sheet, "A1", "B1")
	f.SetCellStyle(sheet, "A1", "B1", excelTitleStyle(f))
	f.SetCellValue(sheet, "A2", fmt.Sprintf("时间范围: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")))
	f.MergeCell(sheet, "A2", "B2")

	// Headers
	headers := []string{"项目", "金额(元)"}
	for i, h := range headers {
		f.SetCellValue(sheet, excelCell(i+1, 4), h)
	}
	f.SetCellStyle(sheet, "A4", excelCell(len(headers), 4), excelHeaderStyle(f))

	// Data
	data := [][]string{
		{"营收", fmt.Sprintf("%.2f", centsToYuan(revenue))},
		{"成本", fmt.Sprintf("%.2f", centsToYuan(cost))},
		{"毛利", fmt.Sprintf("%.2f", centsToYuan(grossProfit))},
		{"费用", fmt.Sprintf("%.2f", centsToYuan(expense))},
		{"净利", fmt.Sprintf("%.2f", centsToYuan(netProfit))},
	}
	for i, row := range data {
		for j, val := range row {
			f.SetCellValue(sheet, excelCell(j+1, 5+i), val)
		}
		f.SetCellStyle(sheet, excelCell(1, 5+i), excelCell(2, 5+i), excelDataStyle(f))
	}

	f.SetColWidth(sheet, "A", "A", 15)
	f.SetColWidth(sheet, "B", "B", 20)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", apperrors.NewInternalError("generate Excel", err)
	}

	filename := fmt.Sprintf("profit_loss_%s_%s.xlsx", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

func (s *Service) ExportProfitLossPDF(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	revenue, cost, expense, err := s.queryProfitLossData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", err
	}

	grossProfit := revenue - cost
	netProfit := grossProfit - expense

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(190, 10, "Profit & Loss / 利润表", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(190, 6, fmt.Sprintf("Period: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Table headers
	pdf.SetFont("Helvetica", "B", 11)
	colWidths := []float64{80, 110}
	headers := []string{"Item / 项目", "Amount (CNY)"}
	pdf.SetFillColor(226, 232, 240)
	for i, h := range headers {
		align := "L"
		if i == 1 {
			align = "R"
		}
		pdf.CellFormat(colWidths[i], 8, h, "1", 0, align, true, 0, "")
	}
	pdf.Ln(-1)

	// Data rows
	pdf.SetFont("Helvetica", "", 11)
	rows := [][]string{
		{"Revenue / 营收", fmt.Sprintf("%.2f", centsToYuan(revenue))},
		{"Cost / 成本", fmt.Sprintf("%.2f", centsToYuan(cost))},
		{"Gross Profit / 毛利", fmt.Sprintf("%.2f", centsToYuan(grossProfit))},
		{"Expense / 费用", fmt.Sprintf("%.2f", centsToYuan(expense))},
		{"Net Profit / 净利", fmt.Sprintf("%.2f", centsToYuan(netProfit))},
	}
	for _, row := range rows {
		for i, val := range row {
			align := "L"
			if i == 1 {
				align = "R"
			}
			pdf.CellFormat(colWidths[i], 8, val, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(190, 5, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytesBuffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", apperrors.NewInternalError("generate PDF", err)
	}

	filename := fmt.Sprintf("profit_loss_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

// ---------------------------------------------------------------------------
// 2. Revenue Detail Excel & PDF
// ---------------------------------------------------------------------------

func (s *Service) ExportRevenueDetailExcel(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	items, err := s.queryRevenueDetailData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", err
	}

	f := excelize.NewFile()
	sheet := "营收明细"
	f.SetSheetName("Sheet1", sheet)

	f.SetCellValue(sheet, "A1", "营收明细表 (Revenue Detail)")
	f.MergeCell(sheet, "A1", "C1")
	f.SetCellStyle(sheet, "A1", "C1", excelTitleStyle(f))
	f.SetCellValue(sheet, "A2", fmt.Sprintf("时间范围: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")))
	f.MergeCell(sheet, "A2", "C2")

	headers := []string{"营收来源", "金额(元)", "订单数"}
	for i, h := range headers {
		f.SetCellValue(sheet, excelCell(i+1, 4), h)
	}
	f.SetCellStyle(sheet, "A4", excelCell(len(headers), 4), excelHeaderStyle(f))

	for i, item := range items {
		f.SetCellValue(sheet, excelCell(1, 5+i), item.Source)
		f.SetCellValue(sheet, excelCell(2, 5+i), fmt.Sprintf("%.2f", centsToYuan(item.AmountCents)))
		f.SetCellValue(sheet, excelCell(3, 5+i), item.OrderCount)
		f.SetCellStyle(sheet, excelCell(1, 5+i), excelCell(3, 5+i), excelDataStyle(f))
	}

	f.SetColWidth(sheet, "A", "A", 20)
	f.SetColWidth(sheet, "B", "C", 18)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", apperrors.NewInternalError("generate Excel", err)
	}

	filename := fmt.Sprintf("revenue_detail_%s_%s.xlsx", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

func (s *Service) ExportRevenueDetailPDF(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewInternalError("invalid time range: " + err.Error(), nil)
	}

	items, err := s.queryRevenueDetailData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", convertError(err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(190, 10, "Revenue Detail / 营收明细表", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(190, 6, fmt.Sprintf("Period: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	colWidths := []float64{80, 55, 55}
	hdrs := []string{"Source / 来源", "Amount (CNY)", "Orders"}
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetFillColor(226, 232, 240)
	for i, h := range hdrs {
		align := "L"
		if i > 0 {
			align = "R"
		}
		pdf.CellFormat(colWidths[i], 8, h, "1", 0, align, true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 11)
	for _, item := range items {
		vals := []string{
			item.Source,
			fmt.Sprintf("%.2f", centsToYuan(item.AmountCents)),
			fmt.Sprintf("%d", item.OrderCount),
		}
		for i, val := range vals {
			align := "L"
			if i > 0 {
				align = "R"
			}
			pdf.CellFormat(colWidths[i], 8, val, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(190, 5, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytesBuffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", apperrors.NewInternalError("generate PDF", err)
	}

	filename := fmt.Sprintf("revenue_detail_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

// ---------------------------------------------------------------------------
// 3. Product Sales Excel & PDF
// ---------------------------------------------------------------------------

func (s *Service) ExportProductSalesExcel(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	items, err := s.queryProductSalesData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", err
	}

	f := excelize.NewFile()
	sheet := "商品销售报表"
	f.SetSheetName("Sheet1", sheet)

	f.SetCellValue(sheet, "A1", "商品销售报表 (Product Sales Report)")
	f.MergeCell(sheet, "A1", "E1")
	f.SetCellStyle(sheet, "A1", "E1", excelTitleStyle(f))
	f.SetCellValue(sheet, "A2", fmt.Sprintf("时间范围: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")))
	f.MergeCell(sheet, "A2", "E2")

	headers := []string{"商品类别", "销售数量", "销售金额(元)", "成本(元)", "毛利(元)"}
	for i, h := range headers {
		f.SetCellValue(sheet, excelCell(i+1, 4), h)
	}
	f.SetCellStyle(sheet, "A4", excelCell(len(headers), 4), excelHeaderStyle(f))

	for i, item := range items {
		f.SetCellValue(sheet, excelCell(1, 5+i), item.CategoryName)
		f.SetCellValue(sheet, excelCell(2, 5+i), item.SalesCount)
		f.SetCellValue(sheet, excelCell(3, 5+i), fmt.Sprintf("%.2f", centsToYuan(item.AmountCents)))
		f.SetCellValue(sheet, excelCell(4, 5+i), fmt.Sprintf("%.2f", centsToYuan(item.CostCents)))
		f.SetCellValue(sheet, excelCell(5, 5+i), fmt.Sprintf("%.2f", centsToYuan(item.ProfitCents)))
		f.SetCellStyle(sheet, excelCell(1, 5+i), excelCell(5, 5+i), excelDataStyle(f))
	}

	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 18)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", apperrors.NewInternalError("generate Excel", err)
	}

	filename := fmt.Sprintf("product_sales_%s_%s.xlsx", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

func (s *Service) ExportProductSalesPDF(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewInternalError("invalid time range: " + err.Error(), nil)
	}

	items, err := s.queryProductSalesData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", convertError(err)
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(270, 10, "Product Sales Report / 商品销售报表", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(270, 6, fmt.Sprintf("Period: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	colWidths := []float64{70, 40, 50, 50, 60}
	hdrs := []string{"Category", "Qty", "Amount (CNY)", "Cost (CNY)", "Profit (CNY)"}
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(226, 232, 240)
	for i, h := range hdrs {
		align := "L"
		if i > 0 {
			align = "R"
		}
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, align, true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 10)
	for _, item := range items {
		vals := []string{
			item.CategoryName,
			fmt.Sprintf("%d", item.SalesCount),
			fmt.Sprintf("%.2f", centsToYuan(item.AmountCents)),
			fmt.Sprintf("%.2f", centsToYuan(item.CostCents)),
			fmt.Sprintf("%.2f", centsToYuan(item.ProfitCents)),
		}
		for i, val := range vals {
			align := "L"
			if i > 0 {
				align = "R"
			}
			pdf.CellFormat(colWidths[i], 7, val, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(270, 5, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytesBuffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", apperrors.NewInternalError("generate PDF", err)
	}

	filename := fmt.Sprintf("product_sales_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

// ---------------------------------------------------------------------------
// 4. Service Performance Excel & PDF
// ---------------------------------------------------------------------------

func (s *Service) ExportServicePerformanceExcel(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewValidationError("invalid time range: " + err.Error())
	}

	svcItems, techRankings, err := s.queryServicePerformanceData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", err
	}

	f := excelize.NewFile()
	sheet := "服务业绩报表"
	f.SetSheetName("Sheet1", sheet)
	dataStyle := excelDataStyle(f)

	f.SetCellValue(sheet, "A1", "服务业绩报表 (Service Performance Report)")
	f.MergeCell(sheet, "A1", "C1")
	f.SetCellStyle(sheet, "A1", "C1", excelTitleStyle(f))
	f.SetCellValue(sheet, "A2", fmt.Sprintf("时间范围: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")))
	f.MergeCell(sheet, "A2", "C2")

	// Section 1: Service Items
	f.SetCellValue(sheet, "A4", "服务项目业绩")
	f.MergeCell(sheet, "A4", "C4")
	svcHeaders := []string{"服务项目", "完成数量", "金额(元)"}
	for i, h := range svcHeaders {
		f.SetCellValue(sheet, excelCell(i+1, 5), h)
	}
	f.SetCellStyle(sheet, "A5", excelCell(len(svcHeaders), 5), excelHeaderStyle(f))

	row := 6
	for _, item := range svcItems {
		f.SetCellValue(sheet, excelCell(1, row), item.ServiceName)
		f.SetCellValue(sheet, excelCell(2, row), item.CompletionCount)
		f.SetCellValue(sheet, excelCell(3, row), fmt.Sprintf("%.2f", centsToYuan(item.AmountCents)))
		f.SetCellStyle(sheet, excelCell(1, row), excelCell(3, row), dataStyle)
		row++
	}

	// Section 2: Technician Ranking
	row++
	f.SetCellValue(sheet, excelCell(1, row), "技师业绩排名")
	f.MergeCell(sheet, excelCell(1, row), excelCell(3, row))
	row++
	techHeaders := []string{"技师姓名", "完成数量", "金额(元)"}
	for i, h := range techHeaders {
		f.SetCellValue(sheet, excelCell(i+1, row), h)
	}
	f.SetCellStyle(sheet, excelCell(1, row), excelCell(len(techHeaders), row), excelHeaderStyle(f))
	row++

	for _, tech := range techRankings {
		f.SetCellValue(sheet, excelCell(1, row), tech.EmployeeName)
		f.SetCellValue(sheet, excelCell(2, row), tech.CompletionCount)
		f.SetCellValue(sheet, excelCell(3, row), fmt.Sprintf("%.2f", centsToYuan(tech.AmountCents)))
		f.SetCellStyle(sheet, excelCell(1, row), excelCell(3, row), dataStyle)
		row++
	}

	for i := 1; i <= 3; i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 22)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", apperrors.NewInternalError("generate Excel", err)
	}

	filename := fmt.Sprintf("service_performance_%s_%s.xlsx", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

func (s *Service) ExportServicePerformancePDF(ctx context.Context, merchantID int64, startTime, endTime string) ([]byte, string, error) {
	start, end, err := parseTimeRange(startTime, endTime)
	if err != nil {
		return nil, "", apperrors.NewInternalError("invalid time range: " + err.Error(), nil)
	}

	svcItems, techRankings, err := s.queryServicePerformanceData(ctx, merchantID, start, end)
	if err != nil {
		return nil, "", convertError(err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(190, 10, "Service Performance / 服务业绩报表", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(190, 6, fmt.Sprintf("Period: %s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02")), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Service items table
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(190, 8, "Service Items / 服务项目业绩", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 11)
	svcColWidths := []float64{80, 50, 60}
	svcHdrs := []string{"Item", "Count", "Amount (CNY)"}
	pdf.SetFillColor(226, 232, 240)
	for i, h := range svcHdrs {
		align := "L"
		if i > 0 {
			align = "R"
		}
		pdf.CellFormat(svcColWidths[i], 7, h, "1", 0, align, true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 10)
	for _, item := range svcItems {
		vals := []string{
			item.ServiceName,
			fmt.Sprintf("%d", item.CompletionCount),
			fmt.Sprintf("%.2f", centsToYuan(item.AmountCents)),
		}
		for i, val := range vals {
			align := "L"
			if i > 0 {
				align = "R"
			}
			pdf.CellFormat(svcColWidths[i], 7, val, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	// Technician ranking table
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(190, 8, "Technician Ranking / 技师业绩排名", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 11)
	techColWidths := []float64{80, 50, 60}
	techHdrs := []string{"Technician", "Count", "Amount (CNY)"}
	pdf.SetFillColor(226, 232, 240)
	for i, h := range techHdrs {
		align := "L"
		if i > 0 {
			align = "R"
		}
		pdf.CellFormat(techColWidths[i], 7, h, "1", 0, align, true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 10)
	for _, tech := range techRankings {
		vals := []string{
			tech.EmployeeName,
			fmt.Sprintf("%d", tech.CompletionCount),
			fmt.Sprintf("%.2f", centsToYuan(tech.AmountCents)),
		}
		for i, val := range vals {
			align := "L"
			if i > 0 {
				align = "R"
			}
			pdf.CellFormat(techColWidths[i], 7, val, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(190, 5, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytesBuffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", apperrors.NewInternalError("generate PDF", err)
	}

	filename := fmt.Sprintf("service_performance_%s_%s.pdf", start.Format("20060102"), end.Format("20060102"))
	return buf.Bytes(), filename, nil
}

// ---------------------------------------------------------------------------
// Data query helpers
// ---------------------------------------------------------------------------

func (s *Service) queryProfitLossData(ctx context.Context, merchantID int64, start, end time.Time) (revenue, cost, expense int64, err error) {
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
		return 0, 0, 0, apperrors.NewInternalError("querying revenue", err)
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(p.cost_cents * oi.quantity), 0)
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 JOIN products p ON p.id = oi.product_id
		 WHERE o.merchant_id = $1
		   AND oi.product_id IS NOT NULL
		   AND o.status IN ('completed', 'partially_refunded', 'refunded')
		   AND o.created_at >= $2 AND o.created_at <= $3`,
		merchantID, start, end).Scan(&cost)
	if err != nil {
		return 0, 0, 0, apperrors.NewInternalError("querying cost", err)
	}

	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_cents), 0)
		 FROM purchase_orders
		 WHERE merchant_id = $1
		   AND status = 'received'
		   AND received_at >= $2 AND received_at <= $3`,
		merchantID, start, end).Scan(&expense)
	if err != nil {
		return 0, 0, 0, apperrors.NewInternalError("querying expenses", err)
	}

	return revenue, cost, expense, nil
}

func (s *Service) queryRevenueDetailData(ctx context.Context, merchantID int64, start, end time.Time) ([]RevenueDetailItem, error) {
	var items []RevenueDetailItem

	// Product sales
	var productRevenue int64
	var productCount int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(oi.price_cents * oi.quantity), 0), COUNT(DISTINCT o.id)
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
		items = append(items, RevenueDetailItem{Source: "商品销售", AmountCents: productRevenue, OrderCount: productCount})
	}

	// Service revenue
	var serviceRevenue int64
	var serviceCount int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(oi.price_cents * oi.quantity), 0), COUNT(DISTINCT o.id)
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
		items = append(items, RevenueDetailItem{Source: "服务收入", AmountCents: serviceRevenue, OrderCount: serviceCount})
	}

	// Recharge
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
		items = append(items, RevenueDetailItem{Source: "充值收入", AmountCents: rechargeRevenue, OrderCount: rechargeCount})
	}

	// Refund
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
		items = append(items, RevenueDetailItem{Source: "退款支出", AmountCents: -refundAmount, OrderCount: refundCount})
	}

	if items == nil {
		items = []RevenueDetailItem{}
	}
	return items, nil
}

func (s *Service) queryProductSalesData(ctx context.Context, merchantID int64, start, end time.Time) ([]ProductSalesItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT
			COALESCE(pc.name, '未分类') AS category_name,
			SUM(oi.quantity) AS sales_count,
			SUM(oi.price_cents * oi.quantity) AS amount_cents,
			SUM(p.cost_cents * oi.quantity) AS cost_cents,
			SUM((oi.price_cents - COALESCE(p.cost_cents, 0)) * oi.quantity) AS profit_cents
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
	if items == nil {
		items = []ProductSalesItem{}
	}
	return items, nil
}

func (s *Service) queryServicePerformanceData(ctx context.Context, merchantID int64, start, end time.Time) ([]ServicePerfItem, []TechnicianRanking, error) {
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
		return nil, nil, apperrors.NewInternalError("querying service performance", err)
	}
	defer rows.Close()

	var svcItems []ServicePerfItem
	for rows.Next() {
		var item ServicePerfItem
		if err := rows.Scan(&item.ServiceName, &item.CompletionCount, &item.AmountCents); err != nil {
			return nil, nil, apperrors.NewInternalError("scanning service performance", err)
		}
		svcItems = append(svcItems, item)
	}
	if svcItems == nil {
		svcItems = []ServicePerfItem{}
	}

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
		return nil, nil, apperrors.NewInternalError("querying technician ranking", err)
	}
	defer techRows.Close()

	var techRankings []TechnicianRanking
	for techRows.Next() {
		var rank TechnicianRanking
		if err := techRows.Scan(&rank.EmployeeName, &rank.CompletionCount, &rank.AmountCents); err != nil {
			return nil, nil, apperrors.NewInternalError("scanning technician ranking", err)
		}
		techRankings = append(techRankings, rank)
	}
	if techRankings == nil {
		techRankings = []TechnicianRanking{}
	}

	return svcItems, techRankings, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type bytesBuffer struct {
	buf []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesBuffer) Bytes() []byte {
	return b.buf
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*apperrors.AppError); ok {
		return appErr
	}
	return apperrors.NewInternalError("internal error", err)
}
