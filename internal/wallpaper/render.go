package wallpaper

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"image"
	_ "image/jpeg"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/macawls/ogre"
)

//go:embed assets/fonts/*.ttf
//go:embed assets/bg-image.png
var embedded embed.FS

type wordData struct {
	text       string
	definition string
	example    string
	pos        string
}

const (
	defaultWidth  = 1920
	defaultHeight = 1080
)

type tmplData struct {
	Width      int
	Height     int
	BgImage    template.URL
	Text       string
	Definition string
	Example    template.HTML
	Pos        string
}

var pageTmpl = template.Must(template.New("wallpaper").Parse(
	`<div style="position:relative;width:{{.Width}}px;height:{{.Height}}px;">` +
		`<img src="{{.BgImage}}" style="position:absolute;top:0;left:0;width:100%;height:100%;" />` +
		`<div style="position:absolute;top:0;left:0;width:100%;height:100%;display:flex;align-items:center;justify-content:flex-end;padding-right:160px;">` +
		`<div style="display:flex;flex-direction:column;align-items:flex-end;text-align:right">` +
		`<div style="font-family:Inter-Italic;font-size:36px;color:rgb(135, 159, 181);letter-spacing:0.02em;position:relative;z-index:1;margin-bottom:-36px;">{{.Pos}}</div>` +
		`<div style="font-family:Fraunces;font-size:164px;color:#FFFFFF;text-align: right;">{{.Text}}</div>` +
		`<div style="font-family:Inter;font-size:36px;color:#d0d1d2;margin-top:18px;">{{.Definition}}</div>` +
		`<div style="font-family:Inter-Italic;font-size:32px;color:#a2a5a8;margin-top:64px;">― {{.Example}}</div>` +
		`</div>` +
		`</div>` +
		`</div>`,
))

var (
	initOnce sync.Once
	initErr  error
	renderer *ogre.Renderer
)

// ensureRenderer lazily loads the embedded fonts and constructs the renderer.
func ensureRenderer() error {
	initOnce.Do(func() {
		renderer = ogre.NewRenderer()
		type fontDef struct {
			file string
			name string
		}
		required := []fontDef{
			{"Fraunces.ttf", "Fraunces"},
			{"Inter-Regular.ttf", "Inter"},
			{"Inter-Bold.ttf", "Inter"},
			{"Inter-Italic.ttf", "Inter-Italic"},
			{"Inter-MediumItalic.ttf", "Inter-MediumItalic"},
		}
		missing := make([]string, 0)
		for _, f := range required {
			if _, err := embedded.ReadFile("assets/fonts/" + f.file); err != nil {
				missing = append(missing, f.file)
			}
		}
		if len(missing) > 0 {
			initErr = fmt.Errorf("missing required font files: %s; download each exact file and place it in internal/wallpaper/assets/fonts", strings.Join(missing, ", "))
			log.Print(initErr)
			return
		}
		for _, f := range required {
			data, err := embedded.ReadFile("assets/fonts/" + f.file)
			if err != nil {
				initErr = fmt.Errorf("missing required font %s: %w", f.file, err)
				log.Print(initErr)
				return
			}
			if err := renderer.LoadFont(ogre.FontSource{Name: f.name, Weight: 400, Style: "normal", Data: data}); err != nil {
				initErr = fmt.Errorf("load font %s: %w", f.name, err)
				log.Print(initErr)
				return
			}
		}
	})
	return initErr
}

// render lays out w over the embedded background and returns the resulting
// image. width and height are expected to be positive.
func render(w wordData, width, height int) (image.Image, error) {
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	if err := ensureRenderer(); err != nil {
		return nil, err
	}

	bgData, err := embedded.ReadFile("assets/bg-image.png")
	if err != nil {
		return nil, fmt.Errorf("read bg image: %w", err)
	}
	bgURI := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(bgData))

	data := tmplData{
		Width:      width,
		Height:     height,
		BgImage:    bgURI,
		Text:       w.text,
		Definition: w.definition,
		Example:    highlightExample(w.example, w.text),
		Pos:        w.pos,
	}

	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	result, err := renderer.Render(buf.String(), ogre.Options{
		Width:  width,
		Height: height,
		Format: ogre.FormatPNG,
	})
	if err != nil {
		return nil, fmt.Errorf("ogre render: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(result.Data))
	if err != nil {
		return nil, fmt.Errorf("decode ogre output: %w", err)
	}
	return img, nil
}

func highlightExample(example, word string) template.HTML {
	escaped := html.EscapeString(example)
	if word == "" {
		return template.HTML(escaped)
	}

	pattern := `(?i)\b` + regexp.QuoteMeta(html.EscapeString(word)) + `\b`
	re := regexp.MustCompile(pattern)
	const open = `<span style="font-family:Inter-MediumItalic;color:#f2f3f4">`
	const close = `</span>`
	highlighted := re.ReplaceAllStringFunc(escaped, func(match string) string {
		return open + match + close
	})
	// ogre trims text at inline-element boundaries; keep spaces on either side
	// of a highlighted match with a zero-width separator.
	highlighted = strings.ReplaceAll(highlighted, " "+open, " \u200b"+open)
	highlighted = strings.ReplaceAll(highlighted, close+" ", close+"\u200b ")
	return template.HTML(highlighted)
}
