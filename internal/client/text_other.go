//go:build !darwin

package client

import "github.com/atotto/clipboard"

func ReadText() (string, error) { return clipboard.ReadAll() }

func WriteText(s string) error { return clipboard.WriteAll(s) }
