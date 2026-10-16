//go:build windows

package main

import (
	"encoding/hex"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsRunConfigReadsPowerShellDPAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbf"+`{"server":"nas:9100","name":"Windows 测试"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	wantToken := "test-only 中文 🔑 with spaces "
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `Import-Module "$PSHOME\Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1"; $env:LOCALCLIP_TEST_SECRET | ConvertTo-SecureString -AsPlainText -Force | ConvertFrom-SecureString`)
	cmd.Env = append(os.Environ(), "LOCALCLIP_TEST_SECRET="+wantToken)
	encrypted, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PowerShell test-token encryption failed: %v: %s", err, encrypted)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "token.dpapi"), encrypted, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadRunConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "nas:9100" || cfg.Name != "Windows 测试" || cfg.token != wantToken {
		t.Fatal("saved configuration or DPAPI token did not round trip")
	}
	for _, explicit := range []bool{false, true} {
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		server := fs.String("server", "", "")
		name := fs.String("name", "default", "")
		token := fs.String("token", "env-token", "")
		var args []string
		if explicit {
			args = []string{"-server", "other:9100", "-name", "override", "-token", ""}
		}
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		if err := applyRunConfig(path, fs, server, name, token); err != nil {
			t.Fatal(err)
		}
		if explicit {
			if *server != "other:9100" || *name != "override" || *token != "" {
				t.Fatal("explicit run flags were overridden by saved configuration")
			}
		} else if *server != cfg.Server || *name != cfg.Name || *token != wantToken {
			t.Fatal("run defaults did not use the saved configuration")
		}
	}
}

func TestWindowsRunConfigRejectsInvalidSettings(t *testing.T) {
	for _, content := range []string{`{`, `{}`, `{"server":" "}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRunConfig(path); err == nil {
			t.Fatalf("accepted invalid configuration %q", content)
		}
	}
	for _, encoded := range []string{"", "not-hex", hex.EncodeToString([]byte("invalid DPAPI data"))} {
		if _, err := decryptWindowsToken(encoded); err == nil {
			t.Fatal("accepted an invalid DPAPI token")
		} else if strings.Contains(err.Error(), "invalid DPAPI data") {
			t.Fatal("error exposed the saved token")
		}
	}
}
