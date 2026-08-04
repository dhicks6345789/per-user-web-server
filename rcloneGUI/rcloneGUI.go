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
	"strconv"
	"net/url"
	"net/http"
	"net/http/httputil"
	"encoding/json"
	"encoding/base64"
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
	passwords map[string]string
}

// NewProxyRegistry initializes the registry
func newProxyRegistry() *ProxyRegistry {
	return &ProxyRegistry{
		proxies: make(map[string]*httputil.ReverseProxy),
		passwords: make(map[string]string),
	}
}

// Get looks up a proxy by its target username.
func (pr *ProxyRegistry) get(username string) (*httputil.ReverseProxy, string, bool) {
	pr.mu.RLock() // Allow multiple readers simultaneously.
	defer pr.mu.RUnlock()
	
	proxy, exists := pr.proxies[username]
	password := pr.passwords[username]
	return proxy, password, exists
}

// Call the connectToSession endpoint on the host's Session Manager to ensure that a "desktop" instance (which runs the rclone GUI server) is running for this user. That endpoint returns the user's generated password
// which we can use for connections.
// To do: Check the session manager is only accepting calls from this container (and the guacAutoConnect client) so users can't call it to create other users' sessions.
func connectToSession(username string, startIfNotRunning bool) (string) {
	// Define our form data to pass via POST to the sessionManager server, using url.Values...
	sessionManagerData := url.Values{}
	sessionManagerData.Set("username", username)
	sessionManagerData.Set("image", "desktop")
	sessionManagerData.Set("start", strconv.FormatBool(startIfNotRunning))
	// ...and encode that data into a string in "bar=baz&foo=qux" format.
	sessionManagerEncodedData := sessionManagerData.Encode()

	// Create a client to call the session manager, with a timeout.
	sessionManagerClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create the POST request using strings.NewReader.
	sessionManagerRequest, err := http.NewRequest("POST", "http://host.docker.internal:8091/connectToSession", strings.NewReader(sessionManagerEncodedData))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return ""
	}

	// Set the correct Content-Type header.
	sessionManagerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute the (POST) request.
	sessionManagerResponse, err := sessionManagerClient.Do(sessionManagerRequest)
	if err != nil {
		log.Printf("Error sending request: %v\n", err)
		return ""
	}
	defer sessionManagerResponse.Body.Close()
	
	// The response should be a string in JSON format, {"port":"..", "password":"..."}, decode that string...
	var responseData map[string]any
	json.NewDecoder(sessionManagerResponse.Body).Decode(&responseData)
	// ...and access the data by key (requires type assertion).
	password := responseData["password"].(string)

	return password
}

// Adds or updates a proxy in the global dictionary.
func (pr *ProxyRegistry) set(username string, password string, targetURLStr string) error {
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
		
		// rclone can use basic authentication, so here we can inject the username and password required by rclone
		// so access is seemless for our (already authenticated) users.
		req.SetBasicAuth(username, password)
		
		// Ensure the host header matches the target so Rclone doesn't reject it.
		req.Host = proxyTargetURL.Host
	}
	
	pr.mu.Lock() // Block readers and other writers.
	defer pr.mu.Unlock()
	
	pr.proxies[username] = rcloneProxy
	pr.passwords[username] = password
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
		log.Printf("Proxying rclone request: %s %s", r.Method, r.URL.Path)
		
		// Get the username ("Remote-User" HTTP header value injected by Pangolin).
		username := strings.Split(r.Header.Get("Remote-User"), "@")[0]
		
		// Make sure a proxy object to the user's Desktop Docker container (which is where rclone will be running) exists.
		proxy, password, exists := rcloneProxies.get(username)
		if exists == false {
			// If we don't have an existing session, make sure one is started, getting the connection password to use in the process.
			password = connectToSession(username, true)

			// Create a new proxy object to connect with.
			rcloneProxies.set(username, password, "http://desktop-" + username + ":8090")
			proxy, password, exists = rcloneProxies.get(username)
		}
		
		// // Rewrite the URL to remove the "/rclone" prefix.
		// r.URL.Path = strings.TrimPrefix(r.URL.Path, "/rclone")
		
		// Redirect the "/" URL to include the (Base64-ed "username:password") login token (if it doesn't already) so the user is logged straight in rather than being shown the "login" screen.
		if r.URL.Path == "/" && !r.URL.Query().Has("login_token") {
			log.Printf("Redirecting request: %s %s", r.Method, r.URL.Path)
			http.Redirect(w, r, "/rclone/?login_token=" + base64.StdEncoding.EncodeToString([]byte(username + ":" + password)), http.StatusSeeOther)
			return
		}
		
		log.Printf("Re-written rclone request: %s %s", r.Method, r.URL.Path)
		proxy.ServeHTTP(w, r)
	})
	
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Proxying app request: %s %s", r.Method, r.URL.Path)

		// Split the URL into 3 parts: username, port, and everything else.
		URLParts := strings.SplitN(r.URL.Path, "/", 3)
		if len(URLParts) >= 2 {
			URLUsername := URLParts[0]
			URLPort := URLParts[1]
			var URLRemainder string
			if len(URLParts) == 3 {
				URLRemainder = URLParts[2]
			}

			// Make sure a proxy object to the user's Desktop Docker container (which is where rclone will be running) exists.
			proxy, password, exists := rcloneProxies.get(URLUsername)
			if exists == false {
				// If we don't have an existing session, make sure one is started, getting the connection password to use in the process.
				password = connectToSession(URLUsername, false)
				
				if password == "" {
					http.Error(w, "Application endpoint not found - user session for " + URLUsername + " not running.", http.StatusNotFound)
					return
				} else {
					// Create a new proxy object to connect with.
					rcloneProxies.set(URLUsername, password, "http://desktop-" + URLUsername + ":" + URLPort)
					proxy, password, exists = rcloneProxies.get(URLUsername)
				}
			}
			
			// Rewrite the URL to point at the given user's app.
			//r.URL.Path = URLRemainder + ":" + URLPort
			r.URL.Path = "/" + URLRemainder

			log.Printf("Re-written app request: %s %s %s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
			proxy.ServeHTTP(w, r)
		} else {
			http.Error(w, "Endpoint not found - app routing requested, but not enough parts to URL.", http.StatusNotFound)
			return
		}
	})
	
	http.Handle("/rclone/", http.StripPrefix("/rclone/", rcloneHandler))
	http.Handle("/app/", http.StripPrefix("/app/", appHandler))
	
	// Execution starts here.
	log.Println("rcloneGUI starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
