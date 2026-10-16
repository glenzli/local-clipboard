//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func loadRunConfig(path string) (runConfig, error) {
	var cfg runConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read client configuration: %w", err)
	}
	// Windows PowerShell writes UTF-8 JSON with a BOM.
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &cfg); err != nil {
		return cfg, fmt.Errorf("parse client configuration: %w", err)
	}
	if strings.TrimSpace(cfg.Server) == "" {
		return cfg, fmt.Errorf("client configuration is missing server")
	}
	encrypted, err := os.ReadFile(filepath.Join(filepath.Dir(path), "token.dpapi"))
	if err != nil {
		return cfg, fmt.Errorf("read saved token: %w", err)
	}
	cfg.token, err = decryptWindowsToken(strings.TrimSpace(string(encrypted)))
	return cfg, err
}

func decryptWindowsToken(encoded string) (string, error) {
	data, err := hex.DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("invalid saved DPAPI token")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	runtime.KeepAlive(data)
	if err != nil {
		return "", fmt.Errorf("decrypt saved token for current Windows user: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	plain := unsafe.Slice(out.Data, int(out.Size))
	defer clear(plain)
	if len(plain)%2 != 0 {
		return "", fmt.Errorf("saved token is not UTF-16")
	}
	codepoints := make([]uint16, len(plain)/2)
	for i := range codepoints {
		codepoints[i] = binary.LittleEndian.Uint16(plain[2*i:])
	}
	defer clear(codepoints)
	return string(utf16.Decode(codepoints)), nil
}
