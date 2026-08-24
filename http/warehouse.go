package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoluoDD/my-backend-services/internal/db"
)

type warehouseImportRequest struct {
	Items []db.WarehousePurchaseOrder `json:"items"`
}

type warehouseStockInItem struct {
	Barcode string `json:"barcode"`
	Quantity int    `json:"quantity"`
	Location string `json:"location"`
}

type warehouseStockInRequest struct {
	Items        []warehouseStockInItem `json:"items"`
	OperatorName string                 `json:"operator_name"`
	Notes        string                 `json:"notes"`
}

func handleWarehouse(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/warehouse")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "不支持的请求"})
		return
	}

	switch parts[0] {
	case "purchase-orders":
		handleWarehousePurchaseOrders(w, r, parts[1:])
	case "stock-in-history":
		handleWarehouseStockInHistory(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "未找到仓库接口"})
	}
}

func handleWarehousePurchaseOrders(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := db.ListWarehousePurchaseOrders(sqlDB)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": items})
		case http.MethodPost:
			var req warehouseImportRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON 格式不正确"})
				return
			}
			count, err := db.ReplaceWarehousePurchaseOrders(sqlDB, req.Items)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": count})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "请使用 GET 或 POST"})
		}
		return
	}

	if len(rest) == 1 && rest[0] == "import-file" && r.Method == http.MethodPost {
		items, err := parseWarehouseUpload(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		count, err := db.ReplaceWarehousePurchaseOrders(sqlDB, items)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": count})
		return
	}

	if len(rest) == 1 && rest[0] == "stock-in" && r.Method == http.MethodPost {
		var req warehouseStockInRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON 格式不正确"})
			return
		}
		result, err := warehouseDoStockIn(sqlDB, req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
		return
	}

	if len(rest) == 1 && rest[0] == "import" && r.Method == http.MethodPost {
		var req warehouseImportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON 格式不正确"})
			return
		}
		count, err := db.ReplaceWarehousePurchaseOrders(sqlDB, req.Items)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": count})
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "未找到接口"})
}

func parseWarehouseUpload(r *http.Request) ([]db.WarehousePurchaseOrder, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, fmt.Errorf("解析上传文件失败: %w", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("请上传文件")
	}
	defer file.Close()

	name := strings.ToLower(header.Filename)
	switch filepath.Ext(name) {
	case ".xlsx":
		return parseWarehouseXLSX(file)
	case ".json":
		return parseWarehouseJSON(file)
	case ".csv":
		return parseWarehouseCSV(file)
	default:
		return nil, fmt.Errorf("仅支持 .xlsx / .csv / .json")
	}
}

func parseWarehouseJSON(r io.Reader) ([]db.WarehousePurchaseOrder, error) {
	var payload struct {
		Items []db.WarehousePurchaseOrder `json:"items"`
	}
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, err
	}
	return normalizeWarehouseItems(payload.Items), nil
}

func parseWarehouseCSV(r io.Reader) ([]db.WarehousePurchaseOrder, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return nil, nil
	}
	headers := splitWarehouseCSVLine(lines[0])
	keys := make([]string, 0, len(headers))
	for _, h := range headers {
		keys = append(keys, normalizeWarehouseHeader(strings.TrimSpace(h)))
	}
	var items []db.WarehousePurchaseOrder
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		cols := splitWarehouseCSVLine(line)
		row := map[string]string{}
		for idx, key := range keys {
			if idx < len(cols) {
				row[key] = cols[idx]
			}
		}
		items = append(items, normalizeWarehouseRow(row))
	}
	return normalizeWarehouseItems(items), nil
}

type xlsxSharedStrings struct {
	Si []struct {
		T string `xml:"t"`
	} `xml:"si"`
}

type xlsxSheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	C []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	R string `xml:"r,attr"`
	T string `xml:"t,attr"`
	V string `xml:"v"`
	Is struct {
		T string `xml:"t"`
	} `xml:"is"`
}

