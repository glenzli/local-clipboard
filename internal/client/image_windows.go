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
if ($img) { $img.Save($args[0], [System.Drawing.Imaging.ImageFormat]::Png) } else { exit 3 }`

const writeImgPS = `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = [System.Drawing.Image]::FromFile($args[0])
[System.Windows.Forms.Clipboard]::SetImage($img)`

func ReadImage() ([]byte, error) {
	tmp := os.TempDir() + "\\localclip-read.png"
	defer os.Remove(tmp)
	cmd := exec.Command("powershell", "-STA", "-NoProfile", "-NonInteractive", "-Command", readImgPS, tmp)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("clipboard image read failed (no image?): %v", err)
	}
	return os.ReadFile(tmp)
}

func WriteImage(data []byte) error {
	tmp := os.TempDir() + "\\localclip-write.png"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	defer os.Remove(tmp)
	cmd := exec.Command("powershell", "-STA", "-NoProfile", "-NonInteractive", "-Command", writeImgPS, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard image write failed: %v %s", err, string(out))
	}
	return nil
}
