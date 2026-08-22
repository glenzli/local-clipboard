package client

import (
	"encoding/base64"

	"github.com/atotto/clipboard"
)

func ReadText() (string, error) { return clipboard.ReadAll() }

func WriteText(s string) error { return clipboard.WriteAll(s) }

func encodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func decodeBase64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }
