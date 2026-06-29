package main

import "testing"

func TestResolveServerURLPrefersEmbeddedDefaultOverSidecar(t *testing.T) {
	t.Setenv("UNYDESK_SERVER", "")
	previousDefault := defaultServerURL
	t.Cleanup(func() {
		defaultServerURL = previousDefault
	})

	defaultServerURL = "https://unydesk.app"
	resolved, source := resolveServerURL("", bootstrapConfig{Server: "http://127.0.0.1:8890"})
	if resolved != defaultServerURL {
		t.Fatalf("expected embedded default %q, got %q", defaultServerURL, resolved)
	}
	if source != "embedded default" {
		t.Fatalf("expected embedded default source, got %q", source)
	}
}

func TestLooksLikeServerAddress(t *testing.T) {
	t.Parallel()

	valid := []string{
		"http://192.168.3.5:8890",
		"https://unydesk.app",
		"https://unydesk.example.com",
		"ws://localhost:8890",
		"192.168.3.5:8890",
		"localhost:8890",
		"unydesk.local:8890",
	}
	for _, value := range valid {
		if !looksLikeServerAddress(value) {
			t.Fatalf("expected %q to look like a server address", value)
		}
	}

	invalid := []string{
		"",
		"MuCkiBi8oGKfagpnmqb93Ue0Mb77Iagc",
		"339 335 816",
		"ftp://192.168.3.5:8890",
		"http://",
	}
	for _, value := range invalid {
		if looksLikeServerAddress(value) {
			t.Fatalf("expected %q to be rejected as a server address", value)
		}
	}
}
