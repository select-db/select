package keymap

import (
	"fmt"
	"sort"
	"strings"
)

// Written is one binding as it appears in a config file.
type Written struct {
	Key string `json:"key"`
	// Command is what the binding runs. Empty unbinds the chord: it shadows
	// every binding for the same chord written before it, and does nothing.
	Command string `json:"command"`
	When    string `json:"when,omitempty"`
}

// Categories groups written bindings the way the config file does, by the part
// of the app they belong to: "workbench", "editor", "modal", "menu".
type Categories map[string][]Written

// Binding is a written binding once its chord has been parsed and resolved for
// a platform. Key is canonical, so matching a keystroke is a string comparison.
type Binding struct {
	Key     string `json:"key"`
	Command string `json:"command"`
	When    string `json:"when,omitempty"`
}

// Problem is something wrong with a binding, kept rather than dropped in
// silence: a binding that cannot be parsed does nothing at all, and a keymap
// that says nothing about it leaves a person pressing a key that will never
// work.
type Problem struct {
	// Level is "error" for a binding that was dropped, "migrated" for one that
	// was read as something else.
	Level   string `json:"level"`
	Source  string `json:"source"`
	Key     string `json:"key"`
	Command string `json:"command,omitempty"`
	Message string `json:"message"`
}

const (
	LevelError    = "error"
	LevelMigrated = "migrated"

	SourceDefaults = "defaults"
	SourceUser     = "user"
)

// categoryPrecedence lists the categories in increasing precedence. A keystroke
// is matched against the last binding that fits, so a category later in this
// list wins over an earlier one: a workbench binding beats an editor binding on
// the same chord, and anything in the user's own file beats both.
var categoryPrecedence = []string{"menu", "modal", "editor", "workbench"}

// Resolve turns the written keymaps into the flat list the app matches against:
// defaults first, then the user's own, each category in precedence order, every
// chord parsed and resolved for this platform.
//
// Bindings that cannot be parsed are left out and reported. Nothing else is:
// two bindings on one chord are both kept, and the later one wins, which is how
// a person overrides a default by writing their own.
func Resolve(defaults, user Categories, platform Platform) ([]Binding, []Problem) {
	bindings, problems := resolve(SourceDefaults, defaults, platform)
	userBindings, userProblems := resolve(SourceUser, user, platform)

	return append(bindings, userBindings...), append(problems, userProblems...)
}

func resolve(source string, categories Categories, platform Platform) ([]Binding, []Problem) {
	var bindings []Binding
	var problems []Problem

	for _, category := range orderCategories(categories) {
		for _, written := range categories[category] {
			key := written.Key
			// Only a personal keymap can be old enough to need it; the defaults
			// ship with the binary that reads them.
			if source == SourceUser {
				migrated, problem := migrate(written, platform)
				if problem != nil {
					problems = append(problems, *problem)
				}
				key = migrated
			}

			chord, err := Parse(key, platform)
			if err != nil {
				problems = append(problems, Problem{
					Level:   LevelError,
					Source:  source,
					Key:     written.Key,
					Command: written.Command,
					Message: err.Error(),
				})
				continue
			}

			bindings = append(bindings, Binding{
				Key:     chord.String(),
				Command: written.Command,
				When:    written.When,
			})
		}
	}

	return bindings, problems
}

// orderCategories lists the categories present in increasing precedence:
// the ones this package does not know about first, then the known ones in
// categoryPrecedence order.
func orderCategories(categories Categories) []string {
	known := make(map[string]bool, len(categoryPrecedence))
	for _, name := range categoryPrecedence {
		known[name] = true
	}

	var unknown []string
	for name := range categories {
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)

	ordered := unknown
	for _, name := range categoryPrecedence {
		if _, ok := categories[name]; ok {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

// migrate reads a binding written before "cmd" meant the Command key.
//
// It used to mean "the shortcut key on this platform", which is what
// "secondary" means now, and a personal keymap written against the old meaning
// would otherwise start asking for the Super key on Linux and Windows -- a key
// the window manager usually takes first. On macOS the two mean the same thing
// and nothing is rewritten.
func migrate(written Written, platform Platform) (string, *Problem) {
	if platform == MacOS {
		return written.Key, nil
	}

	tokens := strings.Split(strings.ToLower(strings.TrimSpace(written.Key)), "+")
	if len(tokens) < 2 {
		return written.Key, nil
	}

	rewritten := false
	for i, token := range tokens[:len(tokens)-1] {
		if token == "cmd" || token == "command" {
			tokens[i] = "secondary"
			rewritten = true
		}
	}
	if !rewritten {
		return written.Key, nil
	}

	key := strings.Join(tokens, "+")
	return key, &Problem{
		Level:   LevelMigrated,
		Source:  SourceUser,
		Key:     written.Key,
		Command: written.Command,
		Message: fmt.Sprintf(
			"%q now names the Command key; read as %q, the shortcut key on this platform",
			written.Key, key,
		),
	}
}
