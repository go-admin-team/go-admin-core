package utils

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A 1x1 PNG, which is enough for http.DetectContentType and short enough to
// prove the detector is given what was read rather than a padded buffer.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func writePNG(t *testing.T, dir string) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(onePixelPNG)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	p := filepath.Join(dir, "pixel.png")
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// Open deferred a Close on the file it was about to return, so every caller
// received one that was already closed.
func TestOpenReturnsAFileTheCallerCanUse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")

	f, err := Open(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString("still open"); err != nil {
		t.Fatalf("writing to the returned file: %v", err)
	}
}

func TestOpenReportsAMissingFile(t *testing.T) {
	f, err := Open(filepath.Join(t.TempDir(), "absent"), os.O_RDONLY, 0o600)
	if err == nil {
		_ = f.Close()
		t.Fatal("Open on an absent file returned no error")
	}
	if f != nil {
		t.Error("Open returned both an error and a file")
	}
}

// Both detectors called os.Exit on a file they could not open, which ends the
// caller's process rather than the call. A test cannot catch that from inside
// — the binary simply stops — so a run against the old code fails outright,
// which is the observation.
func TestDetectorsReportAMissingFileInsteadOfExiting(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent")

	if _, err := GetType(absent); err == nil {
		t.Error("GetType on an absent file returned no error")
	}
	if _, err := GetImgType(absent); err == nil {
		t.Error("GetImgType on an absent file returned no error")
	}
}

func TestGetTypeAndGetImgType(t *testing.T) {
	dir := t.TempDir()
	png := writePNG(t, dir)

	t.Run("a png is detected as one", func(t *testing.T) {
		got, err := GetImgType(png)
		if err != nil {
			t.Fatalf("GetImgType: %v", err)
		}
		if got != "image/png" {
			t.Errorf("GetImgType = %q, want image/png", got)
		}
	})

	t.Run("a short text file is not padded into something else", func(t *testing.T) {
		p := filepath.Join(dir, "short.txt")
		if err := os.WriteFile(p, []byte("hello"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}

		got, err := GetType(p)
		if err != nil {
			t.Fatalf("GetType: %v", err)
		}
		if !strings.HasPrefix(got, "text/plain") {
			t.Errorf("GetType = %q, want text/plain", got)
		}
	})

	t.Run("a text file is not an image", func(t *testing.T) {
		p := filepath.Join(dir, "note.txt")
		if err := os.WriteFile(p, []byte("not an image"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}

		if _, err := GetImgType(p); err == nil {
			t.Error("GetImgType accepted a text file")
		}
	})
}
