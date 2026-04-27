package brain

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTinyPNG writes a 4×4 red PNG to dir and returns its path. Used
// as a fixture so the image helpers can be tested without committing
// binary blobs into the repo.
func makeTinyPNG(t *testing.T, dir, name string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestImageToDataURL(t *testing.T) {
	path := makeTinyPNG(t, t.TempDir(), "x.png")
	url, err := imageToDataURL(path)
	if err != nil {
		t.Fatalf("imageToDataURL: %v", err)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("expected data:image/png;base64, prefix, got %.40s", url)
	}
	// data URL roughly 4× the file size in the base64 chunk
	if len(url) < 80 {
		t.Errorf("data URL suspiciously short: %d chars", len(url))
	}
}

func TestImageToDataURL_UnsupportedExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.bmp")
	os.WriteFile(path, []byte("fake"), 0644)
	_, err := imageToDataURL(path)
	if err == nil {
		t.Fatal("expected error on unsupported extension, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got %v", err)
	}
}

func TestImageToDataURL_FileMissing(t *testing.T) {
	_, err := imageToDataURL("/nonexistent/path/x.png")
	if err == nil {
		t.Fatal("expected error on missing file, got nil")
	}
}

func TestImageToBase64(t *testing.T) {
	path := makeTinyPNG(t, t.TempDir(), "y.png")
	mt, b64, err := imageToBase64(path)
	if err != nil {
		t.Fatalf("imageToBase64: %v", err)
	}
	if mt != "image/png" {
		t.Errorf("media type = %q, want image/png", mt)
	}
	if strings.HasPrefix(b64, "data:") {
		t.Errorf("imageToBase64 should NOT include data: prefix, got %.40s", b64)
	}
	if len(b64) < 50 {
		t.Errorf("base64 payload suspiciously short: %d chars", len(b64))
	}
}

func TestMimeForExtension(t *testing.T) {
	cases := map[string]string{
		"x.png":          "image/png",
		"X.PNG":          "image/png",
		"a/b/c.jpg":      "image/jpeg",
		"a/b/c.JPEG":     "image/jpeg",
		"frame.gif":      "image/gif",
		"shot.webp":      "image/webp",
		"weird.bmp":      "",
		"noext":          "",
		"trailing.png/":  "", // ext picks up empty
	}
	for path, want := range cases {
		got, ok := mimeForExtension(path)
		if want == "" {
			if ok {
				t.Errorf("mimeForExtension(%q) = (%q, true), want unsupported", path, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("mimeForExtension(%q) = (%q, %v), want (%q, true)", path, got, ok, want)
		}
	}
}
