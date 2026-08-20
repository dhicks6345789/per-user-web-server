// A routing application designed to take traffic arriving at various endpoints ("/rclone", "app", etc) and route it to the appropriate
// user's development environment instance. This gives the user a handy way of accessing applications running in their development
// environment (supplied by the system, like rclone, or their own that they've run or developed themselves) directly from the browser,
// but still behind the Pangolin authenticating proxy.

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
		
		// // Pass the original prefix so the downstream app can map static asset MIME types correctly.
		// req.Header.Set("X-Forwarded-Prefix", "/app/" + username + "/" + targetURLStr)
	}
	
	pr.mu.Lock() // Block readers and other writers.
	defer pr.mu.Unlock()
	
	pr.proxies[username] = rcloneProxy
	pr.passwords[username] = password
	return nil
}

// A global instance of the proxy registry to store multiple proxies to user rclone instances.
var rcloneProxies = newProxyRegistry()



// Serves an HTML page that explains the "/app/username/portnumber/" URL scheme to the user, with a form that
// helps them construct the URL to their app.
func serveAppIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Your Applications</title>
<style>
	body { font-family: -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 0; padding: 2rem; background: #f5f5f7; color: #1d1d1f; }
	.card { max-width: 40rem; margin: 2rem auto; background: #fff; padding: 2rem; border-radius: 12px; box-shadow: 0 2px 10px rgba(0,0,0,0.08); }
	h1 { margin-top: 0; }
	code { background: #eee; padding: 0.1rem 0.4rem; border-radius: 4px; font-size: 0.95em; }
	label { display: block; margin: 1rem 0 0.25rem; font-weight: 600; }
	input { width: 100%; box-sizing: border-box; padding: 0.6rem; font-size: 1rem; border: 1px solid #ccc; border-radius: 6px; }
	button { margin-top: 1.25rem; padding: 0.6rem 1.25rem; font-size: 1rem; border: none; border-radius: 6px; background: #0071e3; color: #fff; cursor: pointer; }
	button:hover { background: #005bbd; }
	.result { margin-top: 1.5rem; display: none; word-break: break-all; }
	.result a { color: #0071e3; }
</style>
</head>
<body>
<div class="card">
	<h1>Your Applications</h1>
	<p>Applications you run inside your development environment are accessed via a URL of the form:</p>
	<p><code>/app/&lt;username&gt;/&lt;portnumber&gt;/</code></p>
	<p>For example, if your username is <code>student1</code> and you have an app listening on port <code>8080</code>, you would visit <code>/app/student1/8080/</code>.</p>
	<p>Use the form below to construct the URL for your app.</p>

	<label for="username">Username</label>
	<input type="text" id="username" name="username" placeholder="e.g. student1" autocomplete="off">

	<label for="port">Port number</label>
	<input type="text" id="port" name="port" placeholder="e.g. 8080" autocomplete="off">

	<button type="button" id="build">Build my URL</button>

	<div class="result" id="result">
		<p>Your app URL is:</p>
		<p><a id="appUrl" href="#" target="_blank" rel="noopener"></a></p>
	</div>
</div>
<script>
	document.getElementById("build").addEventListener("click", function () {
		var username = document.getElementById("username").value.trim();
		var port = document.getElementById("port").value.trim();
		var result = document.getElementById("result");
		var link = document.getElementById("appUrl");
		if (username === "" || port === "") {
			alert("Please enter both a username and a port number.");
			return;
		}
		var url = "/app/" + encodeURIComponent(username) + "/" + encodeURIComponent(port) + "/";
		link.textContent = url;
		link.href = url;
		result.style.display = "block";
	});
</script>
</body>
</html>`))
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

		originalPath := r.URL.Path

		// Split the URL into 3 parts: username, port, and everything else.
		URLParts := strings.SplitN(r.URL.Path, "/", 3)
		if len(URLParts) < 2 || URLParts[0] == "" || URLParts[1] == "" {
			// No username and/or port supplied - show the user an HTML page explaining the URL scheme.
			log.Printf("Serving app index page: %s %s", r.Method, r.URL.Path)
			serveAppIndex(w)
			return
		}

		if len(URLParts) >= 2 {
			URLUsername := URLParts[0]
			URLPort := URLParts[1]
			var URLRemainder string
			if len(URLParts) == 3 {
				URLRemainder = URLParts[2]
			}

			// See if a user session to the given user's Desktop Docker container (which is where the "app" / server will be running) exists. Don't create a session if one doesn't exist,
			// that's up to the user themselves.
			proxy, password, exists := rcloneProxies.get(URLUsername)
			if exists == false {
				password = connectToSession(URLUsername, false)
				
				// If we get a blank password, a session doesn't exist - return an error.
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
		serveAppIndex(w)
	})
	
	// Execution starts here.
	log.Println("sessionProxy starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
