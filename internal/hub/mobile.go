package hub

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/g4i/local-clipboard/internal/proto"
)

const (
	mobileSessionCookie = "localclip_session"
	mobileSessionTTL    = 30 * 24 * time.Hour
	mobileTrustedTTL    = 400 * 24 * time.Hour
	maxMobileText       = 1 << 20
	maxMobileJSON       = maxMobileText + (8 << 10)
	maxMobileImage      = 20 << 20
	maxMobileImageJSON  = (maxMobileImage * 4 / 3) + (128 << 10)
	maxInboxItems       = 50
	maxInboxPreview     = 240
)

//go:embed web/*
var mobileWeb embed.FS

type mobileInboxItem struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	TS      int64  `json:"ts"`
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Size    int64  `json:"size,omitempty"`
	FID     string `json:"fid,omitempty"`
	Target  string `json:"target,omitempty"`
	Preview string `json:"preview,omitempty"`
}

func (h *Hub) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		h.handleStatus(w, r)
		return
	}
	h.serveMobileFile(w, "index.html", false)
}

func (h *Hub) handleMobileAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/mobile/")
	if name == "" || path.Clean("/"+name) != "/"+name {
		http.NotFound(w, r)
		return
	}
	h.serveMobileFile(w, name, true)
}

func (h *Hub) serveMobileFile(w http.ResponseWriter, name string, cache bool) {
	data, err := mobileWeb.ReadFile("web/" + name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if filepath.Ext(name) == ".webmanifest" {
		contentType = "application/manifest+json"
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if cache {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob: data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
	}
	w.Write(data)
}

func (h *Hub) handleMobileSession(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "cross-origin session request denied", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			Token  string `json:"token"`
			Trust  bool   `json:"trust"`
			Device string `json:"device"`
		}
		if !decodeJSON(w, r, 8<<10, &req) {
			return
		}
		if h.token != "" && subtle.ConstantTimeCompare([]byte(req.Token), []byte(h.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, err := randomMobileSessionID()
		if err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		now := time.Now()
		expires := now.Add(mobileSessionTTL)
		if req.Trust {
			expires = now.Add(mobileTrustedTTL)
		}
		key := h.mobileCredentialKey(id)
		if req.Trust {
			if err := h.mobileTrust.remember(key, req.Device, now, expires); err != nil {
				http.Error(w, "could not trust this device", http.StatusInternalServerError)
				return
			}
		}
		h.mobileMu.Lock()
		if h.mobileSessions == nil {
			h.mobileSessions = make(map[string]time.Time)
		}
		h.mobileSessions[key] = expires
		h.mobileMu.Unlock()
		http.SetCookie(w, mobileCookie(id, expires, secureRequest(r), req.Trust))
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "trusted": req.Trust})

	case http.MethodDelete:
		if cookie, err := r.Cookie(mobileSessionCookie); err == nil {
			key := h.mobileCredentialKey(cookie.Value)
			if err := h.mobileTrust.forget(key, time.Now()); err != nil {
				http.Error(w, "could not forget this device", http.StatusInternalServerError)
				return
			}
			h.mobileMu.Lock()
			delete(h.mobileSessions, key)
			h.mobileMu.Unlock()
		}
		http.SetCookie(w, mobileCookie("", time.Unix(1, 0), secureRequest(r), true))
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", http.MethodPost+", "+http.MethodDelete)
		http.Error(w, "POST or DELETE required", http.StatusMethodNotAllowed)
	}
}

func (h *Hub) mobileSessionAuthorized(r *http.Request) bool {
	cookie, err := r.Cookie(mobileSessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	key := h.mobileCredentialKey(cookie.Value)
	now := time.Now()
	h.mobileMu.Lock()
	expires, ok := h.mobileSessions[key]
	if ok && now.After(expires) {
		delete(h.mobileSessions, key)
		ok = false
	}
	if ok {
		h.mobileSessions[key] = now.Add(mobileSessionTTL)
	}
	h.mobileMu.Unlock()
	return ok || h.mobileTrust.valid(key, now)
}

func (h *Hub) mobileCredentialKey(value string) string {
	mac := hmac.New(sha256.New, []byte(h.token))
	mac.Write([]byte("local-clipboard/mobile-device/v1\x00"))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func mobileCookie(value string, expires time.Time, secure, persistent bool) *http.Cookie {
	maxAge := 0
	if persistent {
		maxAge = int(time.Until(expires).Seconds())
	}
	if value == "" {
		maxAge = -1
	}
	cookie := &http.Cookie{
		Name:     mobileSessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
	if persistent || value == "" {
		cookie.Expires = expires
	}
	return cookie
}

func randomMobileSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

func (h *Hub) requireMobileRequest(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, method+" required", http.StatusMethodNotAllowed)
		return false
	}
	if method != http.MethodGet && !sameOrigin(r) {
		http.Error(w, "cross-origin request denied", http.StatusForbidden)
		return false
	}
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *Hub) handleMobileInbox(w http.ResponseWriter, r *http.Request) {
	if !h.requireMobileRequest(w, r, http.MethodGet) {
		return
	}
	device := normalizeMobileName(r.URL.Query().Get("device"), "Mobile Web")

	h.mu.Lock()
	items := make([]mobileInboxItem, 0, maxInboxItems)
	for i := len(h.history) - 1; i >= 0 && len(items) < maxInboxItems; i-- {
		msg := h.history[i].message
		if !visibleToMobile(msg, device) {
			continue
		}
		item := mobileInboxItem{
			ID:     msg.ID,
			Source: msg.Source,
			TS:     msg.TS,
			Type:   msg.Type,
			Name:   msg.Name,
			Size:   msg.Size,
			FID:    msg.FID,
			Target: msg.Target,
		}
		if msg.Type == proto.TypeText {
			item.Preview = truncateRunes(msg.Content, maxInboxPreview)
		}
		items = append(items, item)
	}
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, items)
}

