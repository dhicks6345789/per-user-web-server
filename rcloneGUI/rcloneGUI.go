// A small routing application designed to take traffic arriving at the "/rclone" endpoint and route it to the appropriate
// user's instance of the rclone GUI running on their desktop container instance. Gives the user a handy way of using the rclone GUI
// to set up new remotes and so on.

package main

import (
	"os"
	//"io"
	"fmt"
	"log"
	//"bytes"
	//"strings"
	"net/http"
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

		fmt.Fprint(w, "Hello, World!")

		// A message for the user / logs.
		log.Print("rcloneGUI, request: " + requestPath + ", serving: " + fullPath)
	})

	// Execution starts here.
	log.Println("rcloneGUI starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
