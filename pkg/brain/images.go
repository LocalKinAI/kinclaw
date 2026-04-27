package brain

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// imageToDataURL reads the file at `path` and returns a data: URL
// suitable for inlining into a vision API request body. Detects the
// MIME type from the file extension (png / jpg / jpeg / gif / webp).
// Returns an error if the file is unreadable or the extension is one
// the function doesn't recognize as image — caller should skip such
// entries rather than half-encoding them.
//
// 1920×1080 PNG screenshots are typically 1-2 MB raw, ~2.7 MB base64.
// Sending 5 such images in one request is ~13 MB on the wire, plus
// vision tokens on the model side. Real cost; only attach images
// the agent actually needs to look at.
func imageToDataURL(path string) (string, error) {
	mime, ok := mimeForExtension(path)
	if !ok {
		return "", fmt.Errorf("unsupported image extension: %s", filepath.Ext(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}

// imageToBase64 reads the file at `path` and returns the raw base64
// payload (no `data:...;base64,` prefix) along with the detected MIME
// type. Used by the Claude API which takes the base64 and media_type
// in separate fields rather than as a single data URL.
func imageToBase64(path string) (mediaType, b64 string, err error) {
	mime, ok := mimeForExtension(path)
	if !ok {
		return "", "", fmt.Errorf("unsupported image extension: %s", filepath.Ext(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	return mime, base64.StdEncoding.EncodeToString(data), nil
}

func mimeForExtension(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	default:
		return "", false
	}
}
