package wallpaper

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	_ "image/jpeg"
	"log"
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

func defaultSize() (int, int) {
	return 1920, 1080
}

type tmplData struct {
	Width      int
	Height     int
	BgImage    template.URL
	Text       string
	Definition string
	Example    template.HTML
	Pos        string
}
// TODO: Highlight word in the example sentence. (It may need to update beneath data structure to include the highlighted word's position in the example sentence.)
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
			if err := renderer.LoadFont(ogre.FontSource{Name: f.name, Data: data}); err != nil {
				initErr = fmt.Errorf("load font %s: %w", f.name, err)
				log.Print(initErr)
				return
			}
		}
	})
	return initErr
}

func render(w wordData, width, height int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		width, height = defaultSize()
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
		Example:    template.HTML(w.example),
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
