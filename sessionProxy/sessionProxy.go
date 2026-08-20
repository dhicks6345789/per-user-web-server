// A routing application designed to take traffic arriving at various endpoints ("/rclone", "app", etc) and route it to the appropriate
// user's development environment instance. This gives the user a handy way of accessing applications running in their development
// environment (supplied by the system, like rclone, or their own that they've run or developed themselves) directly from the browser,
// but still behind the Pangolin authenticating proxy.

package main

import (
	_ "embed"
	"os"
	"fmt"
	"log"
	"sync"
	"time"
	"strings"
	"strconv"
	"net"
	"sort"
	"net/url"
	"net/http"
	"html"
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
	sessionProxy := httputil.NewSingleHostReverseProxy(proxyTargetURL)
	
	// Customize the proxy's director to handle headers correctly.
	originalDirector := sessionProxy.Director
	sessionProxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// rclone can use basic authentication, so here we can inject the username and password required by rclone
		// so access is seemless for our (already authenticated) users.
		req.SetBasicAuth(username, password)
		
		// Ensure the host header matches the target so Rclone doesn't reject it.
		req.Host = proxyTargetURL.Host
		
		// // Pass the original prefix so the downstream app can map static asset MIME types correctly.
		// req.Header.Set("X-Forwarded-Prefix", "/app/" + username + "/" + targetURLStr)
	}
	
	pr.mu.Lock() // Block readers and other writers.
	defer pr.mu.Unlock()
	
	pr.proxies[username] = sessionProxy
	pr.passwords[username] = password
	return nil
}

// A global instance of the proxy registry to store multiple proxies to user rclone instances.
var sessionProxies = newProxyRegistry()



// The HTML page that explains the "/app/username/portnumber/" URL scheme to the user, with a form that
// helps them construct the URL to their app. Loaded from the "appIndex.html" file at build time using go:embed.
//
//go:embed appIndex.html
var appIndexHTML string

// The network ports we scan for on each user's session to detect running applications. These cover the most
// commonly used development ports, and are checked concurrently with a short timeout so the page still loads quickly.
var scanPorts = []int{3000, 5000, 8000, 8080, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 8088, 8089, 8090}

// scanUserPorts checks which of the common application ports are currently accepting connections in a user's session
// (their Desktop Docker container). It returns the list of open ports, sorted ascending.
func scanUserPorts(username string) []int {
	var openPorts []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, port := range scanPorts {
		// Port 8090 hosts the rclone GUI server, which is handled separately (via the /rclone endpoint) and isn't
		// a user application, so skip it.
		if port == 8090 {
			continue
		}
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			address := net.JoinHostPort("desktop-"+username, strconv.Itoa(p))
			conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
			if err != nil {
				return
			}
			conn.Close()
			mu.Lock()
			openPorts = append(openPorts, p)
			mu.Unlock()
		}(port)
	}

	wg.Wait()
	sort.Ints(openPorts)
	return openPorts
}

// buildPortScanHTML scans the user's session for open ports and returns the HTML section listing them as links
// to the corresponding /app/ URLs. Returns an empty string if no ports are open.
func buildPortScanHTML(username string) string {
	openPorts := scanUserPorts(username)
	if len(openPorts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<div class=\"portscan\"><h2>Running applications</h2><p>The following ports appear to have applications running:</p><ul>")
	for _, port := range openPorts {
		url := "/app/" + html.EscapeString(username) + "/" + strconv.Itoa(port) + "/"
		sb.WriteString("<li><a href=\"" + url + "\">Port " + strconv.Itoa(port) + "</a> <button type=\"button\" class=\"copy-port\" data-url=\"" + url + "\" title=\"Copy URL\" aria-label=\"Copy URL\"><i class=\"bi bi-copy\"></i></button></li>")
	}
	sb.WriteString("</ul></div>")
	return sb.String()
}

