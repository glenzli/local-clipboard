package client

import (
	"fmt"
	"net/http"
	"time"

	"fyne.io/systray"
)

type Tray struct {
	C        *Client
	UIPort   int
	mStatus  *systray.MenuItem
	mPending *systray.MenuItem
	mRecvAll *systray.MenuItem
}

func (t *Tray) OnReady() {
	systray.SetIcon(trayIcon())
	systray.SetTitle("local-clipboard")
	systray.SetTooltip("local-clipboard")

	t.mStatus = systray.AddMenuItem("", "")
	t.mStatus.Disable()
	t.mPending = systray.AddMenuItem("", "")
	t.mPending.Disable()
	systray.AddSeparator()
	mSend := systray.AddMenuItem("Send clipboard", "push current clipboard to hub")
	t.mRecvAll = systray.AddMenuItem("Receive all", "accept all offered files")
	systray.AddSeparator()
	mPanel := systray.AddMenuItem("Open panel", "open web UI in browser")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "")

	go func() {
		for {
			t.refresh()
			time.Sleep(800 * time.Millisecond)
		}
	}()

	for {
		select {
		case <-mSend.ClickedCh:
			go func() {
				if err := SendCurrentClipboard(t.C, "", SendEventHooks{}); err != nil {
					fmt.Println("[tray] send:", err)
				}
			}()
		case <-t.mRecvAll.ClickedCh:
			go t.post("/api/accept-all")
		case <-mPanel.ClickedCh:
			openBrowser(fmt.Sprintf("http://127.0.0.1:%d", t.UIPort))
		case <-mQuit.ClickedCh:
			systray.Quit()
		}
	}
}

func (t *Tray) refresh() {
	up := t.C.store.Connected()
	if up {
		t.mStatus.SetTitle("● connected")
	} else {
		t.mStatus.SetTitle("○ offline — reconnecting…")
	}
	n := 0
	for _, e := range pendingSnapshot(t.C.store) {
		if e.State == StateOffered {
			n++
		}
	}
	if n > 0 {
		t.mPending.SetTitle(fmt.Sprintf("%d file(s) waiting", n))
	} else {
		t.mPending.SetTitle("no pending files")
	}
}

func pendingSnapshot(s *Store) []*Event {
	evsI, _ := s.Snapshot(true)["events"].([]*Event)
	return evsI
}

func (t *Tray) post(path string) {
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d%s", t.UIPort, path), "application/json", nil)
	if err == nil {
		resp.Body.Close()
	}
}

func (t *Tray) OnExit() {}

func RunTray(c *Client, uiPort int) {
	t := &Tray{C: c, UIPort: uiPort}
	systray.Run(t.OnReady, t.OnExit)
}
