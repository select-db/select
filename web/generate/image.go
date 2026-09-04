package main

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Screenshots come in a light cut and a dark cut, one word apart:
//
//	shots/connect.light.png
//	shots/connect.dark.png
//
// Markdown has one <img> and no idea a theme exists, so an image whose path
// ends in `.light.png` is rendered as a <picture> carrying both. The dark
// <source> covers a reader whose system is dark; the `data-shot` attribute is
// what theme.js repaints when they use the toggle instead, exactly as the
// marketing page does.
//
// The light path stays the one written in the markdown, so the file a plain
// markdown reader resolves is a real image rather than a name that only means
// something after this runs.
type imageRenderer struct{}

func (imageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, renderImage)
}

func renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	image := node.(*ast.Image)
	dest := string(image.Destination)
	alt := string(node.Text(source))

	dark, themed := strings.CutSuffix(dest, ".light.png")
	if !themed {
		fmt.Fprintf(w, `<img src="%s" alt="%s" loading="lazy" decoding="async">`,
			util.EscapeHTML([]byte(dest)), util.EscapeHTML([]byte(alt)))
		return ast.WalkSkipChildren, nil
	}

	fmt.Fprintf(w, `<picture data-shot><source media="(prefers-color-scheme:dark)" srcset="%s.dark.png">`+
		`<img src="%s" alt="%s" loading="lazy" decoding="async"></picture>`,
		util.EscapeHTML([]byte(dark)), util.EscapeHTML([]byte(dest)), util.EscapeHTML([]byte(alt)))
	return ast.WalkSkipChildren, nil
}

// images takes precedence over goldmark's renderer for the same node kind,
// which sits at priority 1000.
var images = util.Prioritized(imageRenderer{}, 100)
