package client

import (
	"crypto/sha256"
	"encoding/json"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gorilla/websocket"

	"github.com/g4i/local-clipboard/internal/proto"
)

const (
	pollInterval  = 300 * time.Millisecond
	dialTimeout   = 5 * time.Second
	maxReconnect  = 30 * time.Second
	baseReconnect = time.Second
	readWait      = 60 * time.Second
	writeTimeout  = 5 * time.Second
)

type Client struct {
	server string
	name   string
	token  string

	mu          sync.Mutex
	lastSeen    [sha256.Size]byte
	suppress    [sha256.Size]byte
	hasLastSeen bool
}

func New(server, name, token string) *Client {
	return &Client{server: server, name: name, token: token}
}

func (c *Client) Run() {
	backoff := baseReconnect
	for {
		conn, err := c.dial()
		if err != nil {
			log.Printf("[%s] dial %s: %v", c.name, c.server, err)
			time.Sleep(backoff)
			if backoff *= 2; backoff > maxReconnect {
				backoff = maxReconnect
			}
			continue
		}
		backoff = baseReconnect
		if err := c.serveConn(conn); err != nil {
			log.Printf("[%s] disconnected: %v", c.name, err)
		} else {
			log.Printf("[%s] disconnected", c.name)
		}
		time.Sleep(baseReconnect)
	}
}

func (c *Client) dial() (*websocket.Conn, error) {
	u := "ws://" + c.server + "/ws?name=" + url.QueryEscape(c.name)
	if c.token != "" {
		u += "&token=" + url.QueryEscape(c.token)
	}
	d := websocket.Dialer{HandshakeTimeout: dialTimeout}
	conn, _, err := d.Dial(u, nil)
	return conn, err
}

func (c *Client) serveConn(conn *websocket.Conn) error {
	done := make(chan error, 2)
	go c.receiveLoop(conn, done)
	go c.pollLoop(conn, done)
	err := <-done
	conn.Close()
	return err
}

func (c *Client) pollLoop(conn *websocket.Conn, done chan<- error) {
	ticker := time.NewTicker(pollInterval)
	defer func() { ticker.Stop(); conn.Close() }()

	for range ticker.C {
		text, err := clipboard.ReadAll()
		if err != nil {
			done <- err
			return
		}
		if text == "" {
			continue
		}
		h := sha256.Sum256([]byte(text))

		c.mu.Lock()
		if c.hasLastSeen && h == c.lastSeen {
			c.mu.Unlock()
			continue
		}
		if h == c.suppress {
			c.hasLastSeen = true
			c.lastSeen = h
			c.suppress = [sha256.Size]byte{}
			c.mu.Unlock()
			continue
		}
		c.hasLastSeen = true
		c.lastSeen = h
		c.mu.Unlock()

		data, _ := json.Marshal(proto.NewMessage(c.name, proto.KindText, text))
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			done <- err
			return
		}
		log.Printf("[%s] sent %d bytes to hub", c.name, len(text))
	}
}

func (c *Client) receiveLoop(conn *websocket.Conn, done chan<- error) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(readWait))
	conn.SetPingHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(readWait))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeTimeout))
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			done <- err
			return
		}
		var msg proto.Message
		if json.Unmarshal(data, &msg) != nil || msg.Kind != proto.KindText || msg.Content == "" {
			continue
		}
		if msg.Source == c.name {
			continue
		}

		cur, err := clipboard.ReadAll()
		if err == nil && sha256.Sum256([]byte(cur)) == sha256.Sum256([]byte(msg.Content)) {
			continue
		}
		if err := clipboard.WriteAll(msg.Content); err != nil {
			log.Printf("[%s] write clipboard: %v", c.name, err)
			continue
		}
		h := sha256.Sum256([]byte(msg.Content))
		c.mu.Lock()
		c.suppress = h
		c.lastSeen = h
		c.hasLastSeen = true
		c.mu.Unlock()
		log.Printf("[%s] received %q from %s -> clipboard", c.name, truncate(msg.Content), msg.Source)
	}
}

func SendOnce(server, name, token, text string) error {
	u := "ws://" + server + "/ws?name=" + url.QueryEscape(name)
	if token != "" {
		u += "&token=" + url.QueryEscape(token)
	}
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	msg := proto.NewMessage(name, proto.KindText, text)
	data, _ := json.Marshal(msg)
	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) > 30 {
		return string(r[:30]) + "..."
	}
	return s
}
