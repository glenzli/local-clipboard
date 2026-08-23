//go:build windows

package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImagePowerShellCommandPassesPathViaEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "path with spaces.txt")
	cmd := imagePowerShellCommand(
		`[IO.File]::WriteAllText($env:LOCALCLIP_IMAGE_PATH, "ok")`,
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell path handoff failed: %v: %s", err, out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("file contents = %q, want ok", got)
	}
}
