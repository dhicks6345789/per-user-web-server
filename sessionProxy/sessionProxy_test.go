package main

import (
	"net/http"
	"testing"
)

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
