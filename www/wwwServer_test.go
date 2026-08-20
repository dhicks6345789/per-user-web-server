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

// Stubs the favicon lookup used when a resource has no icon defined, returning the given icon name.
func stubGetIcon(t *testing.T, theName string) {
	t.Helper()
	original := getIconForURL
	getIconForURL = func(theURL, dataPath string) string {
		return theName
	}
	t.Cleanup(func() {
		getIconForURL = original
	})
}

func TestLoadStartScreenJSON(t *testing.T) {
	stubGetIcon(t, "favicon-test.png")
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
	// A blank icon should trigger a favicon lookup (stubbed here to return a fixed icon name).
	secondRow, ok := table[2].([]interface{})
	if !ok || secondRow[3] != "favicon-test.png" {
		t.Fatalf("expected blank icon resolved via favicon lookup, got %#v", secondRow)
	}
}

func TestHandleStartScreen(t *testing.T) {
	stubGetIcon(t, "")
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

func TestGetIconForURLDefault(t *testing.T) {
	// A test server that serves a homepage with a "link rel=icon" pointing at a PNG.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><link rel="icon" href="/icons/myicon.png"></head><body></body></html>`))
		case "/icons/myicon.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("PNG-BYTES"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	theName := getIconForURLDefault(server.URL, dir)
	if theName == "" {
		t.Fatal("expected a favicon to be fetched and saved")
	}
	savedPath := filepath.Join(dir, theName)
	content, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("expected favicon to be saved to data folder: %v", err)
	}
	if string(content) != "PNG-BYTES" {
		t.Fatalf("expected saved favicon content, got %q", content)
	}

	// A second call should re-use the already-saved copy (same filename, no network fetch).
	again := getIconForURLDefault(server.URL, dir)
	if again != theName {
		t.Fatalf("expected second call to reuse saved icon %q, got %q", theName, again)
	}
}

func TestGetIconForURLDefaultFallback(t *testing.T) {
	// A test server with no link rel=icon that serves a conventional favicon.ico.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>hi</title></head><body></body></html>`))
		case "/favicon.ico":
			w.Header().Set("Content-Type", "image/x-icon")
			w.Write([]byte("ICO-BYTES"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	theName := getIconForURLDefault(server.URL, dir)
	if theName == "" {
		t.Fatal("expected fallback favicon.ico to be fetched and saved")
	}
	if !strings.HasSuffix(theName, ".ico") {
		t.Fatalf("expected .ico extension from fallback, got %q", theName)
	}
	content, err := os.ReadFile(filepath.Join(dir, theName))
	if err != nil {
		t.Fatalf("expected fallback favicon to be saved: %v", err)
	}
	if string(content) != "ICO-BYTES" {
		t.Fatalf("expected fallback favicon content, got %q", content)
	}
}

func TestGetIconForURLDefaultNoFavicon(t *testing.T) {
	// A server that serves a homepage but no favicon anywhere.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head></head><body></body></html>`))
	}))
	defer server.Close()

	dir := t.TempDir()
	if theName := getIconForURLDefault(server.URL, dir); theName != "" {
		t.Fatalf("expected empty icon when no favicon exists, got %q", theName)
	}
}