// Serves the app index HTML page, filling in the current user's username (taken from the "Remote-User" header injected
// by Pangolin) in place of the "{{USERNAME}}" placeholder, and the results of a quick port scan in place of "{{PORTS}}".
func serveAppIndex(w http.ResponseWriter, username string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.Replace(appIndexHTML, "{{USERNAME}}", html.EscapeString(username), -1)
	page = strings.Replace(page, "{{PORTS}}", buildPortScanHTML(username), -1)
	w.Write([]byte(page))
}

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
		proxy, password, exists := sessionProxies.get(username)
		if exists == false {
			// If we don't have an existing session, make sure one is started, getting the connection password to use in the process.
			password = connectToSession(username, true)

			// Create a new proxy object to connect with.
			sessionProxies.set(username, password, "http://desktop-" + username + ":8090")
			proxy, password, exists = sessionProxies.get(username)
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

		originalPath := r.URL.Path

		// Split the URL into 3 parts: username, port, and everything else.
		URLParts := strings.SplitN(r.URL.Path, "/", 3)
		if len(URLParts) < 2 || URLParts[0] == "" || URLParts[1] == "" {
			// No username and/or port supplied - show the user an HTML page explaining the URL scheme.
			log.Printf("Serving app index page: %s %s", r.Method, r.URL.Path)
			// Get the current user's username (the "Remote-User" HTTP header value injected by Pangolin).
			username := strings.Split(r.Header.Get("Remote-User"), "@")[0]
			serveAppIndex(w, username)
			return
		}

		if len(URLParts) >= 2 {
			URLUsername := URLParts[0]
			URLPort := URLParts[1]
			var URLRemainder string
			if len(URLParts) == 3 {
				URLRemainder = URLParts[2]
			}

			// The proxy registry is keyed by a unique identifier. As a single user can run multiple apps on
			// different ports, the key must include the port so a proxy created for one port isn't incorrectly
			// reused for another. e.g. accessing both port 8080 and 8081 should give two different apps.
			proxyKey := URLUsername + ":" + URLPort

			// See if a user session to the given user's Desktop Docker container (which is where the "app" / server will be running) exists. Don't create a session if one doesn't exist,
			// that's up to the user themselves.
			proxy, password, exists := sessionProxies.get(proxyKey)
			if exists == false {
				password = connectToSession(URLUsername, false)
				
				// If we get a blank password, a session doesn't exist - return an error.
				if password == "" {
					http.Error(w, "Application endpoint not found - user session for " + URLUsername + " not running.", http.StatusNotFound)
					return
				} else {
					// Create a new proxy object to connect with.
					sessionProxies.set(proxyKey, password, "http://desktop-" + URLUsername + ":" + URLPort)
					proxy, password, exists = sessionProxies.get(proxyKey)
				}
			}
			
			// Rewrite the URL to point at the given user's app.
			r.URL.Path = "/" + URLRemainder

			if r.URL.Path == "/" && !strings.HasSuffix(originalPath, "/") {
				log.Printf("Redirecting (Error 301) root request with no trailing slash (see RFC 3986): %s %s", r.Method, r.URL.Path)
				http.Redirect(w, r, "/app/" + originalPath + "/", http.StatusMovedPermanently)
				return
			}
			
			log.Printf("Re-written app request: %s %s", r.Method, r.URL.Path)
			proxy.ServeHTTP(w, r)
		} else {
			http.Error(w, "Endpoint not found - app routing requested, but not enough parts to URL.", http.StatusNotFound)
			return
		}
	})
	
	http.Handle("/rclone/", http.StripPrefix("/rclone/", rcloneHandler))
	http.Handle("/app/", http.StripPrefix("/app/", appHandler))
	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Serving app index page: %s %s", r.Method, r.URL.Path)
		username := strings.Split(r.Header.Get("Remote-User"), "@")[0]
		serveAppIndex(w, username)
	})
	
	// Execution starts here.
	log.Println("sessionProxy starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
