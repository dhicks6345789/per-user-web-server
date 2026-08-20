package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Writes a small test spreadsheet (one sheet, two data rows, one blank-icon row) to the given folder.
func writeTestSpreadsheet(t *testing.T, theDir string) {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Main"
	f.SetSheetName("Sheet1", sheet)
	for col, val := range []string{"URL", "Title", "Description", "Icon"} {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, val)
	}
	rows := [][]string{
		{"https://example.com/one", "One", "First", "one.png"},
		{"https://example.com/two", "Two", "Second", ""},
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	if err := f.SaveAs(filepath.Join(theDir, "startScreen.xlsx")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStartScreenJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestSpreadsheet(t, dir)

	data, err := loadStartScreenJSON(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resources [][]interface{}
	if err := json.Unmarshal(data, &resources); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 section, got %d", len(resources))
	}
	if resources[0][0] != "Main" {
		t.Fatalf("expected section name 'Main', got %v", resources[0][0])
	}

	table, ok := resources[0][1].([]interface{})
	if !ok {
		t.Fatalf("expected table to be a list, got %T", resources[0][1])
	}
	if len(table) != 3 {
		t.Fatalf("expected header + 2 data rows, got %d", len(table))
	}

	// The first data row should have all four columns filled.
	firstRow, ok := table[1].([]interface{})
	if !ok || len(firstRow) != 4 {
		t.Fatalf("expected a 4-column row, got %#v", table[1])
	}
	// A blank icon should be padded to an empty string.
	secondRow, ok := table[2].([]interface{})
	if !ok || secondRow[3] != "" {
		t.Fatalf("expected blank icon padded to empty string, got %#v", secondRow)
	}
}

func TestHandleStartScreen(t *testing.T) {
	dir := t.TempDir()
	writeTestSpreadsheet(t, dir)
	// Put a fake icon in the folder so the image-serving path can be exercised.
	iconPath := filepath.Join(dir, "one.png")
	if err := os.WriteFile(iconPath, []byte("fake-icon"), 0644); err != nil {
		t.Fatal(err)
	}

	// The JSON endpoint.
	jsonReq := httptest.NewRequest(http.MethodGet, "/startScreen", nil)
	jsonRec := httptest.NewRecorder()
	handleStartScreen(jsonRec, jsonReq, "/startScreen", dir)
	if jsonRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /startScreen, got %d", jsonRec.Code)
	}
	if !strings.Contains(jsonRec.Body.String(), "https://example.com/one") {
		t.Fatalf("expected resource data in /startScreen response")
	}

	// The image endpoint.
	iconReq := httptest.NewRequest(http.MethodGet, "/startScreen/one.png", nil)
	iconRec := httptest.NewRecorder()
	handleStartScreen(iconRec, iconReq, "/startScreen/one.png", dir)
	if iconRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /startScreen/one.png, got %d", iconRec.Code)
	}
	if iconRec.Body.String() != "fake-icon" {
		t.Fatalf("expected icon body, got %q", iconRec.Body.String())
	}

	// A missing image should 404.
	missingReq := httptest.NewRequest(http.MethodGet, "/startScreen/nope.png", nil)
	missingRec := httptest.NewRecorder()
	handleStartScreen(missingRec, missingReq, "/startScreen/nope.png", dir)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing image, got %d", missingRec.Code)
	}
}
