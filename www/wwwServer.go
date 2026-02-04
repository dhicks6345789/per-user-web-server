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
			fullPath = "/root/docs-to-markdown/startScreen/startScreenIndex.html"
		}
		
		if strings.HasSuffix(fullPath, "/") {
			for _, value := range []string{"index.py", "index.html"} {
				_, err := os.Stat(fullPath + value)
				if !os.IsNotExist(err) {
					fullPath = fullPath + value
				}
			}
		}

		log.Print("wwwServer, request: " + requestPath + ", serving: " + fullPath)

		// Check if the file exists
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// Handle CGI scripts (assuming .cgi or .py extension).
		if !info.IsDir() && (filepath.Ext(fullPath) == ".cgi" || filepath.Ext(fullPath) == ".py") {
			handleCGI(w, r, fullPath, info)
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
