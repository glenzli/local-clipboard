package hub

import (
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
