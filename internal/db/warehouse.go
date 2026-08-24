package db

import (
	"database/sql"
	"fmt"
	"time"
)

// WarehousePurchaseOrder 对应桌面端 purchase_orders。
type WarehousePurchaseOrder struct {
	ID             int64  `json:"id"`
	OrderNumber    string `json:"order_number"`
	Department     string `json:"department"`
	Applicant      string `json:"applicant"`
	ProjectNumber  string `json:"project_number"`
	ProductCode    string `json:"product_code"`
	ProductName    string `json:"product_name"`
	Specification  string `json:"specification"`
	Manufacturer   string `json:"manufacturer"`
	Quantity       int    `json:"quantity"`
	StockedQuantity int   `json:"stocked_quantity"`
	Unit           string `json:"unit"`
	StockInDate    string `json:"stock_in_date"`
	Location       string `json:"location"`
	Barcode        string `json:"barcode"`
	DailyNumber    int    `json:"daily_number"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// WarehouseStockIn 对应 purchase_order_stock_in。
type WarehouseStockIn struct {
	ID             int64  `json:"id"`
	OrderNumber    string `json:"order_number"`
	Department     string `json:"department"`
	Applicant      string `json:"applicant"`
	ProjectNumber  string `json:"project_number"`
	ProductCode    string `json:"product_code"`
	ProductName    string `json:"product_name"`
	Specification  string `json:"specification"`
	Manufacturer   string `json:"manufacturer"`
	Quantity       int    `json:"quantity"`
	StockedQuantity int   `json:"stocked_quantity"`
	Unit           string `json:"unit"`
	StockInDate    string `json:"stock_in_date"`
	Location       string `json:"location"`
	Barcode        string `json:"barcode"`
	DailyNumber    int    `json:"daily_number"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// WarehouseStockInHistory 对应 stock_in_history。
type WarehouseStockInHistory struct {
	ID            int64  `json:"id"`
	OrderNumber   string `json:"order_number"`
	Department    string `json:"department"`
	Applicant     string `json:"applicant"`
	ProjectNumber string `json:"project_number"`
	ProductCode   string `json:"product_code"`
	ProductName   string `json:"product_name"`
	Specification string `json:"specification"`
	Manufacturer  string `json:"manufacturer"`
	Unit          string `json:"unit"`
	Quantity      int    `json:"quantity"`
	Location      string `json:"location"`
	Barcode       string `json:"barcode"`
	DailyNumber   int    `json:"daily_number"`
	StockInDate   string `json:"stock_in_date"`
	CreatedAt     string `json:"created_at"`
	OperatorName  string `json:"operator_name"`
	Notes         string `json:"notes"`
}

func createWarehouseTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS purchase_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_number TEXT NOT NULL DEFAULT '',
			department TEXT NOT NULL DEFAULT '',
			applicant TEXT NOT NULL DEFAULT '',
			project_number TEXT NOT NULL DEFAULT '',
			product_code TEXT NOT NULL DEFAULT '',
			product_name TEXT NOT NULL DEFAULT '',
			specification TEXT NOT NULL DEFAULT '',
			manufacturer TEXT NOT NULL DEFAULT '',
			quantity INTEGER NOT NULL DEFAULT 0,
			stocked_quantity INTEGER NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '个',
			stock_in_date TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			barcode TEXT NOT NULL UNIQUE DEFAULT '',
			daily_number INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_orders_barcode ON purchase_orders(barcode)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_orders_order_number ON purchase_orders(order_number)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_orders_project_number ON purchase_orders(project_number)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_orders_daily_number ON purchase_orders(daily_number)`,
		`CREATE TABLE IF NOT EXISTS purchase_order_stock_in (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_number TEXT NOT NULL DEFAULT '',
			department TEXT NOT NULL DEFAULT '',
			applicant TEXT NOT NULL DEFAULT '',
			project_number TEXT NOT NULL DEFAULT '',
			product_code TEXT NOT NULL DEFAULT '',
			product_name TEXT NOT NULL DEFAULT '',
			specification TEXT NOT NULL DEFAULT '',
			manufacturer TEXT NOT NULL DEFAULT '',
			quantity INTEGER NOT NULL DEFAULT 0,
			stocked_quantity INTEGER NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '个',
			stock_in_date TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			barcode TEXT NOT NULL UNIQUE DEFAULT '',
			daily_number INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_order_stock_in_barcode ON purchase_order_stock_in(barcode)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_order_stock_in_date ON purchase_order_stock_in(stock_in_date)`,
		`CREATE TABLE IF NOT EXISTS stock_in_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_number TEXT NOT NULL DEFAULT '',
			department TEXT NOT NULL DEFAULT '',
			applicant TEXT NOT NULL DEFAULT '',
			project_number TEXT NOT NULL DEFAULT '',
			product_code TEXT NOT NULL DEFAULT '',
			product_name TEXT NOT NULL DEFAULT '',
			specification TEXT NOT NULL DEFAULT '',
			manufacturer TEXT NOT NULL DEFAULT '',
			unit TEXT NOT NULL DEFAULT '个',
			quantity INTEGER NOT NULL DEFAULT 0,
			location TEXT NOT NULL DEFAULT '',
			barcode TEXT NOT NULL DEFAULT '',
			daily_number INTEGER NOT NULL DEFAULT 0,
			stock_in_date TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			operator_name TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_in_history_barcode ON stock_in_history(barcode)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_in_history_stock_in_date ON stock_in_history(stock_in_date)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("warehouse migrate: %w", err)
		}
	}
	return nil
}

