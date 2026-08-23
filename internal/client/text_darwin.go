//go:build darwin

package client

import (
	"os"
	"os/exec"
	"strings"
)

const macOSClipboardLocale = "en_US.UTF-8"

func ReadText() (string, error) {
	cmd := utf8ClipboardCommand("/usr/bin/pbpaste", "-Prefer", "txt")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func WriteText(text string) error {
	cmd := utf8ClipboardCommand("/usr/bin/pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func utf8ClipboardCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = utf8ClipboardEnv(os.Environ())
	return cmd
}

func utf8ClipboardEnv(base []string) []string {
	env := make([]string, 0, len(base)+3)
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "LANG", "LC_ALL", "LC_CTYPE":
			continue
		default:
			env = append(env, entry)
		}
	}
	return append(env,
		"LANG="+macOSClipboardLocale,
		"LC_ALL="+macOSClipboardLocale,
		"LC_CTYPE="+macOSClipboardLocale,
	)
}
