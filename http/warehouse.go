package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
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