func parseWarehouseXLSX(r io.Reader) ([]db.WarehousePurchaseOrder, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytesNewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, err
	}

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}

	shared := []string{}
	if f, ok := files["xl/sharedStrings.xml"]; ok {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		var sst xlsxSharedStrings
		if err := xml.NewDecoder(rc).Decode(&sst); err != nil {
			_ = rc.Close()
			return nil, err
		}
		_ = rc.Close()
		for _, si := range sst.Si {
			shared = append(shared, si.T)
		}
	}

	sheetFile := files["xl/worksheets/sheet1.xml"]
	if sheetFile == nil {
		return nil, fmt.Errorf("未找到 sheet1")
	}
	rc, err := sheetFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var sheet xlsxSheet
	if err := xml.NewDecoder(rc).Decode(&sheet); err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, row := range sheet.Rows {
		m := map[string]string{}
		for _, cell := range row.C {
			col := excelCellColumn(cell.R)
			val := cell.V
			switch cell.T {
			case "s":
				if idx, err := strconv.Atoi(strings.TrimSpace(cell.V)); err == nil && idx >= 0 && idx < len(shared) {
					val = shared[idx]
				}
			case "inlineStr":
				val = cell.Is.T
			}
			m[col] = strings.TrimSpace(val)
		}
		rows = append(rows, m)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	headerRow := rows[0]
	keyByCol := map[string]string{}
	for col, val := range headerRow {
		keyByCol[col] = normalizeWarehouseHeader(val)
	}

	var items []db.WarehousePurchaseOrder
	for _, row := range rows[1:] {
		m := map[string]string{}
		for col, val := range row {
			key := keyByCol[col]
			if key != "" {
				m[key] = val
			}
		}
		items = append(items, normalizeWarehouseRow(m))
	}
	return normalizeWarehouseItems(items), nil
}

func bytesNewReader(b []byte) io.ReaderAt { return bytes.NewReader(b) }

func excelCellColumn(ref string) string {
	ref = strings.ToUpper(ref)
	for i := 0; i < len(ref); i++ {
		if ref[i] >= '0' && ref[i] <= '9' {
			return ref[:i]
		}
	}
	return ref
}

func normalizeWarehouseItems(items []db.WarehousePurchaseOrder) []db.WarehousePurchaseOrder {
	out := make([]db.WarehousePurchaseOrder, 0, len(items))
	for _, it := range items {
		n := normalizeWarehouseRow(map[string]string{
			"order_number":     it.OrderNumber,
			"department":       it.Department,
			"applicant":        it.Applicant,
			"project_number":   it.ProjectNumber,
			"product_code":     it.ProductCode,
			"product_name":     it.ProductName,
			"specification":    it.Specification,
			"manufacturer":     it.Manufacturer,
			"quantity":         strconv.Itoa(it.Quantity),
			"stocked_quantity":  strconv.Itoa(it.StockedQuantity),
			"unit":             it.Unit,
			"stock_in_date":    it.StockInDate,
			"location":         it.Location,
			"barcode":          it.Barcode,
			"daily_number":     strconv.Itoa(it.DailyNumber),
		})
		out = append(out, n)
	}
	return out
}

func normalizeWarehouseRow(row map[string]string) db.WarehousePurchaseOrder {
	qty, _ := strconv.Atoi(strings.TrimSpace(row["quantity"]))
	stocked, _ := strconv.Atoi(strings.TrimSpace(row["stocked_quantity"]))
	daily, _ := strconv.Atoi(strings.TrimSpace(row["daily_number"]))
	barcode := strings.TrimSpace(row["barcode"])
	if barcode == "" {
		barcode = warehouseNow()
	}
	unit := strings.TrimSpace(row["unit"])
	if unit == "" {
		unit = "个"
	}
	date := strings.TrimSpace(row["stock_in_date"])
	if date == "" {
		date = warehouseNow()
	}
	return db.WarehousePurchaseOrder{
		OrderNumber:    strings.TrimSpace(row["order_number"]),
		Department:     strings.TrimSpace(row["department"]),
		Applicant:      strings.TrimSpace(row["applicant"]),
		ProjectNumber:  strings.TrimSpace(row["project_number"]),
		ProductCode:    strings.TrimSpace(row["product_code"]),
		ProductName:    strings.TrimSpace(row["product_name"]),
		Specification:  strings.TrimSpace(row["specification"]),
		Manufacturer:   strings.TrimSpace(row["manufacturer"]),
		Quantity:       qty,
		StockedQuantity: stocked,
		Unit:           unit,
		StockInDate:    date,
		Location:       strings.TrimSpace(row["location"]),
		Barcode:        barcode,
		DailyNumber:    daily,
		CreatedAt:      warehouseNow(),
		UpdatedAt:      warehouseNow(),
	}
}

func normalizeWarehouseHeader(h string) string {
	switch strings.TrimSpace(h) {
	case "订单号":
		return "order_number"
	case "部门":
		return "department"
	case "申请人":
		return "applicant"
	case "项目管理号":
		return "project_number"
	case "品番":
		return "product_code"
	case "名称":
		return "product_name"
	case "规格型号":
		return "specification"
	case "厂家品牌":
		return "manufacturer"
	case "数量":
		return "quantity"
	case "入库数量":
		return "stocked_quantity"
	case "单位":
		return "unit"
	case "入库日期":
		return "stock_in_date"
	case "放置位置":
		return "location"
	case "条形码":
		return "barcode"
	case "当日编号":
		return "daily_number"
	default:
		return strings.TrimSpace(h)
	}
}

