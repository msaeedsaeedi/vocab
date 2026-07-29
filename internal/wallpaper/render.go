package wallpaper

import (
	"embed"
	"fmt"
	"image"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/fogleman/gg"
)

//go:embed assets/fonts/*.ttf
var fonts embed.FS

type wordData struct {
	text       string
	definition string
	example    string
	pos        string
	phonetic   string
}

// ---------------------------------------------------------------------------
// Design tokens
//
// Palette: near-black background, warm off-white hero text, a muted grey for
// metadata, and a single desaturated brass accent reserved for structural
// elements only (the divider). Definition and example each get their own
// tone so the eye can tell reference text from example text without
// reading it. Values match the approved wallpaper mockup.
// ---------------------------------------------------------------------------

type rgb struct{ r, g, b float64 }

var (
	colorBackground  = rgb{0.039, 0.039, 0.043} // #0A0A0B
	colorWord        = rgb{0.957, 0.941, 0.914} // #F4F1E9
	colorMeta        = rgb{0.514, 0.522, 0.545} // #83858A
	colorDivider     = rgb{0.788, 0.655, 0.416} // #C9A76A
	colorDefinition  = rgb{0.855, 0.843, 0.816} // #DAD7D0
	colorUsage       = rgb{0.596, 0.580, 0.541} // #98948A
	colorUsageTarget = colorWord                // emphasis = brighten, not re-font
)

const (
	fontHero   = "Fraunces-Black.ttf"
	fontItalic = "Fraunces-Italic.ttf"
	fontBody   = "Inter-Regular.ttf"
	fontMono   = "JetBrainsMono-Medium.ttf"
)

func defaultSize() (int, int) {
	return 1920, 1080
}

