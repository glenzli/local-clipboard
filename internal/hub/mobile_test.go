package hub

import (
	"bytes"
	"encoding/json"
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
	for _, fragment := range []string{`/mobile/app.css`, `/mobile/app.js`, `id="sendPanel"`, `id="receivePanel"`} {
		if !strings.Contains(pageResp.Body.String(), fragment) {
			t.Fatalf("mobile page missing %q", fragment)
		}
	}

	assetReq := httptest.NewRequest(http.MethodGet, "http://hub.local/mobile/app.js", nil)
	assetResp := httptest.NewRecorder()
	mux.ServeHTTP(assetResp, assetReq)
	if assetResp.Code != http.StatusOK || !strings.Contains(assetResp.Body.String(), "sendFiles") {
		t.Fatalf("mobile JavaScript asset was not served: status=%d", assetResp.Code)
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
	return &Hub{
		token:          token,
		instID:         "test-instance",
		storeDir:       t.TempDir(),
		peers:          make(map[*peer]struct{}),
		mobileSessions: make(map[string]time.Time),
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
