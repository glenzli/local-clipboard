package hub

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/g4i/local-clipboard/internal/proto"
)

const (
	historyCap  = 200
	fileTTL     = 24 * time.Hour
	cleanPeriod = time.Hour
	writeWait   = 10 * time.Second
	pingPeriod  = 30 * time.Second
	readWait    = 60 * time.Second
	maxMsgSize  = 32 << 20
	maxUpload   = 1 << 30
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
	instID   string
	storeDir string

	mu      sync.Mutex
	peers   map[*peer]struct{}
	history []*proto.Message
}

func New(token string) (*Hub, error) {
	root, err := rootDir()
	if err != nil {
		return nil, err
	}
	os.MkdirAll(filepath.Join(root, "files"), 0o755)
	id, err := loadOrCreateInstance(filepath.Join(root, "instance_id"))
	if err != nil {
		return nil, err
	}
	h := &Hub{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		token:    token,
		instID:   id,
		storeDir: filepath.Join(root, "files"),
		peers:    make(map[*peer]struct{}),
	}
	go h.cleanLoop()
	log.Printf("[hub] instance %s", short(id))
	return h, nil
}

func rootDir() (string, error) {
	if d := os.Getenv("LOCALCLIP_STORE"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return os.MkdirTemp("", "local-clipboard")
	}
	return filepath.Join(base, "local-clipboard"), nil
}

func loadOrCreateInstance(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		return string(b), nil
	}
	b := make([]byte, 16)
	rand.Read(b)
	id := hex.EncodeToString(b)
	return id, os.WriteFile(path, []byte(id), 0o600)
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
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
	mux.HandleFunc("/upload", h.handleUpload)
	mux.HandleFunc("/download", h.handleDownload)
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
	json.NewEncoder(w).Encode(map[string]any{
		"instance":    h.instID,
		"online":      names,
		"history_len": len(h.history),
	})
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

func (h *Hub) handleUpload(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "unnamed"
	}
	name = filepath.Base(name)
	fid := proto.RandomID() + "-" + name
	dst := filepath.Join(h.storeDir, fid)
	f, err := os.Create(dst)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	n, err := io.Copy(f, io.LimitReader(r.Body, maxUpload+1))
	f.Close()
	if err != nil || n > maxUpload {
		os.Remove(dst)
		http.Error(w, "upload failed or too large", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": fid})
	log.Printf("[hub] stored %s (%d bytes)", name, n)
}

func (h *Hub) handleDownload(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	fid := filepath.Base(r.URL.Query().Get("id"))
	path := filepath.Join(h.storeDir, fid)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	io.Copy(w, f)
	log.Printf("[hub] served %s", fid)
}

func (h *Hub) cleanLoop() {
	for range time.Tick(cleanPeriod) {
		entries, err := os.ReadDir(h.storeDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil || info.IsDir() {
				continue
			}
			if time.Since(info.ModTime()) > fileTTL {
				os.Remove(filepath.Join(h.storeDir, e.Name()))
			}
		}
	}
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

	hello, _ := json.Marshal(proto.Hello{Instance: h.instID})
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		conn.Close()
		return
	}

	h.mu.Lock()
	h.peers[p] = struct{}{}
	online := h.onlineNames()
	h.mu.Unlock()
	log.Printf("[hub] %s connected (%d online)", p.name, len(online))

	go h.writeLoop(p)
	h.readLoop(p)
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
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case proto.TypeText:
			if msg.Content == "" {
				continue
			}
		case proto.TypeImage:
			if msg.Content == "" {
				continue
			}
		case proto.TypeFile:
			if msg.FID == "" {
				continue
			}
		default:
			continue
		}

		h.mu.Lock()
		h.history = append(h.history, &msg)
		if len(h.history) > historyCap {
			h.history = h.history[len(h.history)-historyCap:]
		}
		targets := make([]*peer, 0, len(h.peers))
		for q := range h.peers {
			if q == p {
				continue
			}
			if msg.Target != "" && q.name != msg.Target {
				continue
			}
			targets = append(targets, q)
		}
		h.mu.Unlock()

		log.Printf("[hub] %s -> %d peers [%s]: %s", p.name, len(targets), msg.Type, describe(msg))
		for _, q := range targets {
			select {
			case q.send <- data:
			default:
				log.Printf("[hub] send buffer full for %s, dropping", q.name)
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

func describe(m proto.Message) string {
	switch m.Type {
	case proto.TypeFile:
		return m.Name + " (" + humanBytes(m.Size) + ") fid=" + m.FID
	default:
		s := m.Content
		r := []rune(s)
		if len(r) > 40 {
			return string(r[:40]) + "..."
		}
		return s
	}
}

func humanBytes(n int64) string {
	const kb, mb = 1<<10, 1<<20
	switch {
	case n >= mb:
		return itoa64(n/mb) + "." + itoa64((n%mb)/(1<<17)) + "MB"
	case n >= kb:
		return itoa64(n/kb) + "KB"
	default:
		return itoa64(n) + "B"
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
