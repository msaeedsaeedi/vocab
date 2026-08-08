package wallpaper

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

var testWord = Word{
	Text:       "Fortuitous",
	Definition: "happening by accident",
	Example:    "Their meeting was entirely fortuitous.",
	Pos:        "adj.",
}

func TestRenderImage(t *testing.T) {
	img, err := renderImage(testWord, Option{Width: 320, Height: 180})
	if err != nil {
		t.Fatalf("renderImage: %v", err)
	}
	if img == nil {
		t.Fatal("renderImage returned nil image")
	}
	b := img.Bounds()
	if b.Dx() != 320 || b.Dy() != 180 {
		t.Fatalf("rendered image is %dx%d, want 320x180", b.Dx(), b.Dy())
	}
}

func TestWriteJPEG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))

	if err := writeJPEG(path, img); err != nil {
		t.Fatalf("writeJPEG: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open written jpeg: %v", err)
	}
	defer f.Close()
	decoded, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode written jpeg: %v", err)
	}
	if got := decoded.Bounds().Dx(); got != 64 {
		t.Fatalf("decoded width = %d, want 64", got)
	}
	if got := decoded.Bounds().Dy(); got != 48 {
		t.Fatalf("decoded height = %d, want 48", got)
	}
}

func TestWriteJPEGError(t *testing.T) {
	if err := writeJPEG("/nonexistent/deep/out.jpg", image.NewRGBA(image.Rect(0, 0, 8, 8))); err == nil {
		t.Fatal("writeJPEG to invalid path should error")
	}
}

func TestRenderPreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview.jpg")
	if err := RenderPreview(testWord, path, Option{Width: 320, Height: 180}); err != nil {
		t.Fatalf("RenderPreview: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open preview: %v", err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 180 {
		t.Fatalf("preview is %dx%d, want 320x180", b.Dx(), b.Dy())
	}
}
