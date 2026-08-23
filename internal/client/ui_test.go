package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireLocalMutation(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		origin     string
		wantOK     bool
		wantStatus int
	}{
		{name: "native client", method: http.MethodPost, wantOK: true, wantStatus: http.StatusOK},
		{name: "loopback IPv4", method: http.MethodPost, origin: "http://127.0.0.1:9101", wantOK: true, wantStatus: http.StatusOK},
		{name: "loopback IPv6", method: http.MethodPost, origin: "http://[::1]:9101", wantOK: true, wantStatus: http.StatusOK},
		{name: "localhost", method: http.MethodPost, origin: "http://localhost:9101", wantOK: true, wantStatus: http.StatusOK},
		{name: "remote origin", method: http.MethodPost, origin: "https://example.com", wantOK: false, wantStatus: http.StatusForbidden},
		{name: "opaque origin", method: http.MethodPost, origin: "null", wantOK: false, wantStatus: http.StatusForbidden},
		{name: "wrong method", method: http.MethodGet, wantOK: false, wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://127.0.0.1:9101/api/send-clipboard", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			resp := httptest.NewRecorder()
			if got := requireLocalMutation(resp, req); got != tt.wantOK {
				t.Fatalf("requireLocalMutation() = %v, want %v", got, tt.wantOK)
			}
			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.Code, tt.wantStatus)
			}
		})
	}
}
