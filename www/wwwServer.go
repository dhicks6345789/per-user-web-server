package main

import (
	"os"
	"log"
	"net/http"
	"net/http/cgi"
	"path/filepath"
)

func main() {
	rootPath := "/var/www/" // The directory containing your static files and CGI scripts

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(rootPath, filepath.Clean(r.URL.Path))
		log.Print("Serving: " + fullPath)

		// Check if the file exists
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// Handle CGI scripts (assuming .cgi or .py extension)
		if !info.IsDir() && (filepath.Ext(fullPath) == ".cgi" || filepath.Ext(fullPath) == ".py") {
			handleCGI(w, r, fullPath, info)
			return
		}

		// Otherwise, serve as a static file
		http.ServeFile(w, r, fullPath)
	})

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func handleCGI(w http.ResponseWriter, r *http.Request, path string, info os.FileInfo) {
	username := string.Split(strings.TrimPrefix(path, rootPath), "/")[0]
	
	handler := &cgi.Handler{
		Path: "/usr/bin/sudo",
		Args: []string{"-u", username, path},
		Root: "/cgi-bin/", // Adjust based on your URL prefix
		Dir:  filepath.Dir(path),
		Env:  []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
	}
	handler.ServeHTTP(w, r)
}
