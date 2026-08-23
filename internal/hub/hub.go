package hub

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/g4i/local-clipboard/internal/proto"
)

const (
	historyCap      = 200
	historyBytesCap = 64 << 20
	fileTTL         = 24 * time.Hour
	cleanPeriod     = time.Hour
	writeWait       = 10 * time.Second
	pingPeriod      = 30 * time.Second
	readWait        = 60 * time.Second
	maxMsgSize      = 32 << 20
	maxUpload       = 1 << 30
	sendBufSize     = 32
)

type peer struct {
	conn      *websocket.Conn
	name      string
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func (p *peer) close() {
	p.closeOnce.Do(func() {
		close(p.done)
		if p.conn != nil {
			p.conn.Close()
		}
	})
}

func (p *peer) enqueue(data []byte) bool {
	select {
	case <-p.done:
		return false
	default:
	}
	select {
	case <-p.done:
		return false
	case p.send <- data:
		return true
	default:
		return false
	}
}

type historyEntry struct {
	message  proto.Message
	wireSize int64
}

type Hub struct {
	upgrader websocket.Upgrader
	token    string
	instID   string
	storeDir string

	mu           sync.Mutex
	peers        map[*peer]struct{}
	history      []historyEntry
	historyBytes int64

	mobileMu       sync.Mutex
	mobileSessions map[string]time.Time
	mobileTrust    *mobileTrustStore
}

func New(token string) (*Hub, error) {
	root, err := rootDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
		return nil, err
	}
	id, err := loadOrCreateInstance(filepath.Join(root, "instance_id"))
	if err != nil {
		return nil, err
	}
	mobileTrust, err := loadMobileTrustStore(filepath.Join(root, mobileTrustFile))
	if err != nil {
		log.Printf("[hub] trusted mobile devices unavailable; requiring token again: %v", err)
		mobileTrust = emptyMobileTrustStore(filepath.Join(root, mobileTrustFile))
	}
	h := &Hub{
		upgrader:       websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		token:          token,
		instID:         id,
		storeDir:       filepath.Join(root, "files"),
		peers:          make(map[*peer]struct{}),
		mobileSessions: make(map[string]time.Time),
		mobileTrust:    mobileTrust,
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
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
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
	if h.mobileSessionAuthorized(r) {
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
	mux.HandleFunc("/api/status", h.handleStatus)
	mux.HandleFunc("/api/session", h.handleMobileSession)
	mux.HandleFunc("/api/inbox", h.handleMobileInbox)
	mux.HandleFunc("/api/item", h.handleMobileItem)
	mux.HandleFunc("/api/send-text", h.handleMobileText)
	mux.HandleFunc("/api/send-file", h.handleMobileFile)
	mux.HandleFunc("/mobile/", h.handleMobileAsset)
	mux.HandleFunc("/", h.handleRoot)
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
	sort.Strings(names)
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
	out := make([]proto.Message, len(h.history))
	for i, entry := range h.history {
		out[len(h.history)-1-i] = entry.message
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
	if !sameOrigin(r) {
		http.Error(w, "cross-origin upload denied", http.StatusForbidden)
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
	closeErr := f.Close()
	if err != nil || closeErr != nil || n > maxUpload {
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
	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "could not inspect file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	name := fid
	if i := strings.Index(fid, "-"); i >= 0 && len(fid) > i+1 {
		name = fid[i+1:]
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
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

	p := &peer{
		conn: conn,
		name: r.URL.Query().Get("name"),
		send: make(chan []byte, sendBufSize),
		done: make(chan struct{}),
	}
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
		p.close()
		log.Printf("[hub] %s disconnected", p.name)
	}()

	for {
		_, data, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		msg, data, ok := normalizeIncomingMessage(data, p.name)
		if !ok {
			continue
		}

		h.forwardMessage(msg, data, p)
	}
}

type deliveryResult struct {
	matched   int
	delivered int
}

func (h *Hub) forwardMessage(msg proto.Message, data []byte, exclude *peer) deliveryResult {
	h.mu.Lock()
	h.appendHistoryLocked(msg, int64(len(data)))
	targets := make([]*peer, 0, len(h.peers))
	for q := range h.peers {
		if q == exclude {
			continue
		}
		if msg.Target != "" && q.name != msg.Target {
			continue
		}
		targets = append(targets, q)
	}
	h.mu.Unlock()

	result := deliveryResult{matched: len(targets)}
	log.Printf("[hub] %s -> %d peers [%s]: %s", msg.Source, len(targets), msg.Type, describe(msg))
	for _, q := range targets {
		if q.enqueue(data) {
			result.delivered++
			continue
		}
		log.Printf("[hub] peer unavailable or send buffer full for %s, dropping", q.name)
	}
	return result
}

func normalizeIncomingMessage(data []byte, source string) (proto.Message, []byte, bool) {
	var msg proto.Message
	if json.Unmarshal(data, &msg) != nil {
		return proto.Message{}, nil, false
	}
	switch msg.Type {
	case proto.TypeText, proto.TypeImage:
		if msg.Content == "" {
			return proto.Message{}, nil, false
		}
	case proto.TypeFile:
		if msg.FID == "" {
			return proto.Message{}, nil, false
		}
	default:
		return proto.Message{}, nil, false
	}

	// The WebSocket connection owns source attribution. Never forward a
	// conflicting Source supplied in the message body.
	msg.Source = source
	normalized, err := json.Marshal(msg)
	if err != nil {
		return proto.Message{}, nil, false
	}
	return msg, normalized, true
}

func (h *Hub) appendHistoryLocked(msg proto.Message, wireSize int64) {
	h.history = append(h.history, historyEntry{message: msg, wireSize: wireSize})
	h.historyBytes += wireSize
	for len(h.history) > 1 && (len(h.history) > historyCap || h.historyBytes > historyBytesCap) {
		h.historyBytes -= h.history[0].wireSize
		h.history = h.history[1:]
	}
}

func (h *Hub) writeLoop(p *peer) {
	defer p.close()
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case data, ok := <-p.send:
			p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
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
	const kb, mb = 1 << 10, 1 << 20
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
