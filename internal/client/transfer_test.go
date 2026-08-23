package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadFileUsesTokenHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertTokenHeader(t, r)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upload body: %v", err)
			http.Error(w, "read failed", http.StatusInternalServerError)
			return
		}
		if string(body) != "payload" {
			t.Errorf("body = %q, want payload", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"file-id"}`)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, size, err := UploadFile(strings.TrimPrefix(server.URL, "http://"), "secret", path, "test.txt", nil)
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if id != "file-id" || size != 7 {
		t.Fatalf("id = %q, size = %d", id, size)
	}
}

func TestDownloadToUsesTokenHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertTokenHeader(t, r)
		io.WriteString(w, "payload")
	}))
	defer server.Close()

	outDir := t.TempDir()
	path, err := DownloadTo(strings.TrimPrefix(server.URL, "http://"), "secret", "id-test.txt", outDir, nil)
	if err != nil {
		t.Fatalf("DownloadTo: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatalf("download = %q, want payload", data)
	}
}

func assertTokenHeader(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("X-Token"); got != "secret" {
		t.Errorf("X-Token = %q, want secret", got)
	}
	if got := r.URL.Query().Get("token"); got != "" {
		t.Errorf("token query = %q, want empty", got)
	}
}
