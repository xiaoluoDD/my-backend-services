package ganttexport

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	sheetName     = "甘特图"
	headerRows    = 6
	firstDataRow  = 7
	calendarStart = 8 // H 列起为按日格子
	maxDays       = 400
	maxRows       = 500
)

// Row 甘特一行。Kind 为 group（部门标题）或 task。
type Row struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Owner    string `json:"owner"`
	Progress string `json:"progress"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

// Request 导出请求。日期用 YYYY-MM-DD。
type Request struct {
	Title        string `json:"title"`
	WorkNo       string `json:"work_no"`
	Manager      string `json:"manager"`
	ProjectStart string `json:"project_start"`
	Rows         []Row  `json:"rows"`
}

// Build 生成带条件格式的 xlsx。
// 色条由「状态 + 开始/结束日期」决定，在 Excel 里改日期或状态后会自动重算。
func Build(req Request) ([]byte, error) {
	if len(req.Rows) == 0 {
		return nil, fmt.Errorf("没有可导出的行")
	}
	if len(req.Rows) > maxRows {
		return nil, fmt.Errorf("行数超过 %d", maxRows)
	}

	start, end := timeline(req)
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days > maxDays {
		days = maxDays
		end = start.AddDate(0, 0, days-1)
	}

	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, err
	}

	lastCol, err := excelize.ColumnNumberToName(calendarStart + days - 1)
	if err != nil {
		return nil, err
	}
	lastDataRow := firstDataRow + len(req.Rows) - 1

	if err := writeChrome(f, req, start, days, lastCol); err != nil {
		return nil, err
	}
	if err := writeRows(f, req.Rows, firstDataRow); err != nil {
		return nil, err
	}
	if err := writeConditionalFormats(f, lastCol, lastDataRow); err != nil {
		return nil, err
	}
	if err := layout(f, lastCol, lastDataRow); err != nil {
		return nil, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func timeline(req Request) (time.Time, time.Time) {
	var min, max time.Time
	touch := func(d time.Time, ok bool) {
		if !ok {
			return
		}
		if min.IsZero() || d.Before(min) {
			min = d
		}
		if max.IsZero() || d.After(max) {
			max = d
		}
	}
	touch(parseDate(req.ProjectStart))
	for _, row := range req.Rows {
		touch(parseDate(row.Start))
		touch(parseDate(row.End))
	}
	if min.IsZero() || max.IsZero() {
		now := time.Now()
		min = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		max = min.AddDate(0, 1, -1)
	}
	return min.AddDate(0, 0, -1), max.AddDate(0, 0, 1)
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		s = s[:10]
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func excelSerial(t time.Time) float64 {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	u := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return float64(u.Sub(epoch) / (24 * time.Hour))
}

func writeChrome(f *excelize.File, req Request, start time.Time, days int, lastCol string) error {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "项目甘特图"
	}
	if wn := strings.TrimSpace(req.WorkNo); wn != "" {
		title = wn + " " + title
	}
	if err := f.SetCellStr(sheetName, "A1", title); err != nil {
		return err
	}
	note := fmt.Sprintf("负责人：%s    色条随「状态 + 开始/结束日期」自动变化，可直接改。进度可手填。天数是公式。", strings.TrimSpace(req.Manager))
	if err := f.SetCellStr(sheetName, "A2", note); err != nil {
		return err
	}

	// 图例放在日期区第一行，用单元格底色，不画图形。
	legends := []struct {
		col   int
		text  string
		color string
	}{
		{calendarStart, "进行中", "29A3FF"},
		{calendarStart + 2, "逾期", "FF4D4F"},
		{calendarStart + 4, "待启动", "FDC76F"},
		{calendarStart + 6, "已完结", "70AD47"},
		{calendarStart + 8, "部门", "5B9BD5"},
		{calendarStart + 10, "今天", "E53935"},
	}
	for _, lg := range legends {
		if lg.col > calendarStart+days-1 {
			break
		}
		cell, _ := excelize.CoordinatesToCellName(lg.col, 1)
		if err := f.SetCellStr(sheetName, cell, lg.text); err != nil {
			return err
		}
		style, err := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{lg.color}, Pattern: 1},
			Font: &excelize.Font{Family: "微软雅黑", Size: 9, Color: "FFFFFF", Bold: true},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		if err != nil {
			return err
		}
		if err := f.SetCellStyle(sheetName, cell, cell, style); err != nil {
			return err
		}
	}

	headers := []string{"计划任务项目", "状态", "责任人", "进度", "开始日期", "结束日期", "天数"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 6)
		if err := f.SetCellStr(sheetName, cell, h); err != nil {
			return err
		}
	}

	week := []string{"日", "一", "二", "三", "四", "五", "六"}
	var monthStart int
	var monthKey string
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		col := calendarStart + i
		key := fmt.Sprintf("%d-%02d", d.Year(), d.Month())
		if key != monthKey {
			if monthKey != "" && col-1 >= monthStart {
				a, _ := excelize.CoordinatesToCellName(monthStart, 4)
				b, _ := excelize.CoordinatesToCellName(col-1, 4)
				_ = f.MergeCell(sheetName, a, b)
			}
			monthStart = col
			monthKey = key
			cell, _ := excelize.CoordinatesToCellName(col, 4)
			if err := f.SetCellStr(sheetName, cell, fmt.Sprintf("%d月", d.Month())); err != nil {
				return err
			}
		}
		dateCell, _ := excelize.CoordinatesToCellName(col, 5)
		if err := f.SetCellFloat(sheetName, dateCell, excelSerial(d), 0, 64); err != nil {
			return err
		}
		weekCell, _ := excelize.CoordinatesToCellName(col, 6)
		if err := f.SetCellStr(sheetName, weekCell, week[d.Weekday()]); err != nil {
			return err
		}
	}
	if monthKey != "" {
		a, _ := excelize.CoordinatesToCellName(monthStart, 4)
		b, _ := excelize.CoordinatesToCellName(calendarStart+days-1, 4)
		_ = f.MergeCell(sheetName, a, b)
	}

	// 今天：只高亮日期表头，避免盖住色条。
	todayStyle, err := f.NewConditionalStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"E53935"}, Pattern: 1},
		Font: &excelize.Font{Color: "FFFFFF", Bold: true, Size: 8},
	})
	if err != nil {
		return err
	}
	return f.SetConditionalFormat(sheetName, fmt.Sprintf("H5:%s5", lastCol), []excelize.ConditionalFormatOptions{
		{Type: "formula", Criteria: "H5=TODAY()", Format: &todayStyle, StopIfTrue: true},
	})
}

func writeRows(f *excelize.File, rows []Row, startRow int) error {
	for i, row := range rows {
		r := startRow + i
		name := strings.TrimSpace(row.Name)
		status := strings.TrimSpace(row.Status)
		if row.Kind == "group" {
			status = ""
		}
		if err := f.SetCellStr(sheetName, cell(1, r), name); err != nil {
			return err
		}
		if err := f.SetCellStr(sheetName, cell(2, r), status); err != nil {
			return err
		}
		if err := f.SetCellStr(sheetName, cell(3, r), strings.TrimSpace(row.Owner)); err != nil {
			return err
		}
		if p := strings.TrimSpace(strings.TrimSuffix(row.Progress, "%")); p != "" {
			if err := f.SetCellStr(sheetName, cell(4, r), p+"%"); err != nil {
				return err
			}
		}
		if d, ok := parseDate(row.Start); ok {
			if err := f.SetCellFloat(sheetName, cell(5, r), excelSerial(d), 0, 64); err != nil {
				return err
			}
		}
		if d, ok := parseDate(row.End); ok {
			if err := f.SetCellFloat(sheetName, cell(6, r), excelSerial(d), 0, 64); err != nil {
				return err
			}
		}
		formula := fmt.Sprintf(`IF(OR(E%d="",F%d=""),"",F%d-E%d+1)`, r, r, r, r)
		if err := f.SetCellFormula(sheetName, cell(7, r), formula); err != nil {
			return err
		}
	}
	return nil
}

func writeConditionalFormats(f *excelize.File, lastCol string, lastDataRow int) error {
	ref := fmt.Sprintf("H%d:%s%d", firstDataRow, lastCol, lastDataRow)
	// 公式按区域左上角 H7 书写，Excel 会按格下移。
	rules := []struct {
		formula string
		color   string
	}{
		{`AND(OR($B7="已完结",$B7="完成"),H$5>=$E7,H$5<=$F7)`, "70AD47"},
		{`AND(OR($B7="进行中",$B7="正常进行"),H$5>=$E7,H$5<=$F7)`, "29A3FF"},
		{`AND(OR($B7="逾期",$B7="严重推迟",$B7="推迟"),H$5>=$E7,H$5<=$F7)`, "FF4D4F"},
		{`AND(OR($B7="待启动",$B7="目标",$B7="小幅推迟"),H$5>=$E7,H$5<=$F7)`, "FDC76F"},
		{`AND($B7="",H$5>=$E7,H$5<=$F7)`, "5B9BD5"},
		{`AND($B7<>"",H$5>=$E7,H$5<=$F7)`, "5B9BD5"},
	}
	opts := make([]excelize.ConditionalFormatOptions, 0, len(rules))
	for _, rule := range rules {
		id, err := f.NewConditionalStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{rule.color}, Pattern: 1},
		})
		if err != nil {
			return err
		}
		opts = append(opts, excelize.ConditionalFormatOptions{
			Type:       "formula",
			Criteria:   rule.formula,
			Format:     &id,
			StopIfTrue: true,
		})
	}
	return f.SetConditionalFormat(sheetName, ref, opts)
}

func layout(f *excelize.File, lastCol string, lastDataRow int) error {
	titleStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "微软雅黑", Size: 16, Bold: true},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetName, "A1", "A1", titleStyle); err != nil {
		return err
	}
	noteStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "微软雅黑", Size: 9, Color: "546E7A"},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetName, "A2", "G2", noteStyle); err != nil {
		return err
	}
	if err := f.MergeCell(sheetName, "A2", "G2"); err != nil {
		return err
	}

	headStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "微软雅黑", Size: 9, Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"DCEEFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetName, "A4", lastCol+"6", headStyle); err != nil {
		return err
	}
	dayFmt, err := f.NewStyle(&excelize.Style{
		CustomNumFmt: strPtr("d"),
		Font:         &excelize.Font{Family: "微软雅黑", Size: 8},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"ECEFF1"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetName, "H5", lastCol+"5", dayFmt); err != nil {
		return err
	}
	dateFmt, err := f.NewStyle(&excelize.Style{
		CustomNumFmt: strPtr("yyyy/m/d"),
		Font:         &excelize.Font{Family: "微软雅黑", Size: 9},
		Alignment:    &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(sheetName, fmt.Sprintf("E%d", firstDataRow), fmt.Sprintf("F%d", lastDataRow), dateFmt); err != nil {
		return err
	}

	groupStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "微软雅黑", Size: 9, Bold: true, Color: "1A237E"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"B3D4FC"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	taskStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "微软雅黑", Size: 9},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}
	// 分组行在写入时无法从这里区分，统一任务样式；分组底色靠下面按空状态再刷。
	if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", firstDataRow), fmt.Sprintf("D%d", lastDataRow), taskStyle); err != nil {
		return err
	}
	for r := firstDataRow; r <= lastDataRow; r++ {
		status, _ := f.GetCellValue(sheetName, cell(2, r))
		if strings.TrimSpace(status) != "" {
			continue
		}
		if err := f.SetCellStyle(sheetName, cell(1, r), cell(7, r), groupStyle); err != nil {
			return err
		}
		// 分组行日期仍用日期格式
		if err := f.SetCellStyle(sheetName, cell(5, r), cell(6, r), dateFmt); err != nil {
			return err
		}
	}

	widths := []struct {
		a, b string
		w    float64
	}{
		{"A", "A", 28},
		{"B", "B", 10},
		{"C", "C", 14},
		{"D", "D", 8},
		{"E", "F", 12},
		{"G", "G", 8},
		{"H", lastCol, 3.2},
	}
	for _, w := range widths {
		if err := f.SetColWidth(sheetName, w.a, w.b, w.w); err != nil {
			return err
		}
	}
	_ = f.SetRowHeight(sheetName, 1, 22)
	_ = f.SetRowHeight(sheetName, 2, 28)

	showGrid := false
	zoom := 80.0
	if err := f.SetSheetView(sheetName, 0, &excelize.ViewOptions{
		ShowGridLines: &showGrid,
		ZoomScale:     &zoom,
	}); err != nil {
		return err
	}
	return f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		XSplit:      7,
		YSplit:      headerRows,
		TopLeftCell: "H7",
		ActivePane:  "bottomRight",
	})
}

func cell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

func strPtr(s string) *string { return &s }

// BytesBuffer 便于测试读取。
func BytesBuffer(b []byte) *bytes.Buffer { return bytes.NewBuffer(b) }
