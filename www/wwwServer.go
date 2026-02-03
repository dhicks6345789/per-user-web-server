package main

import (
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	rootPath := "/var/www" // The directory containing your static files and CGI scripts

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(rootPath, filepath.Clean(r.URL.Path))

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
	// Extract the UID of the file owner
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		http.Error(w, "Could not determine file owner", 500)
		return
	}
	uid := stat.Uid
	gid := stat.Gid

	handler := &cgi.Handler{
		Path: path,
		Dir:  filepath.Dir(path),
		// Use SysProcAttr to impersonate the file owner
		InheritEnv: []string{"PATH", "PYTHONPATH"},
		Cmd: exec.Cmd{
			SysProcAttr: &syscall.SysProcAttr{
				Credential: &syscall.Credential{Uid: uid, Gid: gid},
			},
		},
	}

	handler.ServeHTTP(w, r)
}
