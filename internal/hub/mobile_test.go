package hub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/g4i/local-clipboard/internal/proto"
)

func TestMobileWebAndStatusCompatibility(t *testing.T) {
	h := newMobileTestHub(t, "secret")
	mux := http.NewServeMux()
	h.Routes(mux)

	pageReq := httptest.NewRequest(http.MethodGet, "http://hub.local/", nil)
	pageReq.Header.Set("Accept", "text/html")
	pageResp := httptest.NewRecorder()
	mux.ServeHTTP(pageResp, pageReq)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("mobile page status = %d, want %d", pageResp.Code, http.StatusOK)
	}
	if csp := pageResp.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("mobile page Content-Security-Policy = %q", csp)
	}
	for _, fragment := range []string{`/mobile/app.css`, `/mobile/app.js`, `/mobile/manifest.webmanifest`, `id="installBanner"`, `id="trustDevice"`, `id="sendPanel"`, `id="sendImageBtn"`, `id="receivePanel"`} {
		if !strings.Contains(pageResp.Body.String(), fragment) {
			t.Fatalf("mobile page missing %q", fragment)
		}
	}

	assetReq := httptest.NewRequest(http.MethodGet, "http://hub.local/mobile/app.js", nil)
	assetResp := httptest.NewRecorder()
	mux.ServeHTTP(assetResp, assetReq)
	if assetResp.Code != http.StatusOK || !strings.Contains(assetResp.Body.String(), "sendImageToClipboard") || !strings.Contains(assetResp.Body.String(), "/api/send-image") || !strings.Contains(assetResp.Body.String(), "forgetFailed") {
		t.Fatalf("mobile JavaScript asset was not served: status=%d", assetResp.Code)
	}

	manifestReq := httptest.NewRequest(http.MethodGet, "http://hub.local/mobile/manifest.webmanifest", nil)
	manifestResp := httptest.NewRecorder()
	mux.ServeHTTP(manifestResp, manifestReq)
	if manifestResp.Code != http.StatusOK || !strings.Contains(manifestResp.Header().Get("Content-Type"), "manifest") {
		t.Fatalf("web app manifest response: status=%d content-type=%q", manifestResp.Code, manifestResp.Header().Get("Content-Type"))
	}
	var manifest struct {
		StartURL string `json:"start_url"`
		Scope    string `json:"scope"`
		Display  string `json:"display"`
		Icons    []struct {
			Source string `json:"src"`
			Sizes  string `json:"sizes"`
		} `json:"icons"`
	}
	if err := json.NewDecoder(manifestResp.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.StartURL != "/" || manifest.Scope != "/" || manifest.Display != "standalone" || len(manifest.Icons) != 2 {
		t.Fatalf("unexpected web app manifest: %+v", manifest)
	}

	for _, icon := range []struct {
		path string
		size int
	}{{"/mobile/icon-192.png", 192}, {"/mobile/icon-512.png", 512}} {
		iconReq := httptest.NewRequest(http.MethodGet, "http://hub.local"+icon.path, nil)
		iconResp := httptest.NewRecorder()
		mux.ServeHTTP(iconResp, iconReq)
		config, err := png.DecodeConfig(bytes.NewReader(iconResp.Body.Bytes()))
		if iconResp.Code != http.StatusOK || err != nil || config.Width != icon.size || config.Height != icon.size {
			t.Fatalf("PWA icon %s: status=%d size=%dx%d err=%v", icon.path, iconResp.Code, config.Width, config.Height, err)
		}
	}

	statusReq := httptest.NewRequest(http.MethodGet, "http://hub.local/", nil)
	statusReq.Header.Set("X-Token", "secret")
	statusResp := httptest.NewRecorder()
	mux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK || !strings.Contains(statusResp.Body.String(), `"instance":"test-instance"`) {
		t.Fatalf("root JSON compatibility failed: status=%d body=%s", statusResp.Code, statusResp.Body.String())
	}
}

