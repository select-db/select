package keymap

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func keys(bindings []Binding) []string {
	out := make([]string, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, b.Key+" -> "+b.Command)
	}
	return out
}

// The list is matched from the end, so precedence is position: a user's own
// binding comes after the defaults, and a workbench binding after an editor one.
func TestResolveOrdersByPrecedence(t *testing.T) {
	defaults := Categories{
		"editor":    {{Key: "secondary+k", Command: "editor.k"}},
		"workbench": {{Key: "secondary+k", Command: "workbench.k"}},
		"menu":      {{Key: "secondary+k", Command: "menu.k"}},
	}
	user := Categories{
		"workbench": {{Key: "secondary+k", Command: "user.k"}},
	}

	bindings, problems := Resolve(defaults, user, Linux)
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}

	want := []string{
		"ctrl+k -> menu.k",
		"ctrl+k -> editor.k",
		"ctrl+k -> workbench.k",
		"ctrl+k -> user.k",
	}
	got := keys(bindings)
	if len(got) != len(want) {
		t.Fatalf("Resolve = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("binding %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A category this package does not know about is still loaded, at the lowest
// precedence: a keymap is a person's file, not a closed set.
func TestResolveKeepsUnknownCategories(t *testing.T) {
	bindings, problems := Resolve(
		Categories{"terminal": {{Key: "secondary+k", Command: "terminal.clear"}}},
		nil,
		Linux,
	)
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
	if len(bindings) != 1 || bindings[0].Command != "terminal.clear" {
		t.Fatalf("Resolve = %v, want the terminal binding", keys(bindings))
	}
}

func TestResolveReportsUnparseableBindings(t *testing.T) {
	bindings, problems := Resolve(
		Categories{"workbench": {
			{Key: "secondary+p", Command: "workbench.openSearch"},
			{Key: "ctrl+_", Command: "workbench.nextTabInGroup"},
		}},
		nil,
		Linux,
	)

	if len(bindings) != 1 || bindings[0].Command != "workbench.openSearch" {
		t.Errorf("Resolve kept %v, want only the binding that parses", keys(bindings))
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want one", problems)
	}
	if problems[0].Level != LevelError || problems[0].Key != "ctrl+_" {
		t.Errorf("problem = %+v, want an error naming ctrl+_", problems[0])
	}
}

// "cmd" used to mean "the shortcut key on this platform". A personal keymap
// written then still means that, and is read that way -- once, out loud.
func TestResolveMigratesLegacyCmdOffMacOS(t *testing.T) {
	user := Categories{"workbench": {
		{Key: "cmd+shift+p", Command: "workbench.openSearch"},
		{Key: "super+p", Command: "workbench.super"},
	}}

	bindings, problems := Resolve(nil, user, Linux)

	if len(bindings) != 2 {
		t.Fatalf("Resolve = %v, want both bindings", keys(bindings))
	}
	if bindings[0].Key != "ctrl+shift+p" {
		t.Errorf("legacy cmd binding = %q, want ctrl+shift+p", bindings[0].Key)
	}
	// Super was always the Super key, and is left alone.
	if bindings[1].Key != "cmd+p" {
		t.Errorf("super binding = %q, want cmd+p", bindings[1].Key)
	}

	if len(problems) != 1 || problems[0].Level != LevelMigrated {
		t.Fatalf("problems = %+v, want one migration", problems)
	}
	if !strings.Contains(problems[0].Message, "secondary") {
		t.Errorf("migration message = %q, want it to name secondary", problems[0].Message)
	}
}

func TestResolveLeavesMacOSKeymapsAlone(t *testing.T) {
	user := Categories{"workbench": {{Key: "cmd+shift+p", Command: "workbench.openSearch"}}}

	bindings, problems := Resolve(nil, user, MacOS)

	if len(problems) != 0 {
		t.Errorf("problems = %+v, want none: cmd already means cmd here", problems)
	}
	if bindings[0].Key != "shift+cmd+p" {
		t.Errorf("binding = %q, want shift+cmd+p", bindings[0].Key)
	}
}

// An empty command unbinds: it is kept, so that matching stops on it rather
// than falling through to the default it was meant to take away.
func TestResolveKeepsUnbindings(t *testing.T) {
	bindings, problems := Resolve(
		Categories{"workbench": {{Key: "secondary+p", Command: "workbench.openSearch"}}},
		Categories{"workbench": {{Key: "secondary+p", Command: ""}}},
		Linux,
	)
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
	if len(bindings) != 2 || bindings[1].Command != "" {
		t.Fatalf("Resolve = %v, want the unbinding last", keys(bindings))
	}
}

// The browser names keys too -- it is the one holding the keystroke -- so the
// name it produces and the name a binding is written with have to be the same
// word. They meet here: every name the frontend can emit is parsed, and a name
// that has drifted apart fails rather than silently matching nothing.
func TestFrontendKeyNamesAreTheOnesBindingsUse(t *testing.T) {
	const frontend = "../../frontend/src/lib/stores/keybindingsStore.ts"

	source, err := os.ReadFile(frontend)
	if err != nil {
		t.Fatalf("read %s: %v -- if the keymap store moved, point this test at it", frontend, err)
	}

	table := regexp.MustCompile(`(?s)const keyByCode: Record<string, string> = \{(.*?)\n\};`)
	block := table.FindSubmatch(source)
	if block == nil {
		t.Fatalf("no keyByCode table in %s -- if it was renamed, point this test at it", frontend)
	}

	// Values are quoted either way round -- "'" has to be, since it is a quote.
	names := regexp.MustCompile(`:\s*(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")`).
		FindAllStringSubmatch(string(block[1]), -1)
	if len(names) < len(namedKeys) {
		t.Fatalf("found %d key names in the keyByCode table, which cannot be all of them", len(names))
	}

	for _, match := range names {
		name := strings.ReplaceAll(match[1]+match[2], `\\`, `\`)
		if _, err := Parse(name, Linux); err != nil {
			t.Errorf("the frontend can send %q, which no binding can name: %v", name, err)
		}
	}
}
