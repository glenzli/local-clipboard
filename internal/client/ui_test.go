package client

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedConsoleLocalizationAssets(t *testing.T) {
	sub, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	index := string(sub)
	for _, fragment := range []string{`src="/app-icon.png"`, `src="/i18n.js"`, `data-locale="zh"`, `data-locale="en"`, `class="history-row"`, `class="event-main"`, `/api/reveal`, `showInFolder`} {
		if !strings.Contains(index, fragment) {
			t.Fatalf("embedded console is missing %q", fragment)
		}
	}

	i18n, err := webFS.ReadFile("web/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"navigator.languages", "localStorage.getItem", "localStorage.setItem", "stateApplied", "zh:", "en:"} {
		if !strings.Contains(string(i18n), fragment) {
			t.Fatalf("embedded localization runtime is missing %q", fragment)
		}
	}
}

func TestEmbeddedConsoleServesAppIcon(t *testing.T) {
	icon, err := webFS.ReadFile("web/app-icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(icon, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("embedded app icon is not a PNG")
	}

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9101/app-icon.png", nil)
	resp := httptest.NewRecorder()
	http.FileServer(http.FS(mustWebSub(t))).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	served, err := io.ReadAll(resp.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(served, icon) {
		t.Fatal("served app icon differs from embedded asset")
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

func TestRevealReceivedFileUsesStoredPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "received.txt")
	if err := os.WriteFile(path, []byte("received"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	store.Add(&Event{Dir: "in", Kind: "file", FID: "file-1", State: StateOffered})
	store.SetFIDReceived("file-1", path)

	var revealed string
	u := &UIServer{
		c: &Client{store: store},
		revealPath: func(value string) error {
			revealed = value
			return nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9101/api/reveal", strings.NewReader(`{"fid":"file-1"}`))
	resp := httptest.NewRecorder()
	u.handleReveal(resp, req)
	if resp.Code != http.StatusNoContent || revealed != path {
		t.Fatalf("reveal status=%d path=%q, want status=%d path=%q", resp.Code, revealed, http.StatusNoContent, path)
	}

	snapshot := store.Snapshot(true)
	events := snapshot["events"].([]*Event)
	if len(events) != 1 || events[0].State != StateReceived || events[0].Path != path {
		t.Fatalf("received event = %+v", events)
	}
}

func TestRevealRejectsUnknownOrMissingReceivedFile(t *testing.T) {
	store := NewStore()
	store.Add(&Event{Dir: "in", Kind: "file", FID: "missing", State: StateOffered})
	store.SetFIDReceived("missing", filepath.Join(t.TempDir(), "gone.txt"))
	u := &UIServer{c: &Client{store: store}, revealPath: func(string) error {
		t.Fatal("revealPath should not be called")
		return nil
	}}
	for _, fid := range []string{"unknown", "missing"} {
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9101/api/reveal", strings.NewReader(`{"fid":"`+fid+`"}`))
		resp := httptest.NewRecorder()
		u.handleReveal(resp, req)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("fid %q status=%d, want %d", fid, resp.Code, http.StatusNotFound)
		}
	}
}
