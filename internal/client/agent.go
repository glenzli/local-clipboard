package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultUIPort = 9101

var daemonHTTP = &http.Client{
	Timeout:   6 * time.Hour,
	Transport: &http.Transport{Proxy: nil},
}

func tryDaemon(path string, contentType string, body io.Reader) ([]byte, bool, error) {
	resp, err := daemonHTTP.Post("http://127.0.0.1:"+fmt.Sprint(DefaultUIPort)+path, contentType, body)
	if err != nil {
		return nil, false, nil
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode >= 400 {
		return nil, true, fmt.Errorf("daemon: %s", stringsOr(b, resp.Status))
	}
	return b, true, nil
}

func stringsOr(b []byte, s string) string {
	if len(b) > 0 {
		return string(b)
	}
	return s
}

func SendClipboardViaDaemon(to string) (bool, error) {
	payload, _ := json.Marshal(map[string]string{"to": to})
	_, ok, err := tryDaemon("/api/send-clipboard", "application/json", bytes.NewReader(payload))
	if !ok {
		return false, nil
	}
	return true, err
}

func SendFileViaDaemon(path, to string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, nil
	}
	defer f.Close()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, _ := mw.CreateFormFile("file", filepath.Base(path))
		io.Copy(part, f)
		if to != "" {
			mw.WriteField("to", to)
		}
		mw.Close()
		pw.Close()
	}()

	_, ok, err := tryDaemon("/api/send-file", "multipart/form-data; boundary=" + mw.Boundary(), pr)
	if !ok {
		return false, nil
	}
	return true, err
}
