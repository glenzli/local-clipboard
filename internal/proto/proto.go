package proto

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const (
	TypeText  = "text"
	TypeImage = "image"
	TypeFile  = "file"
)

type Message struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	TS      int64  `json:"ts"`
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Content string `json:"content,omitempty"`
	FID     string `json:"fid,omitempty"`
	Target  string `json:"target,omitempty"`
}

func NewMessage(source, typ string) *Message {
	return &Message{ID: newID(), Source: source, TS: time.Now().UnixMilli(), Type: typ}
}

type Hello struct {
	Instance string `json:"instance"`
}

func RandomID() string { return newID() }

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
