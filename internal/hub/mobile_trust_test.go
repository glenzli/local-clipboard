package hub

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustedMobileDeviceSurvivesRestartAndCanBeForgotten(t *testing.T) {
	root := t.TempDir()
	h := newMobileTestHubAt(t, "secret", root)
	mux := http.NewServeMux()
	h.Routes(mux)

	loginReq := mobileJSONRequest(http.MethodPost, "/api/session", `{"token":"secret","trust":true,"device":"My Phone"}`)
	loginResp := httptest.NewRecorder()
	mux.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("trusted login status = %d: %s", loginResp.Code, loginResp.Body.String())
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge <= 0 || cookies[0].Expires.IsZero() {
		t.Fatalf("trusted login cookie = %+v", cookies)
	}
	cookie := cookies[0]

	trustPath := filepath.Join(root, mobileTrustFile)
	data, err := os.ReadFile(trustPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("secret")) || bytes.Contains(data, []byte(cookie.Value)) {
		t.Fatal("trusted-device store exposes the shared token or raw cookie")
	}
	if !bytes.Contains(data, []byte("My Phone")) {
		t.Fatalf("trusted-device store does not contain device label: %s", data)
	}
	info, err := os.Stat(trustPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("trusted-device store mode = %o, want 600", got)
	}

	restarted := newMobileTestHubAt(t, "secret", root)
	restartedMux := http.NewServeMux()
	restarted.Routes(restartedMux)
	statusReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/status", nil)
	statusReq.AddCookie(cookie)
	statusResp := httptest.NewRecorder()
	restartedMux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("trusted session after restart = %d, want %d", statusResp.Code, http.StatusOK)
	}

	rotated := newMobileTestHubAt(t, "new-secret", root)
	rotatedMux := http.NewServeMux()
	rotated.Routes(rotatedMux)
	rotatedReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/status", nil)
	rotatedReq.AddCookie(cookie)
	rotatedResp := httptest.NewRecorder()
	rotatedMux.ServeHTTP(rotatedResp, rotatedReq)
	if rotatedResp.Code != http.StatusUnauthorized {
		t.Fatalf("trusted session after token rotation = %d, want %d", rotatedResp.Code, http.StatusUnauthorized)
	}

	forgetReq := httptest.NewRequest(http.MethodDelete, "http://hub.local/api/session", nil)
	forgetReq.Header.Set("Origin", "http://hub.local")
	forgetReq.AddCookie(cookie)
	forgetResp := httptest.NewRecorder()
	restartedMux.ServeHTTP(forgetResp, forgetReq)
	if forgetResp.Code != http.StatusNoContent {
		t.Fatalf("forget status = %d, want %d: %s", forgetResp.Code, http.StatusNoContent, forgetResp.Body.String())
	}

	forgotten := newMobileTestHubAt(t, "secret", root)
	forgottenMux := http.NewServeMux()
	forgotten.Routes(forgottenMux)
	forgottenReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/status", nil)
	forgottenReq.AddCookie(cookie)
	forgottenResp := httptest.NewRecorder()
	forgottenMux.ServeHTTP(forgottenResp, forgottenReq)
	if forgottenResp.Code != http.StatusUnauthorized {
		t.Fatalf("forgotten session after restart = %d, want %d", forgottenResp.Code, http.StatusUnauthorized)
	}
}

func TestUntrustedMobileSessionDoesNotPersist(t *testing.T) {
	root := t.TempDir()
	h := newMobileTestHubAt(t, "secret", root)
	mux := http.NewServeMux()
	h.Routes(mux)

	cookie := loginMobile(t, mux, "secret")
	if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("untrusted login should use a browser-session cookie: %+v", cookie)
	}
	if _, err := os.Stat(filepath.Join(root, mobileTrustFile)); !os.IsNotExist(err) {
		t.Fatalf("untrusted login unexpectedly persisted a device: %v", err)
	}

	restarted := newMobileTestHubAt(t, "secret", root)
	restartedMux := http.NewServeMux()
	restarted.Routes(restartedMux)
	statusReq := httptest.NewRequest(http.MethodGet, "http://hub.local/api/status", nil)
	statusReq.AddCookie(cookie)
	statusResp := httptest.NewRecorder()
	restartedMux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusUnauthorized {
		t.Fatalf("untrusted session after restart = %d, want %d", statusResp.Code, http.StatusUnauthorized)
	}
}

func TestTrustedMobileDeviceLimitEvictsOldest(t *testing.T) {
	devices := map[string]trustedMobileDevice{
		"oldest": {CreatedAt: 10},
		"middle": {CreatedAt: 20},
		"newest": {CreatedAt: 30},
	}
	forgetOldestTrustedMobileDevice(devices)
	if _, ok := devices["oldest"]; ok {
		t.Fatal("oldest trusted device was not evicted")
	}
	if len(devices) != 2 {
		t.Fatalf("trusted devices after eviction = %d, want 2", len(devices))
	}
}
