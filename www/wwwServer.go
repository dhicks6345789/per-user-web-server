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

func fileExists(thePath string) bool {
	_, pathErr := os.Stat(thePath)
	if os.IsNotExist(pathErr) {
		return false
	}
	return true
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestPath := filepath.Clean(r.URL.Path)
		fullPath := filepath.Join(rootPath, requestPath)
		
		if requestPath == "" || requestPath == "/" {
			fullPath = "/var/www/index.html"
		}

		// Check if the path exists - it might be a file or a folder.
		requestStatInfo, requestStatErr := os.Stat(fullPath)
		if os.IsNotExist(requestStatErr) {
			http.NotFound(w, r)
			return
		}
		
		if requestStatInfo.IsDir() {
			if fileExists(fullPath + "/" + "index.html") {
				fullPath = fullPath + "/" + "index.html"
			} else if fileExists(fullPath + "/" + "index.py") {
				fullPath = fullPath + "/" + "index.py"
			}
			requestStatInfo, _ = os.Stat(fullPath)
		}

		log.Print("wwwServer, request: " + requestPath + ", serving: " + fullPath)

		// Handle CGI scripts (assuming .cgi or .py extension).
		if !requestStatInfo.IsDir() && (filepath.Ext(fullPath) == ".cgi" || filepath.Ext(fullPath) == ".py") {
			handleCGI(w, r, fullPath, requestStatInfo)
			return
		}

		// Otherwise, serve as a static file.
		http.ServeFile(w, r, fullPath)
	})

	log.Println("wwwServer starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func handleCGI(w http.ResponseWriter, r *http.Request, path string, info os.FileInfo) {
	username := strings.Split(strings.TrimPrefix(path, rootPath+"/"), "/")[0]
	
	handler := &cgi.Handler{
		Path:   "/usr/bin/sudo",
		Args:   []string{"-u", username, path},
		Dir:    filepath.Dir(path),
		Env:    []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
	}
	
	var errBuf bytes.Buffer
		
	// Clone handler to ensure thread-safety per request
	handlerClone := *handler
	//handlerClone.Stderr = &errBuf
	handlerClone.Stderr = io.MultiWriter(&errBuf, os.Stderr)
	
	handlerClone.ServeHTTP(w, r)

	// After execution, check if we caught any errors
	if errBuf.Len() > 0 {
		w.Write([]byte("\n--- CGI Background Errors ---\n"))
		w.Write(errBuf.Bytes())
	}
}
