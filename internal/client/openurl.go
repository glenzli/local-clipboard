package client

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

func revealPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,"+filepath.Clean(path))
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("reveal file: %w", err)
	}
	return nil
}
