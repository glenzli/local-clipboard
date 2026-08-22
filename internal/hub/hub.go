package hub

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/g4i/local-clipboard/internal/proto"
)

const (
	historyCap  = 200
	writeWait   = 10 * time.Second
	pingPeriod  = 30 * time.Second
	maxMsgSize  = 16 << 20
	sendBufSize = 32
)

type peer struct {
	conn *websocket.Conn
	name string
	send chan []byte
}

type Hub struct {
	upgrader websocket.Upgrader
	token    string
	mu       sync.Mutex
	peers    map[*peer]struct{}
	history  []*proto.Message
}

func New(token string) *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		token:    token,
		peers:    make(map[*peer]struct{}),
	}
}

func (h *Hub) authorized(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	t := r.URL.Query().Get("token")
	if t == "" {
		t = r.Header.Get("X-Token")
	}
	return subtle.ConstantTimeCompare([]byte(t), []byte(h.token)) == 1
}

func (h *Hub) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/ws", h.handleWS)
	mux.HandleFunc("/history", h.handleHistory)
	mux.HandleFunc("/", h.handleStatus)
}

func (h *Hub) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	names := make([]string, 0, len(h.peers))
	for p := range h.peers {
		names = append(names, p.name)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"online": names, "history_len": len(h.history)})
}

func (h *Hub) handleHistory(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*proto.Message, len(h.history))
	for i, m := range h.history {
		out[len(h.history)-1-i] = m
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[hub] upgrade: %v", err)
		return
	}
	conn.SetReadLimit(maxMsgSize)

	p := &peer{conn: conn, name: r.URL.Query().Get("name"), send: make(chan []byte, sendBufSize)}
	if p.name == "" {
		p.name = "unknown"
	}

	h.mu.Lock()
	last := h.latestFromOthers(p.name)
	h.peers[p] = struct{}{}
	online := h.onlineNames()
	h.mu.Unlock()

	log.Printf("[hub] %s connected (%d online)", p.name, len(online))

	go h.writeLoop(p)

	if last != nil {
		data, _ := json.Marshal(last)
		p.send <- data
	}

	h.readLoop(p)
}

func (h *Hub) latestFromOthers(name string) *proto.Message {
	for i := len(h.history) - 1; i >= 0; i-- {
		if h.history[i].Source != name {
			return h.history[i]
		}
	}
	return nil
}

func (h *Hub) onlineNames() []string {
	names := make([]string, 0, len(h.peers))
	for p := range h.peers {
		names = append(names, p.name)
	}
	return names
}

func (h *Hub) readLoop(p *peer) {
	defer func() {
		h.mu.Lock()
		delete(h.peers, p)
		h.mu.Unlock()
		p.conn.Close()
		close(p.send)
		log.Printf("[hub] %s disconnected", p.name)
	}()

	for {
		_, data, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg proto.Message
		if json.Unmarshal(data, &msg) != nil || msg.Content == "" {
			continue
		}

		h.mu.Lock()
		h.history = append(h.history, &msg)
		if len(h.history) > historyCap {
			h.history = h.history[len(h.history)-historyCap:]
		}
		targets := make([]*peer, 0, len(h.peers))
		for q := range h.peers {
			if q != p {
				targets = append(targets, q)
			}
		}
		h.mu.Unlock()

		log.Printf("[hub] %s -> %d peers: %q", p.name, len(targets), truncate(msg.Content))
		for _, q := range targets {
			select {
			case q.send <- data:
			default:
				log.Printf("[hub] %s send buffer full, dropping message for %s", p.name, q.name)
			}
		}
	}
}

func (h *Hub) writeLoop(p *peer) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case data, ok := <-p.send:
			p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				p.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := p.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) > 40 {
		return string(r[:40]) + "..."
	}
	return s
}
