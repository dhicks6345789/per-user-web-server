// A simple static content / basic CGI server - intended for use in a multi-user learning environment.
// This CGI server runs CGI scripts as the user who's folder they are in - i.e. a file in "/var/www/j.bloggs" will be ran as user "j.bloggs".

package main

import (
	"os"
	"io"
	"fmt"
	"log"
	"bytes"
	"strings"
	"strconv"
	"net/http"
	"net/http/cgi"
	"path/filepath"
)

// The root web server folder. Important: don't include include the trailing slash so the prefix gets removed properly from request path strings.
const rootPath = "/var/www"
// The Javascript cache folder. Used to hold local copies of various Javascript libraries that we can then serve locally .
const JSCachePath = "/var/cache/wwwServer/js"

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

		// Serve files from the "/var/www" folder, where the individual user files are...
		fullPath := filepath.Join(rootPath, requestPath)
		// ...except for the "/js" endpoint, which we serve from our JS cache folder.
		if strings.HasPrefix(requestPath, "js/") {
			fullPath = filepath.Join(JSCachePath, requestPath)
		}
		
		// If the user asks for the root path, we return the special index file with string substitutions.
		if requestPath == "" || requestPath == "/" {
			fullPath = "/var/www/index.html"
		}

		// We want to exlude some special files from being served so the user can place them in their "www" folder but not have to worrry about hiding them.
		if strings.HasSuffix(requestPath, "rclone.conf") {
			http.Error(w, "Forbidden: You do not have permission to access this resource", http.StatusForbidden)
			log.Print("wwwServer, request: " + requestPath + " - file is in special excluded list.")
			return
		}

		// Check if the requested path exists on the file system - it might be a file or a folder.
		requestStatInfo, requestStatErr := os.Stat(fullPath)
		if os.IsNotExist(requestStatErr) {
			http.NotFound(w, r)
			log.Print("wwwServer, request: " + requestPath + ", not found: " + fullPath)
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

	// Execution starts here. First, make sure our local cache folder to serve various JavaScript libraries is set up.
	if err := setupJSCacheDir(); err != nil {
		log.Fatal(err)
	}
	
	log.Println("wwwServer starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// Make sure the local JS cache dir exists and populate it with the Javascript libraries we want to serve locally.
func setupJSCacheDir() error {
	// 1. Check if folder exists, if not, create it
	// os.ModePerm gives standard 0777 permissions (modified by umask)
	if _, err := os.Stat(JSCachePath); os.IsNotExist(err) {
		fmt.Printf("Directory %s does not exist. Creating it...\n", JSCachePath)
		err := os.MkdirAll(JSCachePath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("error checking directory: %w", err)
	}

	// 2. Define the files to download (Official production builds via Unpkg CDN)
	filesToDownload := map[string]string{
		"react.production.min.js":     "https://unpkg.com/react@18/umd/react.production.min.js",
		"react-dom.production.min.js": "https://unpkg.com/react-dom@18/umd/react-dom.production.min.js",
	}

	// 3. Loop through and download each file if it doesn't already exist
	for fileName, url := range filesToDownload {
		filePath := filepath.Join(JSCachePath, fileName)

		// Skip downloading if the file is already there.
		if _, err := os.Stat(filePath); err == nil {
			log.Println("File " + filePath + " already exists. Skipping download.")
			continue
		}
		
		err := downloadFile(filePath, url)
		if err != nil {
			return fmt.Errorf("Failed to download %s: %w", fileName, err)
		}
	}

	return nil
}

// Fetches a URL and writes it directly to the specified local path.
func downloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the local file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
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
		Args:   []string{"--preserve-env", "-u", username, path},
		Dir:    filepath.Dir(path),
		Env:    []string{
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
