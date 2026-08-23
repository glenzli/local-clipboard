package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/g4i/local-clipboard/internal/proto"
)

var directHTTP = &http.Client{
	Timeout:   6 * time.Hour,
	Transport: &http.Transport{Proxy: nil},
}

type progressFunc func(done, total int64)

type progressReader struct {
	r      io.Reader
	total  int64
	cb     progressFunc
	done   int64
	lastPc int
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.cb != nil {
		p.cb(p.done, p.total)
	}
	if p.total > 0 {
		pc := int(p.done * 100 / p.total)
		if pc != p.lastPc && pc%5 == 0 {
			fmt.Printf("\r%s%d%% (%s/%s)", progressPrefix, pc, humanBytes(p.done), humanBytes(p.total))
			p.lastPc = pc
		}
	}
	return n, err
}

var progressPrefix = ""

func UploadFile(server, token, path, displayName string, cb progressFunc) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	size := fi.Size()

	pr := &progressReader{r: f, total: size, cb: cb}
	q := "name=" + url.QueryEscape(displayName)
	req, err := http.NewRequest("POST", "http://"+server+"/upload?"+q, pr)
	if err != nil {
		return "", 0, err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := directHTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	fmt.Println()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", 0, fmt.Errorf("upload failed: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.ID, size, nil
}

func DownloadTo(server, token, fid, outDir string, cb progressFunc) (string, error) {
	name := fid
	if i := strings.Index(fid, "-"); i > 0 && len(fid) > i+1 {
		name = fid[i+1:]
	}
	dst := filepath.Join(outDir, filepath.Base(name))
	q := "id=" + url.QueryEscape(fid)
	req, err := http.NewRequest(http.MethodGet, "http://"+server+"/download?"+q, nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := directHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("download failed: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, cb: cb}
	_, err = io.Copy(out, pr)
	closeErr := out.Close()
	fmt.Println()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return dst, nil
}

type SendEventHooks struct {
	OnAdd    func(ev *Event)
	Progress func(ev *Event, done, total int64)
	SetSt    func(ev *Event, state string)
}

func (c *Client) StoreHooks() SendEventHooks {
	return SendEventHooks{
		OnAdd:    func(ev *Event) { c.store.Add(ev) },
		Progress: func(ev *Event, done, total int64) { c.store.UpdateEvent(ev, done, total) },
		SetSt:    func(ev *Event, state string) { c.store.SetState(ev, state) },
	}
}

func SendCurrentClipboard(c *Client, to string, hooks SendEventHooks) error {
	add := func(ev *Event) {
		if hooks.OnAdd != nil {
			hooks.OnAdd(ev)
		}
	}
	prog := func(ev *Event) progressFunc {
		if hooks.Progress == nil {
			return nil
		}
		return func(d, t int64) { hooks.Progress(ev, d, t) }
	}
	done := func(ev *Event) {
		if hooks.SetSt != nil {
			hooks.SetSt(ev, StateDone)
		}
	}
	fail := func(ev *Event) {
		if hooks.SetSt != nil {
			hooks.SetSt(ev, StateFailed)
		}
	}

	peer := to
	if peer == "" {
		peer = "hub"
	}

	if refs, _ := ReadFileRefs(); len(refs) > 0 {
		for _, p := range refs {
			ev := &Event{Dir: "out", Kind: "file", Name: filepath.Base(p), Peer: peer, State: StateSending}
			add(ev)
			fid, size, err := UploadFile(c.server, c.token, p, filepath.Base(p), prog(ev))
			if err != nil {
				fail(ev)
				return err
			}
			msg := proto.NewMessage("", proto.TypeFile)
			msg.Name = filepath.Base(p)
			msg.Size = size
			msg.FID = fid
			msg.Target = to
			if err := SendPayload(c.server, c.name, c.token, msg); err != nil {
				fail(ev)
				return err
			}
			done(ev)
		}
		return nil
	}

	msg, hint, err := DetectAndBuildClipboardMessage()
	if err != nil {
		return err
	}
	if hint {
		return fmt.Errorf("clipboard text too large for inline sync; save it as a file and use send-file")
	}
	msg.Target = to
	ev := &Event{Dir: "out", Kind: msg.Type, Name: msg.Type + " from clipboard", Size: msg.Size, Peer: peer, State: StateSending}
	add(ev)
	if err := SendPayload(c.server, c.name, c.token, msg); err != nil {
		fail(ev)
		return err
	}
	done(ev)
	return nil
}
func DetectAndBuildClipboardMessage() (*proto.Message, bool, error) {
	// Text wins over image: many apps attach a rendered TIFF preview alongside
	// copied rich text. A real screenshot has no text flavor, so it still
	// lands in the image branch below.
	text, _ := ReadText()
	if text != "" {
		m := proto.NewMessage("", proto.TypeText)
		m.Size = int64(len(text))
		if m.Size > maxInlineText {
			return nil, true, fmt.Errorf("text too large for inline sync (%s); put it in a file and use send-file", humanBytes(m.Size))
		}
		m.Content = text
		return m, false, nil
	}

	img, err := ReadImage()
	if err == nil && len(img) > 0 {
		m := proto.NewMessage("", proto.TypeImage)
		m.Size = int64(len(img))
		m.Content = encodeBase64(img)
		return m, false, nil
	}

	return nil, false, fmt.Errorf("clipboard is empty (image read failed: %v)", err)
}

func humanBytes(n int64) string {
	const kb, mb = 1 << 10, 1 << 20
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1fMB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.0fKB", float64(n)/kb)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
