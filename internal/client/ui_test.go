package client

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedConsoleLocalizationAssets(t *testing.T) {
	sub, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	index := string(sub)
	for _, fragment := range []string{`src="/i18n.js"`, `data-locale="zh"`, `data-locale="en"`} {
		if !strings.Contains(index, fragment) {
			t.Fatalf("embedded console is missing %q", fragment)
		}
	}

	i18n, err := webFS.ReadFile("web/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"navigator.languages", "localStorage.getItem", "localStorage.setItem", "zh:", "en:"} {
		if !strings.Contains(string(i18n), fragment) {
			t.Fatalf("embedded localization runtime is missing %q", fragment)
		}
	}
}

func TestEmbeddedConsoleServesLocalizationRuntime(t *testing.T) {
	sub, err := webFS.ReadFile("web/i18n.js")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9101/i18n.js", nil)
	resp := httptest.NewRecorder()
	http.FileServer(http.FS(mustWebSub(t))).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	served, err := io.ReadAll(resp.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != string(sub) {
		t.Fatal("served localization runtime differs from embedded asset")
	}
}

func mustWebSub(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

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
