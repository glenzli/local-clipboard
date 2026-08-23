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
	mux.HandleFunc("/api/accept-all", u.handleAcceptAll)
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

func inboxDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads", "local-clipboard")
}

func (u *UIServer) acceptOne(fid string) {
	go func() {
		dir := inboxDir()
		os.MkdirAll(dir, 0o755)
		u.c.store.SetFIDState(fid, StateReceiving)
		path, err := DownloadTo(u.c.server, u.c.token, fid, dir, func(done, total int64) {
			u.c.store.UpdateProgress(fid, done, total)
		})
		if err != nil {
			log.Printf("[ui] accept %s: %v", short(fid), err)
			u.c.store.SetFIDState(fid, StateFailed)
			return
		}
		u.c.store.SetFIDState(fid, StateReceived)
		log.Printf("[ui] accepted -> %s", path)
		notify(fmt.Sprintf("已保存 %s", path))
	}()
}

func (u *UIServer) handleAccept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FID string `json:"fid"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req) != nil || req.FID == "" {
		http.Error(w, "bad request", 400)
		return
	}
	u.acceptOne(req.FID)
	w.WriteHeader(202)
}

func (u *UIServer) handleAcceptAll(w http.ResponseWriter, r *http.Request) {
	fids := u.c.store.PendingFIDs()
	for _, fid := range fids {
		u.acceptOne(fid)
	}
	w.WriteHeader(202)
	fmt.Fprintf(w, `{"accepted":%d}`, len(fids))
}

func (u *UIServer) handleSendClipboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req)

	err := SendCurrentClipboard(u.c, req.To, SendEventHooks{
		OnAdd:    func(ev *Event) { u.c.store.Add(ev) },
		Progress: func(ev *Event, done, total int64) { u.c.store.UpdateEvent(ev, done, total) },
		SetSt:    func(ev *Event, state string) { u.c.store.SetState(ev, state) },
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
	}
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
