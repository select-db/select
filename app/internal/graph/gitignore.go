package graph

import (
	"bufio"
	"os"
	"path"
	"strings"
	"sync"
)

// ignoreMatcher decides whether a directory is gitignored.
//
// Only directories are filtered, never files: the shipped .gitignore excludes
// .env, which the app reads for $VARIABLES. Skipping directories is also what
// keeps a large folder off the inotify watch cap.
//
// A matcher is a snapshot: each .gitignore is read once, and a new WorkspaceFS
// is the invalidation point.
//
// Unsupported: .git/info/exclude, core.excludesFile, escaped literals.
type ignoreMatcher struct {
	root string

	mu      sync.Mutex
	byDir   map[string][]ignorePattern
	decided map[string]bool
}

type ignorePattern struct {
	segs []string
	// Set when the pattern holds no metacharacter, so the hot loop can use ==.
	literal  string
	negate   bool
	anchored bool
}

func newIgnoreMatcher(root string) *ignoreMatcher {
	return &ignoreMatcher{
		root:    root,
		byDir:   map[string][]ignorePattern{},
		decided: map[string]bool{},
	}
}

// IgnoresDir reports whether dir is gitignored. rel is dir relative to the
// workspace root, slash-separated; the caller already has it.
//
// Files are read root-first so a deeper .gitignore overrides a shallower one,
// and within a file the last matching pattern wins, which is what makes "!"
// work.
func (m *ignoreMatcher) IgnoresDir(dir, rel string) bool {
	if m == nil || rel == "" || rel == "." {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if was, seen := m.decided[dir]; seen {
		return was
	}

	segments := strings.Split(rel, "/")

	ignored := false
	// Each ancestor's path is a prefix of dir, so slice rather than re-join.
	off := len(m.root)
	for i := range segments {
		if patterns := m.patternsIn(dir[:off]); patterns != nil {
			if decided, isIgnored := matchPatterns(patterns, segments[i:]); decided {
				ignored = isIgnored
			}
		}
		off += 1 + len(segments[i])
	}

	m.decided[dir] = ignored
	return ignored
}

// patternsIn reads dir's .gitignore once, caching "no file here" too.
// Callers hold m.mu.
func (m *ignoreMatcher) patternsIn(dir string) []ignorePattern {
	if patterns, seen := m.byDir[dir]; seen {
		return patterns
	}
	patterns := parseIgnoreFile(dir + string(os.PathSeparator) + ".gitignore")
	m.byDir[dir] = patterns
	return patterns
}

// parseIgnoreFile returns nil when there is no file, or it holds no patterns.
func parseIgnoreFile(path string) []ignorePattern {
	f, err := os.Open(path) // #nosec G304 -- path is a workspace directory joined with a constant name
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var patterns []ignorePattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if p, ok := parseIgnoreLine(scanner.Text()); ok {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

func parseIgnoreLine(line string) (ignorePattern, bool) {
	// Trailing whitespace is not part of a pattern; leading whitespace is.
	line = strings.TrimRight(line, " \t\r")
	if line == "" || strings.HasPrefix(line, "#") {
		return ignorePattern{}, false
	}

	var p ignorePattern
	if strings.HasPrefix(line, "!") {
		p.negate = true
		line = line[1:]
	}
	// "Directories only" is all this matcher is asked about, so a trailing
	// slash carries no information and is dropped.
	line = strings.TrimSuffix(line, "/")

	// A slash anchors the pattern to the .gitignore's directory; without one it
	// matches a name at any depth.
	if strings.HasPrefix(line, "/") {
		p.anchored = true
		line = strings.TrimPrefix(line, "/")
	} else if strings.Contains(line, "/") {
		p.anchored = true
	}
	if line == "" {
		return ignorePattern{}, false
	}

	p.segs = strings.Split(line, "/")
	if len(p.segs) == 1 && !strings.ContainsAny(line, "*?[\\") {
		p.literal = line
	}
	return p, true
}

// matchPatterns reports whether any pattern matched, and the last one's verdict.
func matchPatterns(patterns []ignorePattern, segments []string) (decided, ignored bool) {
	for _, p := range patterns {
		if p.matches(segments) {
			decided, ignored = true, !p.negate
		}
	}
	return decided, ignored
}

// matches covers the path or any directory above it: git never descends into an
// excluded directory, so "node_modules" excludes everything under it.
func (p ignorePattern) matches(segments []string) bool {
	if p.anchored {
		for i := 1; i <= len(segments); i++ {
			if matchSegments(p.segs, segments[:i]) {
				return true
			}
		}
		return false
	}

	// A bare name matches at any depth, so one matching component is enough.
	for _, seg := range segments {
		if p.literal != "" {
			if p.literal == seg {
				return true
			}
			continue
		}
		if ok, err := path.Match(p.segs[0], seg); err == nil && ok {
			return true
		}
	}
	return false
}

// matchSegments matches split pattern against split path. "**" spans any number
// of segments; the rest go through path.Match, whose "*" does not cross a slash.
func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}

	if pattern[0] == "**" {
		for i := 0; i <= len(name); i++ {
			if matchSegments(pattern[1:], name[i:]) {
				return true
			}
		}
		return false
	}

	if len(name) == 0 {
		return false
	}
	if ok, err := path.Match(pattern[0], name[0]); err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], name[1:])
}
