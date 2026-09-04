package main

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// A fence can name the file it comes from by putting the name after the
// language:
//
//	```json db.config.json
//
// which renders the name as a label strip on top of the block. Knowing which
// file a snippet belongs in is most of what a reader needs from a config
// example, and today the docs say it in the prose above -- or not at all.
//
// Goldmark's own renderer writes only the language class and ignores the rest
// of the info string, so the whole node is rendered here instead.
type codeRenderer struct{}

func (codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, renderFence)
}

func renderFence(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	fence := node.(*ast.FencedCodeBlock)
	language, name := fenceInfo(fence, source)

	if name != "" {
		w.WriteString(`<figure class="code"><figcaption>`)
		w.Write(util.EscapeHTML([]byte(name)))
		w.WriteString("</figcaption>")
	}

	w.WriteString("<pre><code")
	if language != "" {
		w.WriteString(` class="language-`)
		w.Write(util.EscapeHTML([]byte(language)))
		w.WriteString(`"`)
	}
	w.WriteString(">")
	for i := 0; i < fence.Lines().Len(); i++ {
		line := fence.Lines().At(i)
		w.Write(util.EscapeHTML(line.Value(source)))
	}
	w.WriteString("</code></pre>\n")

	if name != "" {
		w.WriteString("</figure>\n")
	}

	// The lines were written above; a fenced block has no child nodes to walk.
	return ast.WalkSkipChildren, nil
}

// fenceInfo splits an info string into the language and everything after it,
// which is taken to be a file name.
func fenceInfo(fence *ast.FencedCodeBlock, source []byte) (language, name string) {
	if fence.Info == nil {
		return "", ""
	}
	fields := strings.Fields(string(fence.Info.Segment.Value(source)))
	if len(fields) > 0 {
		language = fields[0]
	}
	if len(fields) > 1 {
		name = strings.Join(fields[1:], " ")
	}
	return language, name
}

// codeBlocks takes precedence over goldmark's renderer for the same node kind,
// which sits at priority 1000.
var codeBlocks = util.Prioritized(codeRenderer{}, 100)
