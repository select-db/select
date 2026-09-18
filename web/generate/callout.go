package main

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// GitHub's alert syntax, which is what the docs already write by hand:
//
//	> [!WARNING]
//	> Variables from .env are not available in proxified mode.
//
// Goldmark has no notion of it, so this walks the parsed document, finds
// blockquotes whose first line is one of those markers, strips the marker and
// tags the blockquote with a class. The CSS does the rest.
//
// The syntax rather than one of our own because it is the one GitHub renders,
// which means the .doc.md files and their markdown twins keep working as
// markdown wherever they are read.
type calloutTransformer struct{}

// calloutKinds is the set GitHub defines. Anything else in brackets is left
// alone and renders as the literal text it is, which is the right outcome for
// a typo.
var calloutKinds = []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"}

func (calloutTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		quote, ok := n.(*ast.Blockquote)
		if !ok {
			return ast.WalkContinue, nil
		}

		first, ok := quote.FirstChild().(*ast.Paragraph)
		if !ok || first.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}

		line := first.Lines().At(0)
		kind := calloutKind(line.Value(source))
		if kind == "" {
			return ast.WalkContinue, nil
		}

		quote.SetAttributeString("class", []byte("callout callout-"+kind))

		// Drop the marker from the rendered text. It is not one node: goldmark
		// starts reading `[!TIP]` as a link, gives up, and leaves the brackets
		// and the word as separate text nodes. So remove every inline that
		// begins inside the marker's line rather than looking for one that
		// matches it.
		//
		// Transformers run after inline parsing, which is also why editing the
		// paragraph's line segments here would change nothing.
		for child := first.FirstChild(); child != nil; {
			text, ok := child.(*ast.Text)
			if !ok || text.Segment.Start >= line.Stop {
				break
			}
			next := child.NextSibling()
			first.RemoveChild(first, text)
			child = next
		}
		if first.FirstChild() == nil {
			quote.RemoveChild(quote, first)
		}

		return ast.WalkContinue, nil
	})
}

// calloutKind reports the lowercase kind for a marker line, or "" if the line
// is not one.
func calloutKind(line []byte) string {
	trimmed := bytes.TrimSpace(line)
	for _, kind := range calloutKinds {
		if bytes.Equal(trimmed, []byte("[!"+kind+"]")) {
			return string(bytes.ToLower([]byte(kind)))
		}
	}
	return ""
}

// callouts registers the transformer at a priority after goldmark's own, so it
// sees a fully parsed tree.
var callouts = util.Prioritized(calloutTransformer{}, 999)
