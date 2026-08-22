package proto

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	KindText = "text"
)

type Message struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	TS      int64  `json:"ts"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

func NewMessage(source, kind, content string) *Message {
	return &Message{
		ID:      newID(),
		Source:  source,
		TS:      time.Now().UnixMilli(),
		Kind:    kind,
		Content: content,
	}
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
