package main

import (
	"os"
	"log"
	"strings"
	"net/http"
	"net/http/cgi"
	"path/filepath"
)

// The root web server folder. Important: don't include include the trailing slash so the prefix gets removed properly from request path strings.
const rootPath = "/var/www"

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
			for _, value := range []string{"index.py", "index.html"} {
				_, defaultIndexErr := os.Stat(fullPath + "/" + value)
				if !os.IsNotExist(defaultIndexErr) {
					fullPath = fullPath + "/" + value
				}
			}
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
		Path: "/usr/bin/sudo",
		Args: []string{"-u", username, path, "2>&1"},
		Dir:  filepath.Dir(path),
		Env:  []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
	}
	handler.ServeHTTP(w, r)
}
