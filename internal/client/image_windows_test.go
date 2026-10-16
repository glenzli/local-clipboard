//go:build windows

package client

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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

// Opt in because this test replaces the interactive user's clipboard. The
// Windows verification script saves and restores it around this test run.
func TestWindowsClipboardImages(t *testing.T) {
	if os.Getenv("LOCALCLIP_TEST_CLIPBOARD") != "1" {
		t.Skip("set LOCALCLIP_TEST_CLIPBOARD=1 in an interactive Windows session")
	}
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	pixels := []color.NRGBA{
		{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255},
		{R: 200, G: 100, B: 50, A: 128}, {A: 0}, {R: 255, G: 255, B: 255, A: 255},
	}
	for i, pixel := range pixels {
		img.SetNRGBA(i%3, i/3, pixel)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	assertImage := func(t *testing.T) {
		t.Helper()
		data, err := ReadImage()
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if got.Bounds() != img.Bounds() {
			t.Fatalf("bounds = %v, want %v", got.Bounds(), img.Bounds())
		}
		for i, want := range pixels {
			pixel := color.NRGBAModel.Convert(got.At(i%3, i/3)).(color.NRGBA)
			if pixel != want {
				t.Fatalf("pixel %d = %v, want %v", i, pixel, want)
			}
		}
	}
	t.Run("PNG round trip preserves color and transparency", func(t *testing.T) {
		if err := WriteImage(encoded.Bytes()); err != nil {
			t.Fatal(err)
		}
		assertImage(t)
	})
	t.Run("PNG-only clipboard from another process", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "图片 with spaces.png")
		if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := imagePowerShellCommand(`$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
$stream = New-Object IO.MemoryStream(,[IO.File]::ReadAllBytes($env:LOCALCLIP_IMAGE_PATH))
$data = New-Object System.Windows.Forms.DataObject
$data.SetData('PNG', $false, $stream)
[System.Windows.Forms.Clipboard]::SetDataObject($data, $true)
$stream.Dispose()`, path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("set PNG clipboard: %v: %s", err, out)
		}
		assertImage(t)
	})
	t.Run("legacy bitmap consumers can read the image", func(t *testing.T) {
		if err := WriteImage(encoded.Bytes()); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "legacy.png")
		cmd := imagePowerShellCommand(`$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = [System.Windows.Forms.Clipboard]::GetImage()
if (-not $img) { throw 'no legacy bitmap' }
try { $img.Save($env:LOCALCLIP_IMAGE_PATH, [System.Drawing.Imaging.ImageFormat]::Png) }
finally { $img.Dispose() }`, path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("legacy bitmap read: %v: %s", err, out)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range pixels {
			if want.A != 255 {
				continue
			}
			if pixel := color.NRGBAModel.Convert(got.At(i%3, i/3)).(color.NRGBA); pixel != want {
				t.Fatalf("legacy pixel %d = %v, want %v", i, pixel, want)
			}
		}
		if got := color.NRGBAModel.Convert(got.At(1, 1)).(color.NRGBA); got != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
			t.Fatalf("legacy transparent background = %v, want white", got)
		}
	})
	t.Run("bitmap-only source from another process", func(t *testing.T) {
		cmd := imagePowerShellCommand(`$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$img = New-Object System.Drawing.Bitmap(2, 2, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
try {
  $img.SetPixel(0, 0, [System.Drawing.Color]::Red)
  [System.Windows.Forms.Clipboard]::SetImage($img)
} finally { $img.Dispose() }`, "")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("set bitmap clipboard: %v: %s", err, out)
		}
		data, err := ReadImage()
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if pixel := color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA); pixel != (color.NRGBA{R: 255, A: 255}) {
			t.Fatalf("bitmap source pixel = %v, want red", pixel)
		}
	})
	t.Run("invalid image reports a failure", func(t *testing.T) {
		if err := WriteImage([]byte("not an image")); err == nil {
			t.Fatal("WriteImage accepted invalid image data")
		}
	})
}
