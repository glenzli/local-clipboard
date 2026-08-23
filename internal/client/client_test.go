package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/g4i/local-clipboard/internal/proto"
)

func TestHandleTextRecordsClipboardResult(t *testing.T) {
	tests := []struct {
		name      string
		writeErr  error
		wantState string
	}{
		{name: "applied", wantState: StateApplied},
		{name: "failed", writeErr: errors.New("write failed"), wantState: StateFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New("hub:9100", "mac", "token")
			var written string
			c.writeText = func(text string) error {
				written = text
				return tt.writeErr
			}
			c.handleText(proto.Message{Source: "windows", Type: proto.TypeText, Content: "中文"})

			if written != "中文" {
				t.Fatalf("written text = %q", written)
			}
			events := snapshotEvents(t, c)
			if len(events) != 1 {
				t.Fatalf("event count = %d, want 1", len(events))
			}
			assertClipboardEvent(t, events[0], proto.TypeText, "windows", int64(len("中文")), tt.wantState)
		})
	}
}

func TestHandleImageRecordsClipboardResult(t *testing.T) {
	c := New("hub:9100", "mac", "token")
	want := []byte{0x89, 'P', 'N', 'G'}
	c.writeImage = func(data []byte) error {
		if string(data) != string(want) {
			t.Fatalf("written image = %v, want %v", data, want)
		}
		return nil
	}
	c.handleImage(proto.Message{Source: "windows", Type: proto.TypeImage, Content: encodeBase64(want), Size: int64(len(want))})

	events := snapshotEvents(t, c)
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	assertClipboardEvent(t, events[0], proto.TypeImage, "windows", int64(len(want)), StateApplied)
}

func snapshotEvents(t *testing.T, c *Client) []*Event {
	t.Helper()
	events, ok := c.store.Snapshot(true)["events"].([]*Event)
	if !ok {
		t.Fatal("snapshot events have unexpected type")
	}
	return events
}

func assertClipboardEvent(t *testing.T, event *Event, kind, peer string, size int64, state string) {
	t.Helper()
	if event.Dir != "in" || event.Kind != kind || event.Peer != peer || event.Size != size || event.State != state {
		t.Fatalf("event = %+v, want in/%s/%s/%d/%s", event, kind, peer, size, state)
	}
}

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
