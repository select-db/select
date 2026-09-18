package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// A keystroke written as an ordinary code span renders as keycaps:
//
//	Press `Cmd+Enter` to run.
//
// becomes <kbd>Cmd</kbd>+<kbd>Enter</kbd>. The docs used bold for these, which
// makes a shortcut look like emphasis rather than a key you press, and reads
// the same as every other bolded phrase around it.
//
// No new syntax: a code span is what a keystroke already is in plain markdown,
// so the .doc.md files stay readable anywhere they are read, and GitHub renders
// them as code rather than as nothing.
type kbdRenderer struct{}

func (kbdRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeSpan, renderCodeSpan)
}

// keystroke matches a chord: at least one modifier, then a key. The leading
// modifier is what keeps this away from ordinary code — `a+b` is arithmetic,
// `Cmd+a` is a shortcut — so an identifier that happens to contain a plus is
// never mistaken for one.
var keystroke = regexp.MustCompile(
	`^(?:(?:Cmd|Ctrl|Alt|Opt|Option|Shift|Meta|Win|⌘|⌥|⇧|⌃)\+)+` +
		"(?:[A-Za-z0-9]|`|,|\\.|/|\\\\|\\[|\\]|-|=|Enter|Esc|Escape|Tab|Space|Backspace|Delete|Up|Down|Left|Right|" +
		`Arrow(?:Up|Down|Left|Right)|F[1-9][0-2]?)$`)

func renderCodeSpan(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	var text strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		segment, ok := child.(*ast.Text)
		if !ok {
			text.Reset()
			break
		}
		text.Write(segment.Segment.Value(source))
	}

	content := text.String()
	if content == "" || !keystroke.MatchString(content) {
		// Goldmark's own renderer, inlined: the children are the same text
		// nodes walked above, and there is no way to hand the node back.
		w.WriteString("<code>")
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if segment, ok := child.(*ast.Text); ok {
				w.Write(util.EscapeHTML(segment.Segment.Value(source)))
			}
		}
		w.WriteString("</code>")
		return ast.WalkSkipChildren, nil
	}

	keys := strings.Split(content, "+")
	for i, key := range keys {
		if i > 0 {
			w.WriteString("+")
		}
		if mod, ok := platformModifier[key]; ok {
			fmt.Fprintf(w, `<kbd data-mod="%s">%s</kbd>`, mod, util.EscapeHTML([]byte(key)))
			continue
		}
		fmt.Fprintf(w, "<kbd>%s</kbd>", util.EscapeHTML([]byte(key)))
	}
	return ast.WalkSkipChildren, nil
}

// The modifiers whose key is named differently off a Mac. The docs are written
// in the Mac spelling because that is what the app's own menus show there; the
// tag carries which modifier it is, and theme.js relabels it for everyone else.
// Shift, Enter and the rest are the same word on every platform and get no tag.
var platformModifier = map[string]string{
	"Cmd": "cmd", "⌘": "cmd", "Meta": "cmd",
	"Opt": "alt", "Option": "alt", "⌥": "alt", "Alt": "alt",
}

// keystrokes takes precedence over goldmark's renderer for the same node kind,
// which sits at priority 1000.
var keystrokes = util.Prioritized(kbdRenderer{}, 100)
