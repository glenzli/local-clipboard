package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/g4i/local-clipboard/internal/proto"
)

const (
	dialTimeout   = 5 * time.Second
	maxReconnect  = 30 * time.Second
	baseReconnect = time.Second
	readWait      = 60 * time.Second
	writeTimeout  = 10 * time.Second
	maxMsgSize    = 32 << 20
	maxInlineText = 1 << 20
)

type Client struct {
	server string
	name   string
	token  string
	store  *Store
}

func New(server, name, token string) *Client {
	return &Client{server: server, name: name, token: token, store: NewStore()}
}

func (c *Client) Store() *Store { return c.store }

func knownHostsPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "local-clipboard", "known_hosts.json"), nil
}

func (c *Client) verifyFingerprint(instance string) error {
	path, err := knownHostsPath()
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(path), 0o700)
	m := map[string]string{}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &m)
	}
	known, ok := m[c.server]
	if !ok {
		m[c.server] = instance
		b, _ := json.MarshalIndent(m, "", "  ")
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, b, 0o600); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	}
	if known != instance {
		return fmt.Errorf("SERVER IDENTITY MISMATCH for %s: expected %s got %s.\nIf you rebuilt the hub intentionally, delete the record:\n  rm %s", c.server, short(known), short(instance), path)
	}
	return nil
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func (c *Client) dialWS() (*websocket.Conn, error) {
	u := "ws://" + c.server + "/ws?name=" + url.QueryEscape(c.name)
	if c.token != "" {
		u += "&token=" + url.QueryEscape(c.token)
	}
	d := websocket.Dialer{HandshakeTimeout: dialTimeout, Proxy: nil}
	conn, resp, err := d.Dial(u, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 401 {
			return nil, errors.New("unauthorized: wrong or missing token")
		}
		return nil, err
	}
	return conn, nil
}

func (c *Client) handshake(conn *websocket.Conn) error {
	conn.SetReadDeadline(time.Now().Add(dialTimeout))
	var hello proto.Hello
	conn.SetReadLimit(maxMsgSize)
	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("no hello from hub: %w", err)
	}
	if err := json.Unmarshal(data, &hello); err != nil || hello.Instance == "" {
		return errors.New("peer is not a local-clipboard hub")
	}
	if err := c.verifyFingerprint(hello.Instance); err != nil {
		conn.Close()
		return err
	}
	conn.SetReadDeadline(time.Time{})
	return nil
}

func (c *Client) connect() (*websocket.Conn, error) {
	conn, err := c.dialWS()
	if err != nil {
		return nil, err
	}
	if err := c.handshake(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) Run() {
	backoff := baseReconnect
	for {
		conn, err := c.connect()
		if err != nil {
			log.Printf("[%s] %v", c.name, err)
			time.Sleep(backoff)
			if backoff *= 2; backoff > maxReconnect {
				backoff = maxReconnect
			}
			continue
		}
		backoff = baseReconnect
		c.store.SetConnected(true)
		log.Printf("[%s] connected to %s, waiting for incoming items...", c.name, c.server)
		err = c.receiveLoop(conn)
		conn.Close()
		c.store.SetConnected(false)
		log.Printf("[%s] disconnected: %v", c.name, err)
		time.Sleep(baseReconnect)
	}
}

func (c *Client) receiveLoop(conn *websocket.Conn) error {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(readWait))
	conn.SetPingHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(readWait))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeTimeout))
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var msg proto.Message
		if json.Unmarshal(data, &msg) != nil || msg.Source == c.name {
			continue
		}
		switch msg.Type {
		case proto.TypeText:
			c.handleText(msg)
		case proto.TypeImage:
			c.handleImage(msg)
		case proto.TypeFile:
			c.handleFileNotice(msg)
		}
	}
}

var writeMu sync.Mutex

func (c *Client) handleText(msg proto.Message) {
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := WriteText(msg.Content); err != nil {
		log.Printf("[%s] write text to clipboard: %v", c.name, err)
		return
	}
	log.Printf("[%s] <- text (%d bytes) from %s -> clipboard", c.name, len(msg.Content), msg.Source)
}

func (c *Client) handleImage(msg proto.Message) {
	writeMu.Lock()
	defer writeMu.Unlock()
	data, err := decodeBase64(msg.Content)
	if err != nil {
		log.Printf("[%s] bad image payload: %v", c.name, err)
		return
	}
	if err := WriteImage(data); err != nil {
		log.Printf("[%s] write image to clipboard: %v", c.name, err)
		return
	}
	log.Printf("[%s] <- image (%d bytes) from %s -> clipboard", c.name, len(data), msg.Source)
	notify(fmt.Sprintf("图片已就绪，可直接粘贴（来自 %s）", msg.Source))
}

func (c *Client) handleFileNotice(msg proto.Message) {
	log.Printf("[%s] <- file offer from %s: %s (%s) id=%s", c.name, msg.Source, msg.Name, humanBytes(msg.Size), msg.FID)
	c.store.Add(&Event{Dir: "in", Kind: "file", Peer: msg.Source, Name: msg.Name, Size: msg.Size, State: StateOffered, FID: msg.FID})
	notify(fmt.Sprintf("%s 发来文件 %s (%s)\n点击接收或在 UI 中操作", msg.Source, msg.Name, humanBytes(msg.Size)))
}

func notify(text string) {
	Notify(text)
}

func SendPayload(server, name, token string, msg *proto.Message) error {
	c := New(server, name, token)
	conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	msg.Source = name
	data, _ := json.Marshal(msg)
	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

