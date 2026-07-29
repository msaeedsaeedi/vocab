package wallpaper

import (
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

type Word struct {
	Text       string
	Definition string
	Example    string
	Pos        string
	Phonetic   string
}

type Option struct {
	Width  int
	Height int
}

func Render(w Word, opt Option) error {
	return renderWithSet(w, opt, true)
}

func RenderPreview(w Word, path string, opt Option) error {
	img, err := render(wordData{
		text:       w.Text,
		definition: w.Definition,
		example:    w.Example,
		pos:        w.Pos,
		phonetic:   w.Phonetic,
	}, opt.Width, opt.Height)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("preview: create: %w", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		return fmt.Errorf("preview: encode: %w", err)
	}
	log.Printf("preview: written %s", path)
	return nil
}

func renderWithSet(w Word, opt Option, set bool) error {
	if opt.Width == 0 {
		opt.Width = 1920
	}
	if opt.Height == 0 {
		opt.Height = 1080
	}

	img, err := render(wordData{
		text:       w.Text,
		definition: w.Definition,
		example:    w.Example,
		pos:        w.Pos,
		phonetic:   w.Phonetic,
	}, opt.Width, opt.Height)
	if err != nil {
		return err
	}

	out, err := wallpaperPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return fmt.Errorf("wallpaper: mkdir: %w", err)
	}
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("wallpaper: create: %w", err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		f.Close()
		return fmt.Errorf("wallpaper: encode: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("wallpaper: close: %w", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		return fmt.Errorf("wallpaper: stat: %w", err)
	}
	log.Printf("wallpaper: written %s (%d bytes)", out, info.Size())
	if info.Size() == 0 {
		return fmt.Errorf("wallpaper: output file is empty")
	}

	if !set {
		return nil
	}
	if err := Set(out); err != nil {
		return fmt.Errorf("wallpaper: set: %w", err)
	}
	return nil
}

func wallpaperPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("wallpaper: home dir: %w", err)
	}
	var dir string
	if runtime.GOOS == "windows" {
		dir = filepath.Join(home, "AppData", "Roaming", "vocab")
	} else {
		dir = filepath.Join(home, ".local", "share", "vocab")
	}
	return filepath.Join(dir, "wallpaper.jpg"), nil
}
