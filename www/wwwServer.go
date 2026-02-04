// A simple static content / basic CGI server - intended for use in a multi-user learning environment.
// This CGI server runs CGI scripts as the user who's folder they are in - i.e. a file in "/var/www/j.bloggs" will be ran as user "j.bloggs".

package main

import (
	"os"
	"io"
	"log"
	"bytes"
	"strings"
	"net/http"
	"net/http/cgi"
	"path/filepath"
)

// The root web server folder. Important: don't include include the trailing slash so the prefix gets removed properly from request path strings.
const rootPath = "/var/www"

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
		fullPath := filepath.Join(rootPath, requestPath)

		// If the user asks for the root path, we return the special index file with string substitutions.
		if requestPath == "" || requestPath == "/" {
			fullPath = "/var/www/index.html"
		}

		// Check if the requested path exists on the file system - it might be a file or a folder.
		requestStatInfo, requestStatErr := os.Stat(fullPath)
		if os.IsNotExist(requestStatErr) {
			http.NotFound(w, r)
			return
		}

		// If the user has requested a directory, serve any default "index" filesthat might be present there, in order of precident.
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
		Path:   "/usr/bin/sudo",
		Args:   []string{"-u", username, path},
		Dir:    filepath.Dir(path),
		Env:    []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		Stderr: io.MultiWriter(&errBuf, os.Stderr)
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