func TestMobileSessionAuthenticatesWithoutExposingSharedToken(t *testing.T) {
	h := newMobileTestHub(t, "secret")
	mux := http.NewServeMux()
	h.Routes(mux)

	wrongReq := mobileJSONRequest(http.MethodPost, "/api/session", `{"token":"wrong"}`)
	wrongResp := httptest.NewRecorder()
	mux.ServeHTTP(wrongResp, wrongReq)
	if wrongResp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want %d", wrongResp.Code, http.StatusUnauthorized)
	}

	loginReq := mobileJSONRequest(http.MethodPost, "/api/session", `{"token":"secret"}`)
	loginResp := httptest.NewRecorder()
	mux.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", loginResp.Code, http.StatusOK, loginResp.Body.String())
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != mobileSessionCookie || !cookies[0].HttpOnly {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
	if strings.Contains(cookies[0].Value, "secret") {
		t.Fatal("session cookie exposes the shared token")
	}

	statusReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/status", nil)
	statusReq.AddCookie(cookies[0])
	statusResp := httptest.NewRecorder()
	mux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("session-authenticated status = %d, want %d", statusResp.Code, http.StatusOK)
	}

	crossOriginReq := mobileJSONRequest(http.MethodPost, "/api/send-text", `{"text":"hello","source":"Phone"}`)
	crossOriginReq.Header.Set("Origin", "https://evil.example")
	crossOriginReq.AddCookie(cookies[0])
	crossOriginResp := httptest.NewRecorder()
	mux.ServeHTTP(crossOriginResp, crossOriginReq)
	if crossOriginResp.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation status = %d, want %d", crossOriginResp.Code, http.StatusForbidden)
	}
}

