// A simple static content / basic CGI server - intended for use in a multi-user learning environment.
// This CGI server runs CGI scripts as the user who's folder they are in - i.e. a file in "/var/www/j.bloggs" will be ran as user "j.bloggs".

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/cgi"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html"
)

// The root web server folder. Important: don't include include the trailing slash so the prefix gets removed properly from request path strings.
const rootPath = "/var/www"

// The folder where the Start Screen source data (the spreadsheet and any local icon images) lives.
const startScreenDataPath = "/etc/puws/startScreen"

// A function to get an icon for a URL, saving it into the data folder and returning its filename (or "" on failure).
// It is a variable so it can be replaced with a stub in tests.
var getIconForURL = getIconForURLDefault

// A function to return a simple boolean "true" if a file exists, false otherwise.
func fileExists(thePath string) bool {
	_, pathErr := os.Stat(thePath)
	if os.IsNotExist(pathErr) {
		return false
	}
	return true
}

func main() {
	// Handle all HTTP request URLs.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestPath := filepath.Clean(r.URL.Path)

		// Handle the "/startScreen" endpoint. It returns the Start Screen resources (the same data the Start Screen
		// template expects) as JSON, built from the spreadsheet in the data folder, with any icons served from
		// "/startScreen/<imageName>".
		if requestPath == "/startScreen" || strings.HasPrefix(requestPath, "/startScreen/") {
			handleStartScreen(w, r, requestPath, startScreenDataPath)
			return
		}

		// Serve files from the "/var/www" folder, where the individual user files are.
		fullPath := filepath.Join(rootPath, requestPath)

		// If the user asks for the root path, we return the special index file with string substitutions.
		if requestPath == "" || requestPath == "/" {
			fullPath = "/var/www/index.html"
		}

		// We want to exclude some special files from being served so the user can place them in their "www" folder but not have to worrry about hiding them.
		if strings.HasSuffix(requestPath, "rclone.conf") {
			http.Error(w, "Forbidden: You do not have permission to access this resource", http.StatusForbidden)
			log.Print("wwwServer, request: " + requestPath + " - file is in special excluded list.")
			return
		}

		// Check if the requested path exists on the file system - it might be a file or a folder.
		requestStatInfo, requestStatErr := os.Stat(fullPath)

		// If the requested path doesn't exist, return an error.
		if os.IsNotExist(requestStatErr) {
			http.NotFound(w, r)
			log.Print("wwwServer, request: " + requestPath + ", not found: " + fullPath)
			return
		}

		// If it's a directory and the (original, not cleaned) path is missing a trailing slash, redirect - this is the standard way to handle directory requests, it causes browsers confusion with relative paths if not handled as standard.
		if requestStatErr == nil && requestStatInfo.IsDir() && !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}

		// If the user has requested a directory, serve any default "index" files that might be present there, in order of precident.
		if requestStatInfo.IsDir() {
			if fileExists(fullPath + "/" + "index.html") {
				fullPath = fullPath + "/" + "index.html"
			} else if fileExists(fullPath + "/" + "index.py") {
				fullPath = fullPath + "/" + "index.py"
			}
			requestStatInfo, _ = os.Stat(fullPath)
		}

		// A message for the user / logs.
		log.Print("wwwServer, request: " + requestPath + ", serving: " + fullPath)

		// Handle CGI scripts (assuming .cgi or .py extension).
		if !requestStatInfo.IsDir() && (filepath.Ext(fullPath) == ".cgi" || filepath.Ext(fullPath) == ".py") {
			handleCGI(w, r, fullPath, requestStatInfo)
			return
		}

		// Otherwise, serve as a static file.
		http.ServeFile(w, r, fullPath)
	})

	// Execution starts here.
	log.Println("wwwServer starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// Handles the "/startScreen" endpoint. Returns the Start Screen resources as JSON, or serves a named icon image.
func handleStartScreen(w http.ResponseWriter, r *http.Request, requestPath, dataPath string) {
	// A request for "/startScreen/<imageName>" serves the named image file from the data folder.
	if requestPath != "/startScreen" {
		imageName := strings.TrimPrefix(requestPath, "/startScreen/")
		imagePath := filepath.Join(dataPath, filepath.Clean(imageName))
		// Guard against path traversal out of the data folder.
		if !strings.HasPrefix(filepath.Clean(imagePath), dataPath) || !fileExists(imagePath) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, imagePath)
		return
	}

	// Otherwise, return the resources as JSON.
	startScreenJSON, err := loadStartScreenJSON(dataPath)
	if err != nil {
		log.Print("wwwServer, /startScreen: unable to load Start Screen data: " + err.Error())
		http.Error(w, "Unable to load Start Screen data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(startScreenJSON)
}

// Reads the Start Screen spreadsheet in the data folder and returns a JSON representation of the resources, in the
// same format expected by the Start Screen template: a list of sections, each being a [name, table] pair, where
// "table" is a list of rows with columns URL,Title,Description,Icon.
func loadStartScreenJSON(dataPath string) ([]byte, error) {
	spreadsheetPath := filepath.Join(dataPath, "startScreen.xlsx")
	spreadsheet, err := excelize.OpenFile(spreadsheetPath)
	if err != nil {
		return nil, err
	}
	defer spreadsheet.Close()

	// Build the resources list, one section per spreadsheet sheet, one resource row per data row.
	resources := [][]interface{}{}
	for _, sheetName := range spreadsheet.GetSheetList() {
		rows, rowErr := spreadsheet.GetRows(sheetName)
		if rowErr != nil {
			return nil, rowErr
		}
		resourceTable := [][]string{}
		for _, row := range rows {
			// Pad short rows out to four columns (URL,Title,Description,Icon).
			resourceRow := []string{"", "", "", ""}
			for index, cell := range row {
				if index < 4 {
					resourceRow[index] = cell
				}
			}
			// Skip the header row (its first column is the literal "URL").
			if resourceRow[0] != "URL" && resourceRow[3] == "" {
				// No icon defined in the spreadsheet - try and get one from the URL's favicon.
				resourceRow[3] = getIconForURL(resourceRow[0], dataPath)
			}
			resourceTable = append(resourceTable, resourceRow)
		}
		resources = append(resources, []interface{}{sheetName, resourceTable})
	}

	return json.Marshal(resources)
}

// The default icon-getter: looks up the favicon for the given URL, saves it into the data folder (so it can be served
// from "/startScreen/<name>") and returns its filename. Returns an empty string if no favicon can be found.
func getIconForURLDefault(theURL, dataPath string) string {
	// If we've already fetched this site's favicon, re-use the saved copy.
	theHash := iconHash(theURL)
	existing := findExistingIcon(dataPath, theHash)
	if existing != "" {
		return existing
	}

	// Find and download the favicon for the URL.
	iconURL := findFaviconURL(theURL)
	if iconURL == "" {
		return ""
	}
	iconData, iconType := downloadIcon(iconURL)
	if iconData == nil {
		return ""
	}

	// Save it into the data folder under a filename derived from the URL hash, so it is served from "/startScreen/<name>".
	theExt := iconExtension(iconType)
	theName := theHash + theExt
	thePath := filepath.Join(dataPath, theName)
	if err := os.WriteFile(thePath, iconData, 0644); err != nil {
		log.Print("wwwServer, /startScreen: unable to save favicon: " + err.Error())
		return ""
	}
	return theName
}

// A simple, deterministic hash of a string, used to name downloaded favicons.
func iconHash(theString string) string {
	hasher := sha256.Sum256([]byte(theString))
	return hex.EncodeToString(hasher[:8])
}

// Looks in the data folder for an existing icon with the given hash prefix (any file extension).
func findExistingIcon(dataPath, theHash string) string {
	matches, _ := filepath.Glob(filepath.Join(dataPath, theHash+".*"))
	if len(matches) > 0 {
		return filepath.Base(matches[0])
	}
	return ""
}

// Finds the URL of a site's favicon. It first fetches the site's homepage and looks for a "link rel=icon" tag,
// falling back to the conventional "/favicon.ico" path if none is found.
func findFaviconURL(theURL string) string {
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(theURL)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return ""
	}

	// Parse the page for a "link rel=icon" element and return its (resolved) href, if present.
	parsed, _ := html.Parse(bytes.NewReader(body))
	baseURL, _ := url.Parse(theURL)
	for node := parsed.FirstChild; node != nil; node = node.NextSibling {
		if iconURL := findIconInNode(node, baseURL); iconURL != "" {
			return iconURL
		}
	}

	// Fall back to the conventional favicon location.
	fallback, _ := url.Parse("/favicon.ico")
	return baseURL.ResolveReference(fallback).String()
}

// Recursively searches an HTML node for a "link rel=icon" element, returning the resolved icon URL (or "").
func findIconInNode(node *html.Node, baseURL *url.URL) string {
	if node.Type == html.ElementNode && node.Data == "link" {
		var rel, href string
		for _, attribute := range node.Attr {
			switch attribute.Key {
			case "rel":
				rel = attribute.Val
			case "href":
				href = attribute.Val
			}
		}
		if href != "" && strings.Contains(strings.ToLower(rel), "icon") {
			iconURL, _ := url.Parse(href)
			return baseURL.ResolveReference(iconURL).String()
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if iconURL := findIconInNode(child, baseURL); iconURL != "" {
			return iconURL
		}
	}
	return ""
}

// Downloads the given URL and returns its bytes and content type, or nil on error.
func downloadIcon(theURL string) ([]byte, string) {
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(theURL)
	if err != nil {
		return nil, ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, ""
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 5*1024*1024))
	if err != nil {
		return nil, ""
	}
	return data, response.Header.Get("Content-Type")
}

// Returns a suitable file extension for a given MIME content type, defaulting to ".png".
func iconExtension(contentType string) string {
	switch {
	case strings.Contains(contentType, "x-icon"), strings.Contains(contentType, "vnd.microsoft.icon"):
		return ".ico"
	case strings.Contains(contentType, "svg"):
		return ".svg"
	case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
		return ".jpg"
	case strings.Contains(contentType, "gif"):
		return ".gif"
	case strings.Contains(contentType, "webp"):
		return ".webp"
	default:
		return ".png"
	}
}

// The function that handles CGI scripts.
func handleCGI(w http.ResponseWriter, r *http.Request, path string, info os.FileInfo) {
	// We extract the username to run the CGI script as.
	username := strings.Split(strings.TrimPrefix(path, rootPath+"/"), "/")[0]

	// To do: we might to disallow some usernames here - "root", probably.

	// We want to capture any error produced by the CGI script so we can display them to the user - this is a CGI server for learners.
	var errBuf bytes.Buffer

	// Set up the request's handler.
	handler := &cgi.Handler{
		// All scripts run under "sudo" so we can change the username they run as.
		Path: "/usr/bin/sudo",
		Args: []string{"--preserve-env", "-u", username, path},
		Dir:  filepath.Dir(path),
		Env: []string{
			"PATH=/usr/local/bin:/usr/bin:/bin",
			// We have to explicity add these headers back in so CGI scripts know how to process the request (GET or POST).
			"REQUEST_METHOD=" + r.Method,
			"CONTENT_TYPE=" + r.Header.Get("Content-Type"),
			"CONTENT_LENGTH=" + strconv.FormatInt(r.ContentLength, 10),
		},
		// We both capture any error output to display to the user and write it to stderr as normal so it appears in the logs.
		Stderr: io.MultiWriter(&errBuf, os.Stderr),
	}

	// Handle the request - hand over to Go's standard library.
	handler.ServeHTTP(w, r)

	// After execution, check if we caught any errors.
	if errBuf.Len() > 0 {
		// We format the error for the user - this is for learners, a nice obvious error message here is a good thing.
		w.Write([]byte("\n<div>\n"))
		w.Write([]byte("<pre style=\"background: #2d2d2d; color: #f8f8f2; padding: 15px; border-radius: 5px; overflow-x: auto; font-family: 'Courier New', monospace;\">\n"))
		w.Write(errBuf.Bytes())
		w.Write([]byte("\n</pre>\n"))
		w.Write([]byte("</div>\n"))
	}
}
