//go:build darwin

package client

import (
	"strings"
	"testing"
)

func TestUTF8ClipboardEnvOverridesInheritedLocale(t *testing.T) {
	env := utf8ClipboardEnv([]string{
		"PATH=/usr/bin:/bin",
		"LANG=C",
		"LC_ALL=POSIX",
		"LC_CTYPE=C",
		"UNCHANGED=value",
	})

	values := make(map[string][]string)
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = append(values[key], value)
	}
	for _, key := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		got := values[key]
		if len(got) != 1 || got[0] != macOSClipboardLocale {
			t.Fatalf("%s = %v, want [%s]", key, got, macOSClipboardLocale)
		}
	}
	if got := values["UNCHANGED"]; len(got) != 1 || got[0] != "value" {
		t.Fatalf("unrelated environment changed: %v", got)
	}
}
