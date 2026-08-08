package wallpaper

import (
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"

	"github.com/msaeedsaeedi/vocab/internal/apppaths"
)

// Word is the lexical content shown on the wallpaper during exposure.
type Word struct {
	Text       string
	Definition string
	Example    string
	Pos        string
}

// Option customizes the rendered image dimensions.
type Option struct {
	Width  int
	Height int
}

// Render draws the word wallpaper, writes it to the data dir, and sets it as
// the desktop background.
func Render(w Word, opt Option) error {
	img, err := renderImage(w, opt)
	if err != nil {
		return err
	}

	out, err := apppaths.WallpaperImagePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return fmt.Errorf("wallpaper: mkdir: %w", err)
	}
	if err := writeJPEG(out, img); err != nil {
		return fmt.Errorf("wallpaper: %w", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		return fmt.Errorf("wallpaper: stat: %w", err)
	}
	log.Printf("wallpaper: written %s (%d bytes)", out, info.Size())
	if info.Size() == 0 {
		return fmt.Errorf("wallpaper: output file is empty")
	}

	if err := Set(out); err != nil {
		return fmt.Errorf("wallpaper: set: %w", err)
	}
	return nil
}

// RenderPreview draws the word wallpaper to path without setting it.
func RenderPreview(w Word, path string, opt Option) error {
	img, err := renderImage(w, opt)
	if err != nil {
		return err
	}
	if err := writeJPEG(path, img); err != nil {
		return fmt.Errorf("preview: %w", err)
	}
	log.Printf("preview: written %s", path)
	return nil
}

func renderImage(w Word, opt Option) (image.Image, error) {
	return render(wordData{
		text:       w.Text,
		definition: w.Definition,
		example:    w.Example,
		pos:        w.Pos,
	}, opt.Width, opt.Height)
}

func writeJPEG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		f.Close()
		return fmt.Errorf("encode: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}