func loadEmbeddedFont(name string) (string, error) {
	data, err := fonts.ReadFile("assets/fonts/" + name)
	if err != nil {
		return "", fmt.Errorf("read embedded font %s: %w", name, err)
	}

	tmpFile, err := os.CreateTemp("", "vocab-font-*.ttf")
	if err != nil {
		return "", fmt.Errorf("create temp font file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("write temp font file: %w", err)
	}

	return tmpFile.Name(), nil
}

// fontSet holds the on-disk paths gg needs to load each face.
type fontSet struct {
	hero   string
	italic string
	body   string
	mono   string
}

// Fonts are extracted from the embedded FS to temp files once per process
// and reused for every render — the old code re-wrote all four temp files
// (and deleted them again) on every single call, which matters once this
// runs per-word, per-user, per notification stage instead of once at
// startup. The temp files live for the life of the process; that's a
// deliberate trade-off against constant re-extraction.
var (
	fontCacheOnce sync.Once
	fontCache     fontSet
	fontCacheErr  error
)

func loadFonts() (fontSet, error) {
	fontCacheOnce.Do(func() {
		type entry struct {
			name string
			dst  *string
		}
		entries := []entry{
			{fontHero, &fontCache.hero},
			{fontItalic, &fontCache.italic},
			{fontBody, &fontCache.body},
			{fontMono, &fontCache.mono},
		}
		for _, e := range entries {
			p, err := loadEmbeddedFont(e.name)
			if err != nil {
				fontCacheErr = err
				return
			}
			*e.dst = p
		}
	})
	return fontCache, fontCacheErr
}

func render(w wordData, width, height int) (image.Image, error) {
	if width == 0 || height == 0 {
		width, height = defaultSize()
	}

	f, err := loadFonts()
	if err != nil {
		return nil, err
	}

	dc := gg.NewContext(width, height)

	// Flat, near-black background. No gradient, no imagery — the previous
	// version simulated a radial gradient with ~190 overlapping
	// low-opacity circles, which is both slow and visibly banded (the
	// per-circle opacity compounds unevenly toward the center). A single
	// flat fill matches the approved design and removes the artifact
	// entirely.
	dc.SetRGB(colorBackground.r, colorBackground.g, colorBackground.b)
	dc.Clear()

	// Every metric below is scaled off a 1920x1080 baseline so the same
	// layout holds up at any output size — 4K desktop wallpapers, phone
	// lock screens, in-app previews, etc.
	scale := math.Min(float64(width)/1920.0, float64(height)/1080.0)
	px := func(v float64) float64 { return v * scale }

	rightEdge := float64(width) - px(140)
	readingWidth := px(650)

	// The whole block is right-aligned: every line ends flush on the same
	// edge (rightEdge), producing the ragged-left / flush-right spine
	// that makes this read as a designed block instead of a text dump.
	// (The previous version anchored everything to a fixed left x, which
	// is why it looked like an ordinary left-aligned paragraph dropped in
	// the corner rather than the bottom-right composition it was supposed
	// to be.)
	y := float64(height) * 0.38

	// 1. The word — right-aligned hero, letter-spaced, shrinks to fit the
	// reading column instead of overflowing it.
	wordSize := px(205)
	if err := dc.LoadFontFace(f.hero, wordSize); err != nil {
		return nil, err
	}
	dc.SetRGB(colorWord.r, colorWord.g, colorWord.b)

	letterSpacing := px(1.5)
	wordWidth := measureTracked(dc, w.text, letterSpacing)
	if wordWidth > readingWidth {
		shrink := readingWidth / wordWidth
		wordSize *= shrink
		letterSpacing *= shrink
		if err := dc.LoadFontFace(f.hero, wordSize); err != nil {
			return nil, err
		}
		wordWidth = measureTracked(dc, w.text, letterSpacing)
	}
	drawTracked(dc, w.text, rightEdge-wordWidth, y, letterSpacing)

	cursorY := y + px(58)

	// 2. Pronunciation + part of speech — one right-aligned row split
	// across two typefaces: italic serif for the grammatical tag (it's
	// language), mono for the phonetic string (it's transcription/data).
	// Either field can be empty; the row and its divider simply collapse
	// when both are missing instead of leaving a dangling gap.
	if w.pos != "" || w.phonetic != "" {
		metaSize := px(28)
		rowWidth, err := drawMetaRow(dc, f.italic, f.mono, w.pos, w.phonetic, rightEdge, cursorY, metaSize, colorMeta, px(18))
		if err != nil {
			return nil, err
		}
		cursorY += px(42)

		// 3. Divider — sized to the row above it, not a fixed decorative
		// bar, so it always reads as underlining metadata rather than
		// an arbitrary rule.
		dc.SetRGBA(colorDivider.r, colorDivider.g, colorDivider.b, 0.55)
		dc.SetLineWidth(math.Max(1.0, px(1.5)))
		dc.DrawLine(rightEdge-rowWidth, cursorY, rightEdge, cursorY)
		dc.Stroke()
		cursorY += px(67)
	} else {
		cursorY += px(20)
	}

	// 4. Definition — clean sans, wrapped to the reading column, each
	// resulting line individually right-aligned so multi-line definitions
	// still end on the shared flush-right edge.
	defSize := px(36)
	if err := dc.LoadFontFace(f.body, defSize); err != nil {
		return nil, err
	}
	dc.SetRGB(colorDefinition.r, colorDefinition.g, colorDefinition.b)
	defLines := dc.WordWrap(w.definition, readingWidth)
	lineHeight := defSize * 1.33
	for _, line := range defLines {
		drawRightAligned(dc, line, rightEdge, cursorY)
		cursorY += lineHeight
	}

	cursorY += px(38)

	// 5. Usage example — italic serif, the same family as the word, so
	// the language pieces of the block visually belong together. The
	// target word is emphasized by brightening it to the word's color,
	// not by switching to the upright Black face mid-sentence — mixing an
	// upright cut into an italic sentence is what made the old emphasis
	// treatment look like a rendering bug rather than a design choice.
	usgSize := px(30)
	if err := dc.LoadFontFace(f.italic, usgSize); err != nil {
		return nil, err
	}
	usgLineHeight := usgSize * 1.45
	drawUsage(dc, "\u2014 "+w.example, w.text, readingWidth, rightEdge, cursorY, usgLineHeight, colorUsage, colorUsageTarget)

	return dc.Image(), nil
}

// measureTracked measures a string as drawTracked will draw it: per-rune
// advances plus a fixed letter-spacing gap between glyphs.
func measureTracked(dc *gg.Context, s string, spacing float64) float64 {
	runes := []rune(s)
	var total float64
	for i, r := range runes {
		w, _ := dc.MeasureString(string(r))
		total += w
		if i < len(runes)-1 {
			total += spacing
		}
	}
	return total
}

// drawTracked draws s left-to-right starting at x with extra letter-spacing
// between glyphs. Used only for the hero word, where a touch of positive
// tracking is part of the display treatment.
func drawTracked(dc *gg.Context, s string, x, y, spacing float64) {
	cur := x
	for _, r := range s {
		rs := string(r)
		w, _ := dc.MeasureString(rs)
		dc.DrawString(rs, cur, y)
		cur += w + spacing
	}
}

// drawRightAligned draws s ending at rightX on baseline y, and returns its
// width so callers can reuse the measurement (e.g. for a matching divider).
func drawRightAligned(dc *gg.Context, s string, rightX, y float64) float64 {
	w, _ := dc.MeasureString(s)
	dc.DrawString(s, rightX-w, y)
	return w
}

// drawMetaRow right-aligns pos /phonetic/ as a single row ending at
// rightX, gracefully handling either field being empty, and returns the
// row's total width so the divider beneath it can be sized to match it
// exactly rather than using an arbitrary fixed length.
func drawMetaRow(dc *gg.Context, italicFont, monoFont, pos, phonetic string, rightX, y, size float64, c rgb, gap float64) (float64, error) {
	dc.SetRGB(c.r, c.g, c.b)

	var posW, phoneW float64

	if phonetic != "" {
		if err := dc.LoadFontFace(monoFont, size); err != nil {
			return 0, err
		}
		phoneW, _ = dc.MeasureString(phonetic)
	}
	if pos != "" {
		if err := dc.LoadFontFace(italicFont, size); err != nil {
			return 0, err
		}
		posW, _ = dc.MeasureString(pos)
	}

	rowGap := 0.0
	if pos != "" && phonetic != "" {
		rowGap = gap
	}
	total := posW + rowGap + phoneW
	x := rightX - total

	if pos != "" {
		if err := dc.LoadFontFace(italicFont, size); err != nil {
			return 0, err
		}
		dc.DrawString(pos, x, y)
		x += posW + rowGap
	}
	if phonetic != "" {
		if err := dc.LoadFontFace(monoFont, size); err != nil {
			return 0, err
		}
		dc.DrawString(phonetic, x, y)
	}

	return total, nil
}

// drawUsage wraps the example sentence to readingWidth (using whichever
// font is currently loaded on dc) and re-draws it line by line, each line
// right-aligned to rightX so it shares the block's flush-right edge. Any
// word containing the target word (case-insensitive) is drawn in the
// emphasis color instead of the base color.
func drawUsage(dc *gg.Context, example, target string, readingWidth, rightX, y, lineHeight float64, base, emphasis rgb) {
	words := strings.Fields(example)
	spaceWidth, _ := dc.MeasureString(" ")

	var lines [][]string
	var current []string
	var currentWidth float64

	for _, word := range words {
		w, _ := dc.MeasureString(word)
		extra := w
		if len(current) > 0 {
			extra += spaceWidth
		}
		if currentWidth+extra > readingWidth && len(current) > 0 {
			lines = append(lines, current)
			current = []string{word}
			currentWidth = w
		} else {
			current = append(current, word)
			currentWidth += extra
		}
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}

	targetLower := strings.ToLower(target)

	for _, line := range lines {
		var lineWidth float64
		for i, word := range line {
			w, _ := dc.MeasureString(word)
			lineWidth += w
			if i < len(line)-1 {
				lineWidth += spaceWidth
			}
		}

		x := rightX - lineWidth
		for _, word := range line {
			if strings.Contains(strings.ToLower(word), targetLower) {
				dc.SetRGB(emphasis.r, emphasis.g, emphasis.b)
			} else {
				dc.SetRGB(base.r, base.g, base.b)
			}
			w, _ := dc.MeasureString(word)
			dc.DrawString(word, x, y)
			x += w + spaceWidth
		}
		y += lineHeight
	}
}