func (h *Hub) handleMobileItem(w http.ResponseWriter, r *http.Request) {
	if !h.requireMobileRequest(w, r, http.MethodGet) {
		return
	}
	id := r.URL.Query().Get("id")
	device := normalizeMobileName(r.URL.Query().Get("device"), "Mobile Web")
	if id == "" {
		http.Error(w, "missing item id", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	var found *proto.Message
	for i := len(h.history) - 1; i >= 0; i-- {
		msg := h.history[i].message
		if msg.ID == id && visibleToMobile(msg, device) {
			copy := msg
			found = &copy
			break
		}
	}
	h.mu.Unlock()
	if found == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, found)
}

func visibleToMobile(msg proto.Message, device string) bool {
	if msg.Source == device {
		return false
	}
	return msg.Target == "" || msg.Target == device
}

func (h *Hub) handleMobileText(w http.ResponseWriter, r *http.Request) {
	if !h.requireMobileRequest(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Text   string `json:"text"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if !decodeJSON(w, r, maxMobileJSON, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is empty", http.StatusBadRequest)
		return
	}
	if len(req.Text) > maxMobileText {
		http.Error(w, "text is too large", http.StatusRequestEntityTooLarge)
		return
	}

	msg := proto.NewMessage(normalizeMobileName(req.Source, "Mobile Web"), proto.TypeText)
	msg.Content = req.Text
	msg.Size = int64(len(req.Text))
	msg.Target = normalizeMobileName(req.Target, "")
	if strings.TrimSpace(req.Target) != "" && msg.Target == "" {
		http.Error(w, "invalid target device", http.StatusBadRequest)
		return
	}
	h.publishMobileMessage(w, msg)
}

func (h *Hub) handleMobileImage(w http.ResponseWriter, r *http.Request) {
	if !h.requireMobileRequest(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Content string `json:"content"`
		Name    string `json:"name"`
		Source  string `json:"source"`
		Target  string `json:"target"`
	}
	if !decodeJSON(w, r, maxMobileImageJSON, &req) {
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil || len(data) == 0 {
		http.Error(w, "invalid image data", http.StatusBadRequest)
		return
	}
	if len(data) > maxMobileImage {
		http.Error(w, "image is too large", http.StatusRequestEntityTooLarge)
		return
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		http.Error(w, "image must be PNG", http.StatusBadRequest)
		return
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 16384 || config.Height > 16384 || int64(config.Width)*int64(config.Height) > 100_000_000 {
		http.Error(w, "image dimensions are too large", http.StatusRequestEntityTooLarge)
		return
	}

	name := filepath.Base(strings.TrimSpace(req.Name))
	if name == "." || name == "" {
		name = "mobile-image.png"
	}
	name = strings.TrimSuffix(name, filepath.Ext(name)) + ".png"
	msg := proto.NewMessage(normalizeMobileName(req.Source, "Mobile Web"), proto.TypeImage)
	msg.Name = name
	msg.Content = req.Content
	msg.Size = int64(len(data))
	msg.Target = normalizeMobileName(req.Target, "")
	if strings.TrimSpace(req.Target) != "" && msg.Target == "" {
		http.Error(w, "invalid target device", http.StatusBadRequest)
		return
	}
	h.publishMobileMessage(w, msg)
}

func (h *Hub) handleMobileFile(w http.ResponseWriter, r *http.Request) {
	if !h.requireMobileRequest(w, r, http.MethodPost) {
		return
	}
	var req struct {
		FID    string `json:"fid"`
		Name   string `json:"name"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if !decodeJSON(w, r, 64<<10, &req) {
		return
	}
	fid := filepath.Base(req.FID)
	if fid == "" || fid != req.FID {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(filepath.Join(h.storeDir, fid))
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "uploaded file not found", http.StatusNotFound)
		return
	}
	name := filepath.Base(strings.TrimSpace(req.Name))
	if name == "." || name == "" {
		if i := strings.Index(fid, "-"); i >= 0 && len(fid) > i+1 {
			name = fid[i+1:]
		} else {
			name = "file"
		}
	}

	msg := proto.NewMessage(normalizeMobileName(req.Source, "Mobile Web"), proto.TypeFile)
	msg.Name = name
	msg.Size = info.Size()
	msg.FID = fid
	msg.Target = normalizeMobileName(req.Target, "")
	if strings.TrimSpace(req.Target) != "" && msg.Target == "" {
		http.Error(w, "invalid target device", http.StatusBadRequest)
		return
	}
	h.publishMobileMessage(w, msg)
}

func (h *Hub) publishMobileMessage(w http.ResponseWriter, msg *proto.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "could not encode message", http.StatusInternalServerError)
		return
	}
	result := h.forwardMessage(*msg, data, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        msg.ID,
		"matched":   result.matched,
		"delivered": result.delivered,
	})
}

func normalizeMobileName(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	var b strings.Builder
	count := 0
	for _, r := range value {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		count++
		if count >= 64 {
			break
		}
	}
	value = strings.TrimSpace(b.String())
	if value == "" {
		return fallback
	}
	return value
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
