// A small routing application designed to take traffic arriving at the "/rclone" endpoint and route it to the appropriate
// user's instance of the rclone GUI running on their desktop container instance. Gives the user a handy way of using the rclone GUI
// to set up new remotes and so on.

package main

import (
	"os"
	"io"
	"fmt"
	"log"
	"sync"
	"time"
	"strings"
	"net/url"
	"net/http"
	"net/http/httputil"
	"encoding/json"
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

	// To do: write this bit in Go. Call the session manager on the host to make sure there's a desktop instance running for the particular user.
	// Check the session manager is only accepting calls from this container (and the guacAutoConnect client) so users can't call it to create other users' sessions.
	APIURL := "http://host.docker.internal:8091/connectOrStartSession"
	
	// Define our form data to pass via POST to the sessionManager server, using url.Values.
	data := url.Values{}
	data.Set("username", key)
	data.Set("image", "desktop")

	// Encode the data into "bar=baz&foo=qux" format.
	encodedData := data.Encode()

	// Create a client with a timeout.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create the POST request using strings.NewReader
	req, err := http.NewRequest("POST", APIURL, strings.NewReader(encodedData))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return nil
	}

	// Set the correct Content-Type header.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute the request.
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	// Read the response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v\n", err)
		return nil
	}

	log.Printf("Status: %s\n", resp.Status)
	log.Printf("Response Body:\n%s\n", string(body))
	
	var genericData map[string]any
	json.NewDecoder(resp.Body).Decode(&genericData)
	
	// Access data by key (requires type assertion).
	password := genericData["password"].(string)
	log.Printf("Password: " + password)
	
	// Customize the proxy's director to handle headers correctly.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// Ensure the host header matches the target so Rclone doesn't reject it.
		req.Host = targetURL.Host

		// Optional: If rclone has basic auth enabled, inject it here 
		// so your users don't have to type it.
		req.SetBasicAuth(key, password)
		log.Printf("Basic auth: %s %s", key, password)
	}
	
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
		proxy.ServeHTTP(w, r)
	})
	
	http.Handle("/rclone/", http.StripPrefix("/rclone", rcloneHandler))
	
	// Execution starts here.
	log.Println("rcloneGUI starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