func warehouseNow() string { return time.Now().Format("2006-01-02 15:04:05") }

func ListWarehousePurchaseOrders(db *sql.DB) ([]WarehousePurchaseOrder, error) {
	rows, err := db.Query(`SELECT id, order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, quantity, stocked_quantity, unit, stock_in_date, location, barcode, daily_number, created_at, updated_at FROM purchase_orders ORDER BY created_at DESC, id DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var list []WarehousePurchaseOrder
	for rows.Next() {
		var it WarehousePurchaseOrder
		if err := rows.Scan(&it.ID, &it.OrderNumber, &it.Department, &it.Applicant, &it.ProjectNumber, &it.ProductCode, &it.ProductName, &it.Specification, &it.Manufacturer, &it.Quantity, &it.StockedQuantity, &it.Unit, &it.StockInDate, &it.Location, &it.Barcode, &it.DailyNumber, &it.CreatedAt, &it.UpdatedAt); err != nil { return nil, err }
		list = append(list, it)
	}
	return list, rows.Err()
}

func UpsertWarehousePurchaseOrder(db *sql.DB, it WarehousePurchaseOrder) (int64, error) {
	now := warehouseNow()
	if it.Unit == "" { it.Unit = "个" }
	if it.StockInDate == "" { it.StockInDate = now }
	res, err := db.Exec(`INSERT INTO purchase_orders (order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, quantity, stocked_quantity, unit, stock_in_date, location, barcode, daily_number, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(barcode) DO UPDATE SET order_number=excluded.order_number, department=excluded.department, applicant=excluded.applicant, project_number=excluded.project_number, product_code=excluded.product_code, product_name=excluded.product_name, specification=excluded.specification, manufacturer=excluded.manufacturer, quantity=excluded.quantity, stocked_quantity=excluded.stocked_quantity, unit=excluded.unit, stock_in_date=excluded.stock_in_date, location=excluded.location, daily_number=excluded.daily_number, updated_at=excluded.updated_at`, it.OrderNumber, it.Department, it.Applicant, it.ProjectNumber, it.ProductCode, it.ProductName, it.Specification, it.Manufacturer, it.Quantity, it.StockedQuantity, it.Unit, it.StockInDate, it.Location, it.Barcode, it.DailyNumber, now, now)
	if err != nil { return 0, err }
	return res.LastInsertId()
}

func ReplaceWarehousePurchaseOrders(db *sql.DB, items []WarehousePurchaseOrder) (int, error) {
	tx, err := db.Begin()
	if err != nil { return 0, err }
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM purchase_orders`); err != nil { return 0, err }
	stmt, err := tx.Prepare(`INSERT INTO purchase_orders (order_number, department, applicant, project_number, product_code, product_name, specification, manufacturer, quantity, stocked_quantity, unit, stock_in_date, location, barcode, daily_number, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil { return 0, err }
	defer stmt.Close()
	for i, it := range items {
		now := warehouseNow()
		if it.Unit == "" { it.Unit = "个" }
		if it.StockInDate == "" { it.StockInDate = now }
		if it.CreatedAt == "" { it.CreatedAt = now }
		if it.UpdatedAt == "" { it.UpdatedAt = now }
		if _, err := stmt.Exec(it.OrderNumber, it.Department, it.Applicant, it.ProjectNumber, it.ProductCode, it.ProductName, it.Specification, it.Manufacturer, it.Quantity, it.StockedQuantity, it.Unit, it.StockInDate, it.Location, it.Barcode, it.DailyNumber, it.CreatedAt, it.UpdatedAt); err != nil { return i, err }
	}
	if err := tx.Commit(); err != nil { return 0, err }
	return len(items), nil
}
