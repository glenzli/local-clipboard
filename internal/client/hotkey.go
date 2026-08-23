//go:build cgo || windows

package client

import (
	"fmt"
	"log"
	"strings"
	"time"

	"golang.design/x/hotkey"
)

// ParseHotkey parses "ctrl+alt+c" style specs.
func ParseHotkey(spec string) ([]hotkey.Modifier, hotkey.Key, error) {
	var mods []hotkey.Modifier
	var key hotkey.Key
	for _, part := range strings.Split(strings.ToLower(spec), "+") {
		switch strings.TrimSpace(part) {
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "alt", "option":
			mods = append(mods, modOption)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "cmd", "super", "win":
			mods = append(mods, modSuper)
		default:
			k, ok := keyMap[part]
			if !ok {
				return nil, 0, fmt.Errorf("unknown key %q in -hotkey spec", part)
			}
			key = k
		}
	}
	if key == 0 || len(mods) == 0 {
		return nil, 0, fmt.Errorf("hotkey needs modifier(s) + a key")
	}
	return mods, key, nil
}

var keyMap = map[string]hotkey.Key{
	"a": hotkey.KeyA, "b": hotkey.KeyB, "c": hotkey.KeyC, "d": hotkey.KeyD,
	"e": hotkey.KeyE, "f": hotkey.KeyF, "g": hotkey.KeyG, "h": hotkey.KeyH,
	"i": hotkey.KeyI, "j": hotkey.KeyJ, "k": hotkey.KeyK, "l": hotkey.KeyL,
	"m": hotkey.KeyM, "n": hotkey.KeyN, "o": hotkey.KeyO, "p": hotkey.KeyP,
	"q": hotkey.KeyQ, "r": hotkey.KeyR, "s": hotkey.KeyS, "t": hotkey.KeyT,
	"u": hotkey.KeyU, "v": hotkey.KeyV, "w": hotkey.KeyW, "x": hotkey.KeyX,
	"y": hotkey.KeyY, "z": hotkey.KeyZ,
	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2, "3": hotkey.Key3,
	"4": hotkey.Key4, "5": hotkey.Key5, "6": hotkey.Key6, "7": hotkey.Key7,
	"8": hotkey.Key8, "9": hotkey.Key9,
	"space": hotkey.KeySpace,
}

func RegisterHotkey(c *Client, spec string) error {
	mods, key, err := ParseHotkey(spec)
	if err != nil {
		return err
	}
	hk := hotkey.New(mods, key)
	for attempt := 1; ; attempt++ {
		err = hk.Register()
		if err == nil {
			break
		}
		c.store.SetHotkey(false)
		log.Printf("[hotkey] %v", err)
		if attempt == 1 {
			Notify("快捷键需要辅助功能权限：系统设置 → 隐私与安全性 → 辅助功能，勾选 local-clipboard 后将自动生效")
		}
		time.Sleep(15 * time.Second)
	}
	c.store.SetHotkey(true)
	log.Printf("[hotkey] %s registered — press anywhere to send clipboard", strings.ToLower(spec))
	go func() {
		for range hk.Keydown() {
			go func() {
				if err := SendCurrentClipboard(c, "", SendEventHooks{}); err != nil {
					log.Printf("[hotkey] send failed: %v", err)
				} else {
					log.Printf("[hotkey] clipboard sent")
				}
			}()
		}
	}()
	return nil
}
