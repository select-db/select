// Package keymap turns the keybindings written in a config file into the
// keystrokes an app can match against.
//
// A binding is written as modifiers and a key joined by "+": "ctrl+shift+p".
// It is parsed once, here, into a Chord -- the modifiers held and the key
// pressed -- and printed back in one canonical form, so a binding and a
// keystroke can be compared as strings without either side guessing what the
// other meant.
//
// Two rules carry most of the weight:
//
//   - A key is named by where it is on the keyboard, never by the character
//     shifting it produces. "shift+-" is a chord; "_" is not one, and saying so
//     is what keeps a binding working on a layout it was not written on.
//   - "secondary" is the modifier a platform uses for its own shortcuts: cmd on
//     macOS, ctrl everywhere else. It is resolved when the keymap is read, so
//     nothing downstream has to know which platform it is on.
package keymap

import (
	"fmt"
	"strconv"
	"strings"
)

// Chord is one keystroke: the modifiers held down, and the key pressed.
type Chord struct {
	Ctrl  bool
	Alt   bool
	Shift bool
	// Cmd is the Command key on macOS, and the Super/Windows key elsewhere --
	// what a browser reports as metaKey.
	Cmd bool

	// Key names a physical key, lowercased: "a", "5", "-", "escape", "f7".
	Key string
}

// String prints the chord in canonical form: modifiers in a fixed order, then
// the key. Parse(String(c)) == c for every chord Parse produces.
func (c Chord) String() string {
	parts := make([]string, 0, 5)
	if c.Ctrl {
		parts = append(parts, "ctrl")
	}
	if c.Alt {
		parts = append(parts, "alt")
	}
	if c.Shift {
		parts = append(parts, "shift")
	}
	if c.Cmd {
		parts = append(parts, "cmd")
	}
	return strings.Join(append(parts, c.Key), "+")
}

// modifierAliases maps every accepted spelling of a modifier onto its canonical
// name. "secondary" is resolved by Parse and so is not in here.
var modifierAliases = map[string]string{
	"ctrl":    "ctrl",
	"control": "ctrl",
	"alt":     "alt",
	"option":  "alt",
	"opt":     "alt",
	"shift":   "shift",
	"cmd":     "cmd",
	"command": "cmd",
	"meta":    "cmd",
	"super":   "cmd",
	"win":     "cmd",
}

// namedKeys are the keys that are spelled out rather than printed.
var namedKeys = map[string]bool{
	"enter":     true,
	"escape":    true,
	"space":     true,
	"tab":       true,
	"backspace": true,
	"delete":    true,
	"insert":    true,
	"home":      true,
	"end":       true,
	"pageup":    true,
	"pagedown":  true,
	"up":        true,
	"down":      true,
	"left":      true,
	"right":     true,
	"capslock":  true,
}

// keyAliases maps other spellings of a key onto the name above.
var keyAliases = map[string]string{
	"esc":        "escape",
	"return":     "enter",
	"del":        "delete",
	"spacebar":   "space",
	"pgup":       "pageup",
	"pgdn":       "pagedown",
	"arrowup":    "up",
	"arrowdown":  "down",
	"arrowleft":  "left",
	"arrowright": "right",
}

// punctuation is every key that carries a symbol on an unshifted US layout.
// They are named by the symbol they produce unshifted, which is the one thing
// about them that does not move between layouts in practice.
const punctuation = "-=[]\\;',./`"

// shiftedNames maps the character a key produces when shifted onto the key
// itself, so a binding that names one can be turned down with a message that
// says what to write instead.
var shiftedNames = map[rune]rune{
	'_': '-',
	'+': '=',
	'{': '[',
	'}': ']',
	'|': '\\',
	':': ';',
	'"': '\'',
	'<': ',',
	'>': '.',
	'?': '/',
	'~': '`',
	'!': '1',
	'@': '2',
	'#': '3',
	'$': '4',
	'%': '5',
	'^': '6',
	'&': '7',
	'*': '8',
	'(': '9',
	')': '0',
}

// Parse reads a written binding -- "secondary+shift+p" -- into a Chord, with
// "secondary" resolved for the given platform.
func Parse(binding string, platform Platform) (Chord, error) {
	written := strings.TrimSpace(strings.ToLower(binding))
	if written == "" {
		return Chord{}, fmt.Errorf("empty binding")
	}

	tokens := strings.Split(written, "+")
	key := tokens[len(tokens)-1]
	modifiers := tokens[:len(tokens)-1]

	var chord Chord
	for _, token := range modifiers {
		name := token
		if name == "secondary" {
			name = platform.Secondary()
		}
		canonical, ok := modifierAliases[name]
		if !ok {
			if token == "" {
				return Chord{}, fmt.Errorf("%q has an empty modifier", binding)
			}
			return Chord{}, fmt.Errorf("%q: unknown modifier %q", binding, token)
		}
		switch canonical {
		case "ctrl":
			chord.Ctrl = true
		case "alt":
			chord.Alt = true
		case "shift":
			chord.Shift = true
		case "cmd":
			chord.Cmd = true
		}
	}

	canonicalKey, err := parseKey(key, binding)
	if err != nil {
		return Chord{}, err
	}
	chord.Key = canonicalKey

	return chord, nil
}

func parseKey(key, binding string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("%q has no key", binding)
	}

	if canonical, ok := keyAliases[key]; ok {
		key = canonical
	}
	if namedKeys[key] {
		return key, nil
	}

	if len(key) > 1 && key[0] == 'f' {
		if n, err := strconv.Atoi(key[1:]); err == nil && n >= 1 && n <= 24 {
			return key, nil
		}
	}

	runes := []rune(key)
	if len(runes) == 1 {
		r := runes[0]
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune(punctuation, r) {
			return key, nil
		}
		if unshifted, ok := shiftedNames[r]; ok {
			return "", fmt.Errorf(
				"%q names %q, which is %q with shift held: write \"shift+%c\" instead",
				binding, key, string(unshifted), unshifted,
			)
		}
	}

	return "", fmt.Errorf("%q: unknown key %q", binding, key)
}
