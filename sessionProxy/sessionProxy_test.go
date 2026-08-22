package main

import (
	"bytes"
	"compress/gzip"
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
	// rclone's OAuth auth server handles the callback at its root path, regardless of the public redirect URI path.
	if received.path != "/" {
		t.Fatalf("expected callback forwarded to the auth server root, got %q", received.path)
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

// The "/rclone/auth" endpoint should be proxied to the requesting user's own desktop container, with the
// "/rclone" prefix stripped so it lands on "/auth" on the OAuth auth server, preserving the query (state).
func TestHandleRcloneOAuthAuthProxiesToUser(t *testing.T) {
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

	original := rcloneOAuthTarget
	rcloneOAuthTarget = func(username string) string { return testServer.URL }
	t.Cleanup(func() { rcloneOAuthTarget = original })

	req := httptest.NewRequest(http.MethodGet, "/rclone/auth?state=abc", nil)
	req.Header.Set("Remote-User", "jane.doe@example.com")
	rec := httptest.NewRecorder()
	handleRcloneOAuthAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	received.mu.Lock()
	defer received.mu.Unlock()
	if received.path != "/auth" {
		t.Fatalf("expected auth forwarded to /auth, got %q", received.path)
	}
	if received.query != "state=abc" {
		t.Fatalf("expected auth query preserved, got %q", received.query)
	}
}

// The OAuth helper script (injected into the GUI HTML) must contain the polling fetch to config/oauthstatus and the
// public-path rewrite of the auth URL.
func TestOAuthHelperScript(t *testing.T) {
	s := oauthHelperScript()
	for _, want := range []string{"config/oauthstatus", `"/rclone"`, "Complete OAuth"} {
		if !strings.Contains(s, want) {
			t.Fatalf("expected OAuth helper script to contain %q", want)
		}
	}
}

// The RC API target for a user points at their desktop container's RC API port (5572).
func TestRcloneRCATarget(t *testing.T) {
	if got := rcloneRCATarget("jane.doe"); got != "http://desktop-jane.doe:5572" {
		t.Fatalf("unexpected RC API target %q", got)
	}
}

// rewriteGUIGUI should prefix root-absolute asset URLs in HTML and JS responses with the "/rclone" sub-path, and leave
// other response types (CSS, images, etc.) untouched.
func TestRewriteGUIGUI(t *testing.T) {
	html := `<!doctype html><link rel="icon" href="/icon.svg"><script src="/assets/index-abc.js"></script>`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(html)),
	}
	if err := rewriteGUIGUI(resp); err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	got := string(out)
	for _, want := range []string{`href="/rclone/icon.svg"`, `src="/rclone/assets/index-abc.js"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected rewritten HTML to contain %q, got %q", want, got)
		}
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected HTML response to be non-cacheable, got %q", cc)
	}

	// A gzip-encoded HTML response must be decompressed, rewritten, and the encoding cleared so the browser receives
	// the rewritten (uncompressed) HTML with the correct asset paths.
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	gz.Write([]byte(html))
	gz.Close()
	respGz := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html"}, "Content-Encoding": []string{"gzip"}},
		Body:       io.NopCloser(bytes.NewReader(gzBuf.Bytes())),
	}
	if err := rewriteGUIGUI(respGz); err != nil {
		t.Fatal(err)
	}
	if enc := respGz.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("expected gzip encoding to be cleared, got %q", enc)
	}
	outGz, _ := io.ReadAll(respGz.Body)
	if !strings.Contains(string(outGz), `src="/rclone/assets/index-abc.js"`) {
		t.Fatalf("expected gzip HTML to be rewritten, got %q", string(outGz))
	}

	// The JS bundle renders the logo as "/icon.svg" (root-absolute); it must be prefixed too, while other JS content
	// (e.g. "/assets/..." module paths) is left alone since the browser loads those via the rewritten HTML.
	js := `var logo = '<img src="/icon.svg">', mod = "/assets/index-abc.js";`
	respJS := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/javascript"}},
		Body:       io.NopCloser(bytes.NewBufferString(js)),
	}
	if err := rewriteGUIGUI(respJS); err != nil {
		t.Fatal(err)
	}
	outJS, _ := io.ReadAll(respJS.Body)
	gotJS := string(outJS)
	if !strings.Contains(gotJS, `"/rclone/icon.svg"`) {
		t.Fatalf("expected JS logo path to be rewritten, got %q", gotJS)
	}
	if strings.Contains(gotJS, `"/rclone/assets/index-abc.js"`) {
		t.Fatalf("expected JS module path to be left unchanged, got %q", gotJS)
	}

	// Non-HTML/JS responses (e.g. CSS) must be passed through unchanged.
	css := `.x { background: url("/icon.svg"); }`
	respCSS := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/css"}},
		Body:       io.NopCloser(bytes.NewBufferString(css)),
	}
	if err := rewriteGUIGUI(respCSS); err != nil {
		t.Fatal(err)
	}
	outCSS, _ := io.ReadAll(respCSS.Body)
	if string(outCSS) != css {
		t.Fatalf("non-HTML/JS response was modified: %q", string(outCSS))
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