func splitWarehouseCSVLine(line string) []string {
	var out []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch ch {
		case '"':
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				cur.WriteByte('"')
				i++
			} else {
				inQuotes = !inQuotes
			}
		case ',':
			if inQuotes {
				cur.WriteByte(ch)
			} else {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	out = append(out, cur.String())
	return out
}

func warehouseDoStockIn(dbConn *sql.DB, req warehouseStockInRequest) (map[string]any, error) {
	items := req.Items
	if len(items) == 0 {
		return map[string]any{"success": 0, "failed": 0}, nil
	}

	tx, err := dbConn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	success := 0
	failed := 0
	for _, item := range items {
		barcode := strings.TrimSpace(item.Barcode)
		if barcode == "" || item.Quantity <= 0 {
			failed++
			continue
		}

		var po db.WarehousePurchaseOrder
		err := tx.QueryRow(`SELECT id, order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, quantity, stocked_quantity, unit, stock_in_date, location, barcode, daily_number, created_at, updated_at FROM purchase_orders WHERE barcode = ?`, barcode).Scan(
			&po.ID, &po.OrderNumber, &po.Department, &po.Applicant, &po.ProjectNumber, &po.ProductCode, &po.ProductName, &po.Specification, &po.Manufacturer, &po.Quantity, &po.StockedQuantity, &po.Unit, &po.StockInDate, &po.Location, &po.Barcode, &po.DailyNumber, &po.CreatedAt, &po.UpdatedAt,
		)
		if err != nil {
			failed++
			continue
		}

		loc := strings.TrimSpace(item.Location)
		if loc == "" {
			loc = po.Location
		}
		now := warehouseNow()
		if _, err := tx.Exec(`INSERT INTO purchase_order_stock_in (order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, quantity, stocked_quantity, unit, stock_in_date, location, barcode, daily_number, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(barcode) DO UPDATE SET quantity=quantity+excluded.quantity, stocked_quantity=stocked_quantity+excluded.stocked_quantity, location=excluded.location, stock_in_date=excluded.stock_in_date, updated_at=excluded.updated_at`,
			po.OrderNumber, po.Department, po.Applicant, po.ProjectNumber, po.ProductCode, po.ProductName, po.Specification, po.Manufacturer, item.Quantity, item.Quantity, po.Unit, now, loc, barcode, po.DailyNumber, now, now); err != nil {
			failed++
			continue
		}

		if _, err := tx.Exec(`INSERT INTO stock_in_history (order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, unit, quantity, location, barcode, daily_number, stock_in_date, created_at, operator_name, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			po.OrderNumber, po.Department, po.Applicant, po.ProjectNumber, po.ProductCode, po.ProductName, po.Specification, po.Manufacturer, po.Unit, item.Quantity, loc, barcode, po.DailyNumber, now, now, req.OperatorName, req.Notes); err != nil {
			failed++
			continue
		}

		if _, err := tx.Exec(`UPDATE purchase_orders SET stocked_quantity = stocked_quantity + ?, location = ?, stock_in_date = ?, updated_at = ? WHERE barcode = ?`, item.Quantity, loc, now, now, barcode); err != nil {
			failed++
			continue
		}

		if _, err := tx.Exec(`DELETE FROM purchase_orders WHERE barcode = ? AND stocked_quantity >= quantity`, barcode); err != nil {
			failed++
			continue
		}

		success++
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"success": success, "failed": failed}, nil
}

func handleWarehouseStockInHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "请使用 GET"})
		return
	}
	rows, err := sqlDB.Query(`SELECT id, order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, unit, quantity, location, barcode, daily_number, stock_in_date, created_at, operator_name, notes FROM stock_in_history ORDER BY id DESC LIMIT 200`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer rows.Close()
	var list []db.WarehouseStockInHistory
	for rows.Next() {
		var it db.WarehouseStockInHistory
		if err := rows.Scan(&it.ID, &it.OrderNumber, &it.Department, &it.Applicant, &it.ProjectNumber, &it.ProductCode, &it.ProductName, &it.Specification, &it.Manufacturer, &it.Unit, &it.Quantity, &it.Location, &it.Barcode, &it.DailyNumber, &it.StockInDate, &it.CreatedAt, &it.OperatorName, &it.Notes); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		list = append(list, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": list})
}

func warehouseNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
