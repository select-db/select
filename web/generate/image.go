package main

import (
	"fmt"
	"html"
	"image"
	"os"

	// Lossless WebP is what a capture writes; x/image reads its header, which
	// is all the dimensions need.
	_ "golang.org/x/image/webp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Screenshots come in a light cut and a dark cut, one word apart:
//
//	shots/connect.light.webp
//	shots/connect.dark.webp
//
// Markdown has one <img> and no idea a theme exists, so an image whose path
// ends in `.light.webp` is rendered as a <picture> carrying both. The dark
// <source> covers a reader whose system is dark; the `data-shot` attribute is
// what theme.js repaints when they use the toggle instead, exactly as the
// marketing page does.
//
// The light path stays the one written in the markdown, so the file a plain
// markdown reader resolves is a real image rather than a name that only means
// something after this runs.
type imageRenderer struct {
	// size reports a staged screenshot's pixel dimensions by file name.
	size func(name string) (w, h int)
}

func (r imageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.render)
}

// dimensions is the width/height pair that reserves the image's box before it
// arrives. Without it the page reflows around every screenshot as it loads,
// which is most of the docs' layout shift.
func (r imageRenderer) dimensions(dest string) string {
	if r.size == nil {
		return ""
	}
	w, h := r.size(dest[strings.LastIndexByte(dest, '/')+1:])
	if w == 0 || h == 0 {
		return ""
	}
	return fmt.Sprintf(` width="%d" height="%d"`, w, h)
}

func (r imageRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	image := node.(*ast.Image)
	dest := string(image.Destination)
	// Typographer hands back the entity it already substituted; escaping that
	// a second time is how an alt ended up reading "table&rsquo;s".
	alt := html.UnescapeString(string(node.Text(source)))
	box := r.dimensions(dest)

	dark, themed := strings.CutSuffix(dest, ".light.webp")
	if !themed {
		fmt.Fprintf(w, `<img src="%s" alt="%s"%s loading="lazy" decoding="async">`,
			util.EscapeHTML([]byte(dest)), util.EscapeHTML([]byte(alt)), box)
		return ast.WalkSkipChildren, nil
	}

	fmt.Fprintf(w, `<picture data-shot><source media="(prefers-color-scheme:dark)" srcset="%s.dark.webp"%s>`+
		`<img src="%s" alt="%s"%s loading="lazy" decoding="async"></picture>`,
		util.EscapeHTML([]byte(dark)), box, util.EscapeHTML([]byte(dest)),
		util.EscapeHTML([]byte(alt)), box)
	return ast.WalkSkipChildren, nil
}

// shotSizes reads the dimensions of every staged screenshot once, so a page
// with six of them does not open six files per render.
func shotSizes(from map[string]string) func(string) (int, int) {
	cache := map[string][2]int{}
	for name, path := range from {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			continue
		}
		cache[name] = [2]int{cfg.Width, cfg.Height}
	}
	return func(name string) (int, int) {
		wh := cache[name]
		return wh[0], wh[1]
	}
}

// images takes precedence over goldmark's renderer for the same node kind,
// which sits at priority 1000.
func imagesRenderer(size func(string) (int, int)) util.PrioritizedValue {
	return util.Prioritized(imageRenderer{size: size}, 100)
}
