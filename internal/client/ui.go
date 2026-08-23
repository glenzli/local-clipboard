package client

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/g4i/local-clipboard/internal/proto"
)

//go:embed web
var webFS embed.FS

type UIServer struct {
	c      *Client
	port   int
	client *http.Client
}

func StartUI(c *Client, port int) error {
	u := &UIServer{c: c, port: port, client: &http.Client{Transport: &http.Transport{Proxy: nil}}}
	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/state", u.handleState)
	mux.HandleFunc("/api/accept", u.handleAccept)
	mux.HandleFunc("/api/send-clipboard", u.handleSendClipboard)
	mux.HandleFunc("/api/send-file", u.handleSendFile)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("[ui] http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

func (u *UIServer) handleState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u.c.store.Snapshot(true))
}

func (u *UIServer) handleAccept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FID string `json:"fid"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req) != nil || req.FID == "" {
		http.Error(w, "bad request", 400)
		return
	}
	go func() {
		dir := inboxDir()
		os.MkdirAll(dir, 0o755)
		path, err := DownloadTo(u.c.server, u.c.token, req.FID, dir, func(done, total int64) {
			u.c.store.UpdateProgress(req.FID, done, total)
		})
		if err != nil {
			log.Printf("[ui] accept %s: %v", short(req.FID), err)
			u.c.store.Add(&Event{Dir: "in", Kind: "file", Name: filepath.Base(req.FID), FID: req.FID, State: StateFailed})
			return
		}
		u.c.store.Mark(req.FID, StateReceived)
		log.Printf("[ui] accepted -> %s", path)
		notify(fmt.Sprintf("已保存 %s", path))
	}()
	w.WriteHeader(202)
}

func inboxDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads", "local-clipboard")
}

func (u *UIServer) handleSendClipboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req)

	msg, hint, err := DetectAndBuildClipboardMessage()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if hint {
		http.Error(w, "clipboard holds very large text; use send-file instead", 400)
		return
	}
	msg.Target = req.To
	ev := &Event{Dir: "out", Kind: msg.Type, Name: msg.Type + " from clipboard", Size: msg.Size, Peer: "hub", State: StateSending}
	if msg.Target != "" {
		ev.Peer = msg.Target
	}
	u.c.store.Add(ev)
	if err := SendPayload(u.c.server, u.c.name, u.c.token, msg); err != nil {
		u.c.store.SetState(ev, StateFailed)
		http.Error(w, err.Error(), 502)
		return
	}
	u.c.store.SetState(ev, StateDone)
	w.WriteHeader(200)
}

func (u *UIServer) handleSendFile(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(64 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", 400)
		return
	}
	defer file.Close()
	to := r.FormValue("to")

	tmp, err := os.CreateTemp("", "localclip-*-"+filepath.Base(header.Filename))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmpPath := tmp.Name()
	size, _ := io.Copy(tmp, file)
	tmp.Close()
	defer os.Remove(tmpPath)

	ev := &Event{Dir: "out", Kind: "file", Name: filepath.Base(header.Filename), Size: size, Peer: "hub", State: StateSending}
	if to != "" {
		ev.Peer = to
	}
	u.c.store.Add(ev)

	fid, _, err := UploadFile(u.c.server, u.c.token, tmpPath, header.Filename, func(done, total int64) {
		u.c.store.UpdateEvent(ev, done, total)
	})
	if err != nil {
		u.c.store.SetState(ev, StateFailed)
		http.Error(w, err.Error(), 502)
		return
	}

	msg := proto.NewMessage("", proto.TypeFile)
	msg.Name = filepath.Base(header.Filename)
	msg.Size = size
	msg.FID = fid
	msg.Target = to
	if err := SendPayload(u.c.server, u.c.name, u.c.token, msg); err != nil {
		u.c.store.SetState(ev, StateFailed)
		http.Error(w, err.Error(), 502)
		return
	}
	u.c.store.SetState(ev, StateDone)
	fmt.Fprintf(w, `{"ok":true,"fid":%q}`, fid)
}
