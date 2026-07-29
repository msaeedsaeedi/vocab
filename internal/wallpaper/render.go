package wallpaper

import (
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fogleman/gg"
)

type wordData struct {
	text       string
	definition string
	example    string
}

func defaultSize() (int, int) {
	return 1920, 1080
}

func findFont() string {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("SystemRoot")
		if dir == "" {
			dir = `C:\Windows`
		}
		for _, name := range []string{"segoeui.ttf", "arial.ttf", "calibri.ttf", "tahoma.ttf"} {
			p := filepath.Join(dir, "Fonts", name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if runtime.GOOS == "linux" {
		for _, p := range []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
			"/usr/share/fonts/TTF/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if runtime.GOOS == "darwin" {
		for _, p := range []string{
			"/System/Library/Fonts/Helvetica.ttc",
			"/Library/Fonts/Arial.ttf",
			"/System/Library/Fonts/SFNS.ttf",
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func render(w wordData, width, height int) (image.Image, error) {
	if width == 0 || height == 0 {
		width, height = defaultSize()
	}

	fontPath := findFont()
	if fontPath == "" {
		return nil, fmt.Errorf("wallpaper: no system font found")
	}
	log.Printf("wallpaper: using font %s", fontPath)

	dc := gg.NewContext(width, height)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)

	cx := float64(width) / 2
	titleY := float64(height)/2 - 100

	if err := dc.LoadFontFace(fontPath, 96); err != nil {
		return nil, fmt.Errorf("wallpaper: load font: %w", err)
	}
	dc.DrawStringAnchored(w.text, cx, titleY, 0.5, 0.5)

	if w.definition != "" {
		if err := dc.LoadFontFace(fontPath, 42); err != nil {
			return nil, fmt.Errorf("wallpaper: load font: %w", err)
		}
		dc.DrawStringAnchored(w.definition, cx, titleY+120, 0.5, 0.5)
	}

	if w.example != "" {
		if err := dc.LoadFontFace(fontPath, 32); err != nil {
			return nil, fmt.Errorf("wallpaper: load font: %w", err)
		}
		dc.DrawStringWrapped(w.example, cx, titleY+220, 0.5, 0, float64(width)-200, 1.5, gg.AlignCenter)
	}

	return dc.Image(), nil
}
