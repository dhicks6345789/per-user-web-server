// A small routing application designed to take traffic arriving at the "/rclone" endpoint and route it to the appropriate
// user's instance of the rclone GUI running on their desktop container instance. Gives the user a handy way of using the rclone GUI
// to set up new remotes and so on.

package main

import (
	"os"
	"fmt"
	"log"
	"sync"
	"strings"
	"net/url"
	"net/http"
	"net/http/httputil"
)

// The root web server folder. Important: don't include include the trailing slash so the prefix gets removed properly from request path strings.
const rootPath = "/var/www"



/* We need a separate proxy object for each rclone instance running inisde a user's container. Standard Go maps are not safe for concurrent use,
   therfore we protect our global dictionary using a sync.RWMutex to prevent race conditions when multiple incoming HTTP requests try to read
   from or write to the dictionary simultaneously. */

// ProxyRegistry manages our global dictionary of reverse proxies safely.
type ProxyRegistry struct {
	mu sync.RWMutex
	proxies map[string]*httputil.ReverseProxy
}

// NewProxyRegistry initializes the registry
func newProxyRegistry() *ProxyRegistry {
	return &ProxyRegistry{
		proxies: make(map[string]*httputil.ReverseProxy),
	}
}

// Get looks up a proxy by its target key.
func (pr *ProxyRegistry) get(key string) (*httputil.ReverseProxy, bool) {
	pr.mu.RLock() // Allow multiple readers simultaneously.
	defer pr.mu.RUnlock()
	
	proxy, exists := pr.proxies[key]
	return proxy, exists
}

// Set adds or updates a proxy in the global dictionary.
func (pr *ProxyRegistry) set(key string, targetURLStr string) error {
	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		return fmt.Errorf("invalid target URL %s: %w", targetURLStr, err)
	}

	// Create the reverse proxy instance
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	pr.mu.Lock() // Block readers and other writers.
	defer pr.mu.Unlock()
	
	pr.proxies[key] = proxy
	return nil
}

// Global instance
var rcloneProxies = newProxyRegistry()



// A function to return a simple boolean "true" if a file exists, false otherwise.
func fileExists(thePath string) bool {
	_, pathErr := os.Stat(thePath)
	if os.IsNotExist(pathErr) {
		return false
	}
	return true
}

func main() {
	rcloneHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the username ("Remote-User" HTTP header value injected by Pangolin).
		username := strings.Split(r.Header.Get("Remote-User"), "@")[0]

		// Make sure a proxy object to the user's Desktop Docker container (which is where rclone will be running) exists.
		proxy, exists := rcloneProxies.get(username)
		if exists == false {
			rcloneProxies.set(username, "http://desktop-" + username + ":8090")
			proxy, exists = rcloneProxies.get(username)
		}

		// Rewrite the URL to remove the "/rclone" prefix.
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/rclone")
		
		log.Printf("Proxying request: %s %s", r.Method, r.URL.Path)
		log.Printf("Headers: %v", r.Header)
		proxy.ServeHTTP(w, r)
	})
	
	http.Handle("/rclone/", http.StripPrefix("/rclone", rcloneHandler))
	
	// Execution starts here.
	log.Println("rcloneGUI starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
