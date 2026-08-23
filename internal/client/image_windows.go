//go:build windows

package client

import (
	"fmt"
	"os"
	"os/exec"
)

const readImgPS = `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = [System.Windows.Forms.Clipboard]::GetImage()
if ($img) {
  try { $img.Save($env:LOCALCLIP_IMAGE_PATH, [System.Drawing.Imaging.ImageFormat]::Png) }
  finally { $img.Dispose() }
} else { exit 3 }`

const writeImgPS = `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = [System.Drawing.Image]::FromFile($env:LOCALCLIP_IMAGE_PATH)
try { [System.Windows.Forms.Clipboard]::SetImage($img) }
finally { $img.Dispose() }`

func imagePowerShellCommand(script, path string) *exec.Cmd {
	cmd := exec.Command("powershell", "-STA", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "LOCALCLIP_IMAGE_PATH="+path)
	return cmd
}

func ReadImage() ([]byte, error) {
	f, err := os.CreateTemp("", "localclip-read-*.png")
	if err != nil {
		return nil, err
	}
	tmp := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	defer os.Remove(tmp)
	cmd := imagePowerShellCommand(readImgPS, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("clipboard image read failed (no image?): %v %s", err, string(out))
	}
	return os.ReadFile(tmp)
}

func WriteImage(data []byte) error {
	f, err := os.CreateTemp("", "localclip-write-*.png")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	defer os.Remove(tmp)
	cmd := imagePowerShellCommand(writeImgPS, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard image write failed: %v %s", err, string(out))
	}
	return nil
}
