// A small web-based control panel for the per-user-web-server (PUWS) project. Provides an admin-only
// dashboard showing the status of the server: active user sessions (Docker containers) and host resource usage.
//
// Authentication is handled by Pangolin, which passes the user's identity to proxied applications via the
// "Remote-User" and "Remote-Role" headers. This application only responds to users whose "Remote-Role"
// header has the value "admin" - everyone else receives a 403 Forbidden response.
//
// The actual status data is gathered by the Session Manager service running on the host server, which this
// application calls using a shared admin key. The key is passed to this container via the "ADMIN_KEY"
// environment variable, and must match the "adminKey" value in the Session Manager's config file.

package main

import (
	_ "embed"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// The value of the "Remote-Role" header we accept as an administrator. Pangolin passes the user's role
// through in this header, other values are rejected.
const adminRole = "admin"

// The name of the header the Session Manager expects to contain the shared admin key.
const adminKeyHeader = "X-Admin-Key"

// The location of the Session Manager service, running on the host machine. "host.docker.internal"
// is the standard Docker way to refer to the host from inside a container.
const sessionManagerURL = "http://host.docker.internal:8091"

// The admin dashboard web page, embedded into this executable so we don't have to worry about shipping
// static files around. This needs to be built from the "adminPanel" folder.

//go:embed index.html
var indexHTML []byte

func main() {
	// Read the shared admin key from an environment variable. This value is passed to this container
	// when it is created, and must match the "adminKey" value in the Session Manager's config file.
	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		log.Fatal("ADMIN_KEY environment variable not set - refusing to start.")
	}

	// A wrapper for the request handlers that only lets users with an "admin" role through, based on
	// the "Remote-Role" header value injected by Pangolin.
	adminOnly := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Check the "Remote-Role" header has the value we're looking for.
			if !strings.EqualFold(r.Header.Get("Remote-Role"), adminRole) {
				log.Print("adminPanel, rejected request from user: " + r.Header.Get("Remote-User"))
				http.Error(w, "Forbidden: Administrators only.", http.StatusForbidden)
				return
			}
			log.Print("adminPanel, request from user: " + r.Header.Get("Remote-User"))
			next(w, r)
		}
	}

	// The admin dashboard web page.
	http.HandleFunc("/", adminOnly(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	}))

	// The JSON API endpoint that fetches the server status from the Session Manager and returns it
	// to the dashboard page.
	http.HandleFunc("/api/status", adminOnly(func(w http.ResponseWriter, r *http.Request) {
		// Build a request to the Session Manager's admin status endpoint...
		sessionManagerRequest, requestErr := http.NewRequest(http.MethodGet, sessionManagerURL+"/admin/status", nil)
		if requestErr != nil {
			http.Error(w, "Error building Session Manager request: "+requestErr.Error(), http.StatusInternalServerError)
			return
		}
		// ...adding the shared admin key as a header.
		sessionManagerRequest.Header.Set(adminKeyHeader, adminKey)

		// Send the request to the Session Manager.
		sessionManagerClient := &http.Client{}
		sessionManagerResponse, clientErr := sessionManagerClient.Do(sessionManagerRequest)
		if clientErr != nil {
			http.Error(w, "Error contacting Session Manager: "+clientErr.Error(), http.StatusBadGateway)
			return
		}
		defer sessionManagerResponse.Body.Close()

		// Pass the response (and the status code) straight back to the dashboard page.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(sessionManagerResponse.StatusCode)
		io.Copy(w, sessionManagerResponse.Body)
	}))

	// Execution starts here.
	log.Println("adminPanel starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
