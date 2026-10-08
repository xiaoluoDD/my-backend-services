package ganttexport

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestBuildConditionalGantt(t *testing.T) {
	data, err := Build(Request{
		Title:   "测试项目",
		WorkNo:  "GW1",
		Manager: "张三",
		Rows: []Row{
			{Kind: "group", Name: "采购科", Owner: "凌玉迷", Start: "2026-04-01", End: "2026-04-10"},
			{Kind: "task", Name: "长周期采购", Status: "进行中", Owner: "凌玉迷", Progress: "40", Start: "2026-04-01", End: "2026-04-05"},
			{Kind: "task", Name: "已完成项", Status: "已完结", Owner: "凌玉迷", Progress: "100", Start: "2026-04-02", End: "2026-04-03"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(BytesBuffer(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	title, err := f.GetCellValue(sheetName, "A1")
	if err != nil || title != "GW1 测试项目" {
		t.Fatalf("title=%q err=%v", title, err)
	}
	status, _ := f.GetCellValue(sheetName, "B8")
	if status != "进行中" {
		t.Fatalf("status=%q", status)
	}
	groupStatus, _ := f.GetCellValue(sheetName, "B7")
	if groupStatus != "" {
		t.Fatalf("group status=%q", groupStatus)
	}
	formula, err := f.GetCellFormula(sheetName, "G8")
	if err != nil || formula == "" {
		t.Fatalf("days formula=%q err=%v", formula, err)
	}
	cfs, err := f.GetConditionalFormats(sheetName)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfs) < 2 {
		t.Fatalf("conditional formats=%d, want header today + bar rules", len(cfs))
	}
}
