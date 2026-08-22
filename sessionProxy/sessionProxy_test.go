package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Without a Remote-User header (which Pangolin injects) we can't know whose container to route the OAuth
// callback to, so the request should be rejected.
func TestHandleRcloneOAuthCallbackMissingUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rclone/oauth2callback?state=abc&code=123", nil)
	rec := httptest.NewRecorder()
	handleRcloneOAuthCallback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Remote-User, got %d", rec.Code)
	}
}

// A valid callback should be proxied to the requesting user's own desktop container, with the "/rclone" prefix
// stripped and the OAuth query parameters (state, code) preserved.
func TestHandleRcloneOAuthCallbackProxiesToUser(t *testing.T) {
	var received struct {
		mu    sync.Mutex
		path  string
		query string
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.mu.Lock()
		received.path = r.URL.Path
		received.query = r.URL.RawQuery
		received.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Redirect the callback target to the local test server instead of a real desktop container.
	original := rcloneOAuthTarget
	rcloneOAuthTarget = func(username string) string { return testServer.URL }
	t.Cleanup(func() { rcloneOAuthTarget = original })

	req := httptest.NewRequest(http.MethodGet, "/rclone/oauth2callback?state=abc&code=123", nil)
	req.Header.Set("Remote-User", "jane.doe@example.com")
	rec := httptest.NewRecorder()
	handleRcloneOAuthCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	received.mu.Lock()
	defer received.mu.Unlock()
	if received.path != "/oauth2callback" {
		t.Fatalf("expected callback forwarded to /oauth2callback, got %q", received.path)
	}
	if received.query != "state=abc&code=123" {
		t.Fatalf("expected OAuth query preserved, got %q", received.query)
	}
}

// The obfuscated value should be a fixed-length hex digest that is deterministic for the same input, differs
// between inputs, and is never blank for a non-blank input.
func TestObfuscateIdentityValue(t *testing.T) {
	if got := obfuscateIdentityValue(""); got != "" {
		t.Fatalf("expected empty input to stay empty, got %q", got)
	}

	first := obfuscateIdentityValue("jane.doe@example.com")
	if len(first) != 64 {
		t.Fatalf("expected a 64-char SHA-256 hex digest, got %d chars", len(first))
	}
	if second := obfuscateIdentityValue("jane.doe@example.com"); second != first {
		t.Fatalf("expected deterministic output, got %q then %q", first, second)
	}
	if other := obfuscateIdentityValue("john.doe@example.com"); other == first {
		t.Fatalf("expected different inputs to produce different digests")
	}
	// The digest must not contain the original value (i.e. it is actually hashed).
	if other := obfuscateIdentityValue("jane.doe@example.com"); containsIdentity(other, "jane.doe") {
		t.Fatalf("digest unexpectedly contains the original identity: %q", other)
	}
}

// The RC API target for a user points at their desktop container's RC API port (5572).
func TestRcloneRCATarget(t *testing.T) {
	if got := rcloneRCATarget("jane.doe"); got != "http://desktop-jane.doe:5572" {
		t.Fatalf("unexpected RC API target %q", got)
	}
}

// rewriteGUIHTML should prefix root-absolute asset URLs in an HTML response with the "/rclone" sub-path, but leave
// non-HTML responses (e.g. the SPA's JS bundle) untouched.
func TestRewriteGUIHTML(t *testing.T) {
	html := `<!doctype html><link rel="icon" href="/icon.svg"><script src="/assets/index-abc.js"></script>`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(html)),
	}
	if err := rewriteGUIHTML(resp); err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	got := string(out)
	for _, want := range []string{`href="/rclone/icon.svg"`, `src="/rclone/assets/index-abc.js"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected rewritten HTML to contain %q, got %q", want, got)
		}
	}

	// Non-HTML responses must be passed through unchanged.
	js := `var x = "/assets/index-abc.js";`
	respJS := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/javascript"}},
		Body:       io.NopCloser(bytes.NewBufferString(js)),
	}
	if err := rewriteGUIHTML(respJS); err != nil {
		t.Fatal(err)
	}
	outJS, _ := io.ReadAll(respJS.Body)
	if string(outJS) != js {
		t.Fatalf("non-HTML response was modified: %q", string(outJS))
	}
}

func containsIdentity(theHash, thePart string) bool {
	for i := 0; i+len(thePart) <= len(theHash); i++ {
		if theHash[i:i+len(thePart)] == thePart {
			return true
		}
	}
	return false
}

// The identifying Remote-* headers should be replaced with hashes, while unrelated headers are left untouched.
func TestObfuscateIdentityHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Remote-User", "user_123")
	req.Header.Set("Remote-Email", "jane.doe@example.com")
	req.Header.Set("Remote-Name", "Jane Doe")
	req.Header.Set("Remote-Role", "admin")
	req.Header.Set("X-Custom", "keep-me")

	obfuscateIdentityHeaders(req)

	if got := req.Header.Get("Remote-User"); got == "user_123" {
		t.Fatalf("Remote-User was not obfuscated")
	}
	if got := req.Header.Get("Remote-Email"); got == "jane.doe@example.com" {
		t.Fatalf("Remote-Email was not obfuscated")
	}
	if got := req.Header.Get("Remote-Name"); got == "Jane Doe" {
		t.Fatalf("Remote-Name was not obfuscated")
	}
	if got := req.Header.Get("X-Custom"); got != "keep-me" {
		t.Fatalf("unrelated header was modified: %q", got)
	}
}
