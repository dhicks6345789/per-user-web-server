package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
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

// Returns the bytes of a tiny (16x16) red PNG, used to simulate a downloaded favicon.
func tinyPNGBytes() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// Asserts that the saved icon is a valid PNG upscaled to the standard 1024x1024 Start Screen tile size.
func assertNormalizedIcon(t *testing.T, thePath string) {
	t.Helper()
	file, err := os.Open(thePath)
	if err != nil {
		t.Fatalf("expected saved icon to open: %v", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("expected saved icon to be a decodable image: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1024 || bounds.Dy() != 1024 {
		t.Fatalf("expected icon upscaled to 1024x1024, got %dx%d", bounds.Dx(), bounds.Dy())
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
			w.Write(tinyPNGBytes())
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
	if !strings.HasSuffix(theName, ".png") {
		t.Fatalf("expected raster favicon to be saved as a PNG, got %q", theName)
	}
	savedPath := filepath.Join(dir, theName)
	if _, err := os.Stat(savedPath); err != nil {
		t.Fatalf("expected favicon to be saved to data folder: %v", err)
	}
	assertNormalizedIcon(t, savedPath)

	// A second call should re-use the already-saved copy (same filename, no network fetch).
	again := getIconForURLDefault(server.URL, dir)
	if again != theName {
		t.Fatalf("expected second call to reuse saved icon %q, got %q", theName, again)
	}
}

func TestGetIconForURLDefaultFallback(t *testing.T) {
	// A test server with no link rel=icon that serves a conventional favicon.ico (simulated with a PNG, as the Go
	// standard library can't decode ICO files).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>hi</title></head><body></body></html>`))
		case "/favicon.ico":
			w.Header().Set("Content-Type", "image/x-icon")
			w.Write(tinyPNGBytes())
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
	if !strings.HasSuffix(theName, ".png") {
		t.Fatalf("expected fallback raster favicon to be saved as a PNG, got %q", theName)
	}
	assertNormalizedIcon(t, filepath.Join(dir, theName))
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
