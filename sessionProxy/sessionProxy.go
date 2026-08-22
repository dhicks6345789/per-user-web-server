// A routing application designed to take traffic arriving at various endpoints ("/rclone", "app", etc) and route it to the appropriate
// user's development environment instance. This gives the user a handy way of accessing applications running in their development
// environment (supplied by the system, like rclone, or their own that they've run or developed themselves) directly from the browser,
// but still behind the Pangolin authenticating proxy.

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The root web server folder. Important: don't include include the trailing slash so the prefix gets removed properly from request path strings.
const rootPath = "/var/www"

// A salt prepended to identity header values before hashing, so the resulting digests can't be reversed with a
// simple dictionary / rainbow-table lookup (email addresses are fairly predictable). Read from the "IDENTITY_SALT"
// environment variable, which is set by the install script; falls back to a constant so the app still works if it
// isn't configured.
var identitySalt = func() string {
	if value := os.Getenv("IDENTITY_SALT"); value != "" {
		return value
	}
	return "per-user-web-server-identity-salt"
}()

// Obfuscates an identity header value (username / email / name) into a salted SHA-256 digest. The same input always
// produces the same digest, so an app can still recognise a returning user, but can't learn their real identity.
func obfuscateIdentityValue(theValue string) string {
	if theValue == "" {
		return ""
	}
	hasher := sha256.New()
	hasher.Write([]byte(identitySalt))
	hasher.Write([]byte(":"))
	hasher.Write([]byte(theValue))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Rewrites the identifying "Remote-*" headers on the given request to salted hashes, so that when a user accesses
// another user's application the app owner can't see the accessing user's real username / email address / name.
func obfuscateIdentityHeaders(r *http.Request) {
	for _, header := range []string{"Remote-User", "Remote-Email", "Remote-Name"} {
		if value := r.Header.Get(header); value != "" {
			r.Header.Set(header, obfuscateIdentityValue(value))
		}
	}
}

/* We need a separate proxy object for each rclone instance running inisde a user's container. Standard Go maps are not safe for concurrent use,
   therfore we protect our global dictionary using a sync.RWMutex to prevent race conditions when multiple incoming HTTP requests try to read
   from or write to the dictionary simultaneously. */

// ProxyRegistry manages our global dictionary of reverse proxies safely.
type ProxyRegistry struct {
	mu        sync.RWMutex
	proxies   map[string]*httputil.ReverseProxy
	passwords map[string]string
}

// NewProxyRegistry initializes the registry
func newProxyRegistry() *ProxyRegistry {
	return &ProxyRegistry{
		proxies:   make(map[string]*httputil.ReverseProxy),
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
func connectToSession(username string, startIfNotRunning bool) string {
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
func (pr *ProxyRegistry) set(username string, password string, targetURLStr string, rewriteHTML bool) error {
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

	// The rclone web GUI (served at "/rclone") is a single-page app built with root-absolute asset URLs (e.g. "/assets/...",
	// "/icon.svg"). Served behind a path prefix, a browser would resolve those against the domain root and miss the proxy
	// route. When set, we rewrite those paths in HTML responses to add the "/rclone" prefix so the assets load correctly.
	if rewriteHTML {
		sessionProxy.ModifyResponse = rewriteGUIHTML
	}

	pr.mu.Lock() // Block readers and other writers.
	defer pr.mu.Unlock()

	pr.proxies[username] = sessionProxy
	pr.passwords[username] = password
	return nil
}

// rewriteGUIHTML rewrites a response's root-absolute asset URLs to include the "/rclone" path prefix, so the SPA's
// static assets load correctly when the GUI is served behind that prefix. Only HTML responses are rewritten.
func rewriteGUIHTML(resp *http.Response) error {
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	bodyStr := string(body)

	rewritten := strings.ReplaceAll(bodyStr, `"/assets/`, `"/rclone/assets/`)
	rewritten = strings.ReplaceAll(rewritten, `"/icon.svg"`, `"/rclone/icon.svg"`)

	resp.Body = io.NopCloser(bytes.NewBufferString(rewritten))
	resp.ContentLength = int64(len(rewritten))
	resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	// The HTML is rewritten per-request (asset URLs depend on the serving prefix), so it must not be cached or a
	// browser could keep serving stale, un-rewritten asset paths. The hashed static assets themselves stay cacheable.
	resp.Header.Set("Cache-Control", "no-cache")
	return nil
}

// A global instance of the proxy registry to store multiple proxies to user rclone instances.
var sessionProxies = newProxyRegistry()

// A separate registry for the per-user rclone RC API servers (which, unlike the GUI server, are keyed only by username).
var rcloneRCProxies = newProxyRegistry()

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

// The port rclone's OAuth callback webserver listens on inside the user's desktop container. Our custom-built
// rclone binds this to 0.0.0.0 (upstream uses 127.0.0.1) so it's reachable from this proxy container.
const rcloneOAuthPort = 53682

// Builds the address of the OAuth callback webserver in a given user's desktop container. A variable so tests can
// redirect it to a local test server.
var rcloneOAuthTarget = func(username string) string {
	return "http://desktop-" + username + ":" + strconv.Itoa(rcloneOAuthPort)
}

// The port rclone's remote control (RC) API server listens on inside the user's desktop container. Since rclone v1.74
// the "rclone gui" command serves the web GUI and the RC API on two separate ports. This is the fixed RC API port the
// startup script binds, so this proxy can forward "/rclone/rc" requests to it.
const rcloneRCAPIPort = 5572

// Builds the address of the RC API server in a given user's desktop container. A variable so tests can redirect it to
// a local test server.
var rcloneRCATarget = func(username string) string {
	return "http://desktop-" + username + ":" + strconv.Itoa(rcloneRCAPIPort)
}

// Handles the "/rclone/oauth2callback" endpoint. When a user adds a cloud remote in the rclone web GUI, the OAuth
// flow redirects the user's browser back to this URL (which is registered with the OAuth provider). We route it to
// the OAuth callback webserver running in the requesting user's own desktop container - the one that started the
// flow - so the handshake completes. The redirect URI is a single, shared URL for the whole site, but it's routed
// by the "Remote-User" header (injected by Pangolin), so each user's callback always lands in their own container.
func handleRcloneOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Get the current user's username (the "Remote-User" HTTP header value injected by Pangolin).
	username := strings.Split(r.Header.Get("Remote-User"), "@")[0]
	if username == "" {
		log.Print("rclone OAuth callback: no user supplied (missing Remote-User header).")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Route to the user's own desktop container (where the rclone GUI / OAuth flow is running). No session is
	// started here - a callback only ever arrives while that container already has an active OAuth flow.
	targetURL, err := url.Parse(rcloneOAuthTarget(username))
	if err != nil {
		log.Printf("rclone OAuth callback: error parsing target URL: %v", err)
		http.Error(w, "Bad gateway", http.StatusBadGateway)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	// Customize the proxy's director to forward the callback onto the right path in the container.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Ensure the Host header matches the target so rclone's callback server doesn't reject it.
		req.Host = targetURL.Host

		// Strip the "/rclone" prefix so the callback lands on "/oauth2callback" inside the container (the
		// query parameters - state, code, etc - are preserved by the proxy).
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/rclone")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}

	log.Printf("rclone OAuth callback for user %s: %s %s", username, r.Method, r.URL.String())
	proxy.ServeHTTP(w, r)
}

func main() {
	rcloneHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Proxying rclone request: %s %s", r.Method, r.URL.Path)

		// Get the username ("Remote-User" HTTP header value injected by Pangolin).
		username := strings.Split(r.Header.Get("Remote-User"), "@")[0]

		// Make sure proxy objects to the user's Desktop Docker container (which is where rclone is running) exist for
		// both the web GUI (port 8090) and the RC API (port 5572). The two proxies share the same session and password.
		guiProxy, _, guiExists := sessionProxies.get(username)
		_, password, rcExists := rcloneRCProxies.get(username)
		if guiExists == false || rcExists == false {
			// If we don't have an existing session, make sure one is started, getting the connection password to use in the process.
			password = connectToSession(username, true)

			// Create proxy objects to connect with - one for the GUI server and one for the RC API server.
			sessionProxies.set(username, password, "http://desktop-"+username+":8090", true)
			rcloneRCProxies.set(username, password, rcloneRCATarget(username), false)
			guiProxy, _, _ = sessionProxies.get(username)
		}

		// Redirect the "/" URL to the GUI's auto-login page, telling the frontend where its RC API lives. Since rclone
		// v1.74 the web GUI and RC API run on separate ports, so we point the frontend at a same-origin path ("/rclone/rc")
		// that this proxy forwards to the user's RC API server (see rcloneRCHandler below). The frontend is handed the
		// username/password so it can authenticate to the RC API seamlessly.
		if (r.URL.Path == "/" || r.URL.Path == "") && !r.URL.Query().Has("url") {
			log.Printf("Redirecting request: %s %s", r.Method, r.URL.Path)
			loginQuery := url.Values{}
			loginQuery.Set("url", "/rclone/rc")
			loginQuery.Set("user", username)
			loginQuery.Set("pass", password)
			http.Redirect(w, r, "/rclone/login?"+loginQuery.Encode(), http.StatusSeeOther)
			return
		}

		log.Printf("Re-written rclone request: %s %s", r.Method, r.URL.Path)
		guiProxy.ServeHTTP(w, r)
	})

	// Proxies requests to a user's rclone RC API server (port 5572 in their desktop container). The GUI frontend makes
	// all its remote-control calls here (paths like "/rclone/rc/rc/noop", i.e. the "/rclone/rc" prefix from its login
	// URL followed by the RC method path), so this handler strips that prefix and forwards to the RC API root.
	rcloneRCHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Proxying rclone RC request: %s %s", r.Method, r.URL.Path)

		// Get the username ("Remote-User" HTTP header value injected by Pangolin).
		username := strings.Split(r.Header.Get("Remote-User"), "@")[0]

		// Make sure a proxy object to the user's RC API server exists (starting their session if necessary).
		rcProxy, _, exists := rcloneRCProxies.get(username)
		if exists == false {
			password := connectToSession(username, true)
			rcloneRCProxies.set(username, password, rcloneRCATarget(username), false)
			rcProxy, _, _ = rcloneRCProxies.get(username)
		}

		rcProxy.ServeHTTP(w, r)
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
					http.Error(w, "Application endpoint not found - user session for "+URLUsername+" not running.", http.StatusNotFound)
					return
				} else {
					// Create a new proxy object to connect with.
					sessionProxies.set(proxyKey, password, "http://desktop-"+URLUsername+":"+URLPort, false)
					proxy, password, exists = sessionProxies.get(proxyKey)
				}
			}

			// Rewrite the URL to point at the given user's app.
			r.URL.Path = "/" + URLRemainder

			if r.URL.Path == "/" && !strings.HasSuffix(originalPath, "/") {
				log.Printf("Redirecting (Error 301) root request with no trailing slash (see RFC 3986): %s %s", r.Method, r.URL.Path)
				http.Redirect(w, r, "/app/"+originalPath+"/", http.StatusMovedPermanently)
				return
			}

			// Obfuscate the accessing user's identity before it reaches the target user's app, so the app owner
			// can't learn other users' real usernames / email addresses from the request headers.
			obfuscateIdentityHeaders(r)

			log.Printf("Re-written app request: %s %s", r.Method, r.URL.Path)
			proxy.ServeHTTP(w, r)
		} else {
			http.Error(w, "Endpoint not found - app routing requested, but not enough parts to URL.", http.StatusNotFound)
			return
		}
	})

	// The rclone OAuth callback is routed to the requesting user's own desktop container (see
	// handleRcloneOAuthCallback). Registering the exact path means it takes precedence over the
	// more general "/rclone/" pattern below.
	http.HandleFunc("/rclone/oauth2callback", handleRcloneOAuthCallback)
	// The more specific "/rclone/rc/" pattern (registered before "/rclone/") takes precedence over it, so requests to
	// a user's RC API are sent to rcloneRCHandler while everything else under "/rclone" goes to the GUI server.
	http.Handle("/rclone/rc/", http.StripPrefix("/rclone/rc", rcloneRCHandler))
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
