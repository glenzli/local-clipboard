package hub

import (
	"encoding/json"
	"testing"

	"github.com/g4i/local-clipboard/internal/proto"
)

func TestClosedPeerRejectsStaleProducer(t *testing.T) {
	p := &peer{send: make(chan []byte, 1), done: make(chan struct{})}
	p.close()
	p.close()
	if p.enqueue([]byte("late message")) {
		t.Fatal("closed peer accepted a message")
	}
}

func TestNormalizeIncomingMessageOwnsSourceAtConnection(t *testing.T) {
	in := proto.NewMessage("spoofed-device", proto.TypeText)
	in.Content = "hello"
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	msg, forwarded, ok := normalizeIncomingMessage(data, "connected-device")
	if !ok {
		t.Fatal("valid message was rejected")
	}
	if msg.Source != "connected-device" {
		t.Fatalf("source = %q, want connected-device", msg.Source)
	}
	var decoded proto.Message
	if err := json.Unmarshal(forwarded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Source != "connected-device" {
		t.Fatalf("forwarded source = %q, want connected-device", decoded.Source)
	}
}

func TestNormalizeIncomingMessageRejectsInvalidPayload(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`not json`),
		[]byte(`{"type":"text"}`),
		[]byte(`{"type":"file"}`),
		[]byte(`{"type":"unknown","content":"value"}`),
	} {
		if _, _, ok := normalizeIncomingMessage(data, "device"); ok {
			t.Fatalf("accepted invalid payload: %s", data)
		}
	}
}

func TestHistoryIsBoundedByWireBytes(t *testing.T) {
	h := &Hub{}
	for i := 0; i < 3; i++ {
		h.appendHistoryLocked(*proto.NewMessage("test", proto.TypeImage), 30<<20)
	}
	if got := len(h.history); got != 2 {
		t.Fatalf("history length = %d, want 2", got)
	}
	if h.historyBytes > historyBytesCap {
		t.Fatalf("history bytes = %d, cap = %d", h.historyBytes, historyBytesCap)
	}
}
