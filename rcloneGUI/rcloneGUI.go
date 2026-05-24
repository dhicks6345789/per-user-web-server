// A small routing application designed to take traffic arriving at the "/rclone" endpoint and route it to the appropriate
// user's instance of the rclone GUI running on their desktop container instance. Gives the user a handy way of using the rclone GUI
// to set up new remotes and so on.

package main

import (
	"os"
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
	// Before we can create / update a new Proxy object, we need to call the Session Manager process on the host to get the password for the proxy to forward to rclone on the user desktop instance container.
	// So, we do an API call (via a HTTP POST request) the session manager on the host. That will make sure there's a desktop instance (with rclone) running for the particular user, and will return the password
	// to use to connect to it.
	// To do: Check the session manager is only accepting calls from this container (and the guacAutoConnect client) so users can't call it to create other users' sessions.
	// Define our form data to pass via POST to the sessionManager server, using url.Values...
	sessionManagerData := url.Values{}
	sessionManagerData.Set("username", key)
	sessionManagerData.Set("image", "desktop")
	// ...and encode that data into a string in "bar=baz&foo=qux" format.
	sessionManagerEncodedData := sessionManagerData.Encode()

	// Create a client to call the session manager, with a timeout.
	sessionManagerClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create the POST request using strings.NewReader.
	sessionManagerRequest, err := http.NewRequest("POST", "http://host.docker.internal:8091/connectOrStartSession", strings.NewReader(sessionManagerEncodedData))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return nil
	}

	// Set the correct Content-Type header.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute the (POST) request.
	sessionManagerResponse, err := sessionManagerClient.Do(sessionManagerRequest)
	if err != nil {
		log.Printf("Error sending request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	
	// The response should be a string in JSON format, {"port":"..", "password":"..."}, decode that string...
	var sessionManagerData map[string]any
	json.NewDecoder(sessionManagerResponse.Body).Decode(&sessionManagerData)
	// ...and access the data by key (requires type assertion).
	password := genericData["password"].(string)
	log.Printf("Password: " + password)


	
	// Now we have the password to use when we create the new Proxy object. First we have to create a URL...
	proxyTargetURL, err := url.Parse(targetURLStr)
	if err != nil {
		return fmt.Errorf("invalid target URL %s: %w", targetURLStr, err)
	}
	// ...then we can create a new reverse proxy instance to that URL.
	rcloneProxy := httputil.NewSingleHostReverseProxy(proxyTargetURL)
	
	// Customize the proxy's director to handle headers correctly.
	originalDirector := rcloneProxy.Director
	rcloneProxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// Ensure the host header matches the target so Rclone doesn't reject it.
		req.Host = targetURL.Host

		// rclone uses basic authentication, so here we can inject the username and password required by rclone
		// so access is seemless for our (already authenticated) users.
		log.Printf("Basic auth: %s %s", key, password)
		req.SetBasicAuth(key, password)
	}
	
	pr.mu.Lock() // Block readers and other writers.
	defer pr.mu.Unlock()
	
	pr.proxies[key] = proxy
	return nil
}

// A global instance of the proxy registry to store multiple proxies to user rclone instances.
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
