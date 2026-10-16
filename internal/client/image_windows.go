//go:build windows

package client

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const readImgPS = `$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$data = [System.Windows.Forms.Clipboard]::GetDataObject()
if ($data -and $data.GetDataPresent('PNG', $false)) {
  $png = $data.GetData('PNG', $false)
  if ($png -is [IO.Stream]) {
    $output = [IO.File]::Create($env:LOCALCLIP_IMAGE_PATH)
    try { $png.Position = 0; $png.CopyTo($output) }
    finally { $output.Dispose(); $png.Dispose() }
    exit 0
  }
  if ($png -is [byte[]]) {
    [IO.File]::WriteAllBytes($env:LOCALCLIP_IMAGE_PATH, $png)
    exit 0
  }
}
$img = [System.Windows.Forms.Clipboard]::GetImage()
if ($img) {
  try { $img.Save($env:LOCALCLIP_IMAGE_PATH, [System.Drawing.Imaging.ImageFormat]::Png) }
  finally { $img.Dispose() }
} else { exit 3 }`

const writeImgPS = `$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = [System.Drawing.Image]::FromFile($env:LOCALCLIP_IMAGE_PATH)
try {
  $png = New-Object IO.MemoryStream
  $bitmap = New-Object System.Drawing.Bitmap($img.Width, $img.Height, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
  try {
    if ($img.RawFormat.Guid -eq [System.Drawing.Imaging.ImageFormat]::Png.Guid) {
      $bytes = [IO.File]::ReadAllBytes($env:LOCALCLIP_IMAGE_PATH)
      $png.Write($bytes, 0, $bytes.Length)
    } else { $img.Save($png, [System.Drawing.Imaging.ImageFormat]::Png) }
    $png.Position = 0
    # Keep the original PNG's alpha channel. Legacy bitmap consumers get an
    # opaque image on white instead of an unsupported alpha channel on black.
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
      $graphics.Clear([System.Drawing.Color]::White)
      $graphics.DrawImageUnscaled($img, 0, 0)
    } finally { $graphics.Dispose() }
    $data = New-Object System.Windows.Forms.DataObject
    $data.SetData('PNG', $false, $png)
    $data.SetImage($bitmap)
    [System.Windows.Forms.Clipboard]::SetDataObject($data, $true, 10, 100)
  } finally { $bitmap.Dispose(); $png.Dispose() }
} finally { $img.Dispose() }`

func imagePowerShellCommand(script, path string) *exec.Cmd {
	cmd := exec.Command("powershell.exe", "-STA", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
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
