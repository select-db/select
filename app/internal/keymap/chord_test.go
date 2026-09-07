package keymap

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		binding  string
		platform Platform
		want     string
	}{
		// "secondary" is whatever the platform uses for its own shortcuts.
		{"secondary+p", MacOS, "cmd+p"},
		{"secondary+p", Linux, "ctrl+p"},
		{"secondary+p", Windows, "ctrl+p"},

		// Everything else names a key, and means it on every platform.
		{"ctrl+`", Linux, "ctrl+`"},
		{"ctrl+`", MacOS, "ctrl+`"},
		{"cmd+p", Linux, "cmd+p"},

		// Modifiers come out in one order however they were written.
		{"shift+ctrl+alt+cmd+k", Linux, "ctrl+alt+shift+cmd+k"},
		{"CMD+SHIFT+P", MacOS, "shift+cmd+p"},

		// Spellings that mean the same modifier.
		{"control+a", Linux, "ctrl+a"},
		{"option+a", MacOS, "alt+a"},
		{"super+a", Linux, "cmd+a"},

		// Spellings that mean the same key.
		{"secondary+arrowup", Linux, "ctrl+up"},
		{"esc", Linux, "escape"},
		{"secondary+return", Linux, "ctrl+enter"},

		// Keys that are not letters.
		{"secondary+-", Linux, "ctrl+-"},
		{"secondary+shift+-", Linux, "ctrl+shift+-"},
		{"secondary+0", Linux, "ctrl+0"},
		{"f7", Linux, "f7"},
		{"secondary+shift+space", Linux, "ctrl+shift+space"},
	}

	for _, c := range cases {
		got, err := Parse(c.binding, c.platform)
		if err != nil {
			t.Errorf("Parse(%q, %s): %v", c.binding, c.platform, err)
			continue
		}
		if got.String() != c.want {
			t.Errorf("Parse(%q, %s) = %q, want %q", c.binding, c.platform, got.String(), c.want)
		}
	}
}

func TestParseCanonicalFormRoundTrips(t *testing.T) {
	for _, binding := range []string{"ctrl+alt+shift+cmd+k", "ctrl+-", "escape", "f12"} {
		chord, err := Parse(binding, Linux)
		if err != nil {
			t.Fatalf("Parse(%q): %v", binding, err)
		}
		again, err := Parse(chord.String(), Linux)
		if err != nil {
			t.Fatalf("Parse(%q): %v", chord.String(), err)
		}
		if again != chord {
			t.Errorf("Parse(%q) did not round-trip: %+v vs %+v", binding, chord, again)
		}
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct {
		binding string
		because string
	}{
		{"", "there is nothing to parse"},
		{"cmd+", "there is no key"},
		{"hyper+k", "there is no such modifier"},
		{"ctrl+enterr", "there is no such key"},
		{"ctrl+f25", "function keys stop at 24"},
	}

	for _, c := range cases {
		if _, err := Parse(c.binding, Linux); err == nil {
			t.Errorf("Parse(%q) = nil error, want one: %s", c.binding, c.because)
		}
	}
}

// A key is named by where it sits, not by what shifting it prints. The error
// says what to write instead, because the difference is invisible until a
// binding silently never fires.
func TestParseRejectsShiftedCharacters(t *testing.T) {
	cases := map[string]string{
		"ctrl+_": "shift+-",
		"ctrl+?": "shift+/",
		"ctrl+:": "shift+;",
		"ctrl+~": "shift+`",
		"ctrl+!": "shift+1",
	}

	for binding, suggestion := range cases {
		_, err := Parse(binding, Linux)
		if err == nil {
			t.Errorf("Parse(%q) = nil error, want a refusal", binding)
			continue
		}
		if !strings.Contains(err.Error(), suggestion) {
			t.Errorf("Parse(%q) error = %q, want it to suggest %q", binding, err, suggestion)
		}
	}
}

func TestSecondary(t *testing.T) {
	if got := MacOS.Secondary(); got != "cmd" {
		t.Errorf("MacOS.Secondary() = %q, want cmd", got)
	}
	for _, p := range []Platform{Linux, Windows} {
		if got := p.Secondary(); got != "ctrl" {
			t.Errorf("%s.Secondary() = %q, want ctrl", p, got)
		}
	}
}
