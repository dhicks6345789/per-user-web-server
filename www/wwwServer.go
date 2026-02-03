package main

import (
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	//"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	rootPath := "/var/www" // The directory containing your static files and CGI scripts

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
	_, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		http.Error(w, "Could not determine file owner", 500)
		return
	}

	handler := &cgi.Handler{
		Path: path,
		Root: "/cgi-bin/", // Adjust based on your URL prefix
		Dir:  filepath.Dir(path),
		Env:  []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		// Note: Standard cgi.Handler does not support SysProcAttr directly.
		// To fix the "unknown field" error, we remove the Cmd field.
	}

	// Because cgi.Handler doesn't expose SysProcAttr, 
	// standard practice for UID switching involves a wrapper 
	// or using a modified version of the cgi package.
	handler.ServeHTTP(w, r)
}