func TestMobileTextTargetsDesktopAndAppearsInHistory(t *testing.T) {
	h := newMobileTestHub(t, "secret")
	desktop := testPeer("Desktop")
	other := testPeer("Other")
	h.peers[desktop] = struct{}{}
	h.peers[other] = struct{}{}

	req := mobileJSONRequest(http.MethodPost, "/api/send-text", `{"text":"你好","source":"Phone","target":"Desktop"}`)
	req.Header.Set("X-Token", "secret")
	resp := httptest.NewRecorder()
	h.handleMobileText(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("send text status = %d, want %d: %s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var result struct {
		Matched   int `json:"matched"`
		Delivered int `json:"delivered"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Matched != 1 || result.Delivered != 1 {
		t.Fatalf("delivery result = %+v, want matched=1 delivered=1", result)
	}

	select {
	case data := <-desktop.send:
		var msg proto.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Source != "Phone" || msg.Target != "Desktop" || msg.Type != proto.TypeText || msg.Content != "你好" {
			t.Fatalf("forwarded message = %+v", msg)
		}
	default:
		t.Fatal("target desktop did not receive the text")
	}
	if len(other.send) != 0 {
		t.Fatal("non-target desktop received the text")
	}
	if len(h.history) != 1 || h.history[0].message.Source != "Phone" {
		t.Fatalf("history = %+v", h.history)
	}

	invalidReq := mobileJSONRequest(http.MethodPost, "/api/send-text", `{"text":"must not broadcast","target":"\u0001"}`)
	invalidReq.Header.Set("X-Token", "secret")
	invalidResp := httptest.NewRecorder()
	h.handleMobileText(invalidResp, invalidReq)
	if invalidResp.Code != http.StatusBadRequest || len(h.history) != 1 {
		t.Fatalf("invalid target status=%d history=%d", invalidResp.Code, len(h.history))
	}
}

func TestMobileImageTargetsDesktopClipboard(t *testing.T) {
	h := newMobileTestHub(t, "secret")
	desktop := testPeer("Desktop")
	h.peers[desktop] = struct{}{}

	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 120, G: 167, B: 207, A: 255})
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{
		"content": base64.StdEncoding.EncodeToString(imageData.Bytes()),
		"name":    "手机照片.heic",
		"source":  "Phone",
		"target":  "Desktop",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := mobileJSONRequest(http.MethodPost, "/api/send-image", string(body))
	req.Header.Set("X-Token", "secret")
	resp := httptest.NewRecorder()
	h.handleMobileImage(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("send image status = %d, want %d: %s", resp.Code, http.StatusOK, resp.Body.String())
	}

	select {
	case data := <-desktop.send:
		var msg proto.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type != proto.TypeImage || msg.Name != "手机照片.png" || msg.Source != "Phone" || msg.Target != "Desktop" {
			t.Fatalf("forwarded image metadata = %+v", msg)
		}
		decoded, err := base64.StdEncoding.DecodeString(msg.Content)
		if err != nil || !bytes.Equal(decoded, imageData.Bytes()) || msg.Size != int64(imageData.Len()) {
			t.Fatalf("forwarded image bytes=%d size=%d err=%v", len(decoded), msg.Size, err)
		}
	default:
		t.Fatal("target desktop did not receive the image")
	}
	if len(h.history) != 1 || h.history[0].message.Type != proto.TypeImage {
		t.Fatalf("history = %+v", h.history)
	}

	badReq := mobileJSONRequest(http.MethodPost, "/api/send-image", `{"content":"bm90IGEgcG5n","name":"bad.png"}`)
	badReq.Header.Set("X-Token", "secret")
	badResp := httptest.NewRecorder()
	h.handleMobileImage(badResp, badReq)
	if badResp.Code != http.StatusBadRequest || len(h.history) != 1 {
		t.Fatalf("invalid image status=%d history=%d", badResp.Code, len(h.history))
	}
}

func TestMobileInboxFiltersAndLoadsExactItem(t *testing.T) {
	h := newMobileTestHub(t, "secret")
	wanted := proto.NewMessage("Desktop", proto.TypeText)
	wanted.Content = "完整的中文文本"
	wanted.Size = int64(len(wanted.Content))
	wanted.Target = "Phone"
	h.forwardMessage(*wanted, mustJSON(t, wanted), nil)

	other := proto.NewMessage("Desktop", proto.TypeText)
	other.Content = "private"
	other.Target = "Other Phone"
	h.forwardMessage(*other, mustJSON(t, other), nil)

	own := proto.NewMessage("Phone", proto.TypeText)
	own.Content = "sent by this phone"
	h.forwardMessage(*own, mustJSON(t, own), nil)

	inboxReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/inbox?device=Phone", nil)
	inboxReq.Header.Set("X-Token", "secret")
	inboxResp := httptest.NewRecorder()
	h.handleMobileInbox(inboxResp, inboxReq)
	if inboxResp.Code != http.StatusOK {
		t.Fatalf("inbox status = %d, want %d", inboxResp.Code, http.StatusOK)
	}
	var items []mobileInboxItem
	if err := json.NewDecoder(inboxResp.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != wanted.ID || items[0].Preview != wanted.Content {
		t.Fatalf("inbox items = %+v", items)
	}

	itemReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/item?id="+wanted.ID+"&device=Phone", nil)
	itemReq.Header.Set("X-Token", "secret")
	itemResp := httptest.NewRecorder()
	h.handleMobileItem(itemResp, itemReq)
	var full proto.Message
	if itemResp.Code != http.StatusOK || json.NewDecoder(itemResp.Body).Decode(&full) != nil {
		t.Fatalf("item response status=%d body=%s", itemResp.Code, itemResp.Body.String())
	}
	if full.Content != wanted.Content {
		t.Fatalf("item content = %q, want %q", full.Content, wanted.Content)
	}
}

func TestMobileFileUploadAnnounceAndCookieDownload(t *testing.T) {
	h := newMobileTestHub(t, "secret")
	desktop := testPeer("Desktop")
	h.peers[desktop] = struct{}{}
	mux := http.NewServeMux()
	h.Routes(mux)
	cookie := loginMobile(t, mux, "secret")

	uploadReq := httptest.NewRequest(http.MethodPost, "http://hub.local/upload?name=报告.txt", strings.NewReader("mobile file"))
	uploadReq.Header.Set("Origin", "http://hub.local")
	uploadReq.AddCookie(cookie)
	uploadResp := httptest.NewRecorder()
	mux.ServeHTTP(uploadResp, uploadReq)
	if uploadResp.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want %d: %s", uploadResp.Code, http.StatusOK, uploadResp.Body.String())
	}
	var upload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&upload); err != nil {
		t.Fatal(err)
	}

	announceReq := mobileJSONRequest(http.MethodPost, "/api/send-file", `{"fid":"`+upload.ID+`","name":"报告.txt","source":"Phone","target":"Desktop"}`)
	announceReq.AddCookie(cookie)
	announceResp := httptest.NewRecorder()
	mux.ServeHTTP(announceResp, announceReq)
	if announceResp.Code != http.StatusOK {
		t.Fatalf("announce status = %d, want %d: %s", announceResp.Code, http.StatusOK, announceResp.Body.String())
	}
	data := <-desktop.send
	var msg proto.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != proto.TypeFile || msg.FID != upload.ID || msg.Name != "报告.txt" || msg.Size != int64(len("mobile file")) {
		t.Fatalf("file message = %+v", msg)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "http://hub.local/download?id="+upload.ID, nil)
	downloadReq.AddCookie(cookie)
	downloadResp := httptest.NewRecorder()
	mux.ServeHTTP(downloadResp, downloadReq)
	if downloadResp.Code != http.StatusOK || downloadResp.Body.String() != "mobile file" {
		t.Fatalf("download status=%d body=%q", downloadResp.Code, downloadResp.Body.String())
	}
	disposition := downloadResp.Header().Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(disposition)
	if err != nil || params["filename"] != "报告.txt" {
		t.Fatalf("Content-Disposition = %q, filename=%q, err=%v", disposition, params["filename"], err)
	}
}

func newMobileTestHub(t *testing.T, token string) *Hub {
	t.Helper()
	return newMobileTestHubAt(t, token, t.TempDir())
}

func newMobileTestHubAt(t *testing.T, token, root string) *Hub {
	t.Helper()
	storeDir := filepath.Join(root, "files")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trust, err := loadMobileTrustStore(filepath.Join(root, mobileTrustFile))
	if err != nil {
		t.Fatal(err)
	}
	return &Hub{
		token:          token,
		instID:         "test-instance",
		storeDir:       storeDir,
		peers:          make(map[*peer]struct{}),
		mobileSessions: make(map[string]time.Time),
		mobileTrust:    trust,
	}
}

func testPeer(name string) *peer {
	return &peer{name: name, send: make(chan []byte, 8), done: make(chan struct{})}
}

func mobileJSONRequest(method, requestPath, body string) *http.Request {
	req := httptest.NewRequest(method, "http://hub.local"+requestPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://hub.local")
	return req
}

func loginMobile(t *testing.T, handler http.Handler, token string) *http.Cookie {
	t.Helper()
	req := mobileJSONRequest(http.MethodPost, "/api/session", `{"token":"`+token+`"}`)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", resp.Code, resp.Body.String())
	}
	cookies := resp.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %+v", cookies)
	}
	return cookies[0]
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMobileUploadedFileLivesInHubStore(t *testing.T) {
	h := newMobileTestHub(t, "")
	req := httptest.NewRequest(http.MethodPost, "http://hub.local/upload?name=test.txt", bytes.NewBufferString("stored"))
	resp := httptest.NewRecorder()
	h.handleUpload(resp, req)
	var result struct {
		ID string `json:"id"`
	}
	if resp.Code != http.StatusOK || json.NewDecoder(resp.Body).Decode(&result) != nil {
		t.Fatalf("upload status=%d body=%s", resp.Code, resp.Body.String())
	}
	f, err := os.Open(filepath.Join(h.storeDir, result.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "stored" {
		t.Fatalf("stored data=%q err=%v", data, err)
	}
}
