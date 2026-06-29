package server

import (
	"net/http"
	"testing"
)

func TestResolveRequestClientIPv4(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name: "forwarded for first IPv4",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10, 10.0.0.4",
			},
			remoteAddr: "10.0.0.2:42100",
			want:       "203.0.113.10",
		},
		{
			name: "real IP fallback",
			headers: map[string]string{
				"X-Real-IP": "198.51.100.24",
			},
			remoteAddr: "10.0.0.2:42100",
			want:       "198.51.100.24",
		},
		{
			name: "cloudflare connecting IP takes precedence",
			headers: map[string]string{
				"CF-Connecting-IP": "198.51.100.77",
				"X-Forwarded-For":  "10.0.0.4",
			},
			remoteAddr: "10.0.0.2:42100",
			want:       "198.51.100.77",
		},
		{
			name: "explicit nginx client IP takes precedence",
			headers: map[string]string{
				"X-UnyDesk-Client-IP": "192.168.2.2",
				"X-Forwarded-For":     "213.169.178.160",
			},
			remoteAddr: "10.0.0.2:42100",
			want:       "192.168.2.2",
		},
		{
			name: "standard forwarded header",
			headers: map[string]string{
				"Forwarded": `for=203.0.113.42;proto=https;host=unydesk.app`,
			},
			remoteAddr: "10.0.0.2:42100",
			want:       "203.0.113.42",
		},
		{
			name:       "remote addr fallback",
			remoteAddr: "192.0.2.44:51234",
			want:       "192.0.2.44",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header:     make(http.Header),
				RemoteAddr: tt.remoteAddr,
			}
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			if got := resolveRequestClientIPv4(req); got != tt.want {
				t.Fatalf("resolveRequestClientIPv4() = %q, want %q", got, tt.want)
			}
		})
	}
}
