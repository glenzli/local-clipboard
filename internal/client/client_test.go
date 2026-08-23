package client

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestLoadKnownHostsFailsClosedOnCorruptFile(t *testing.T) {
	for _, content := range []string{`{"broken"`, `null`} {
		path := t.TempDir() + "/known_hosts.json"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadKnownHosts(path); err == nil {
			t.Fatalf("invalid known-hosts content was accepted: %s", content)
		}
	}
}

func TestDialWSUsesTokenHeader(t *testing.T) {
	requestChecked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Token"); got != "secret" {
			t.Errorf("X-Token = %q, want secret", got)
		}
		if got := r.URL.Query().Get("token"); got != "" {
			t.Errorf("token query = %q, want empty", got)
		}
		requestChecked <- struct{}{}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			conn.Close()
		}
	}))
	defer server.Close()

	c := New(strings.TrimPrefix(server.URL, "http://"), "test-device", "secret")
	conn, err := c.dialWS()
	if err != nil {
		t.Fatalf("dialWS: %v", err)
	}
	conn.Close()
	<-requestChecked
}
