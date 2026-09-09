package graph

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
// Only directories. A gitignored file stays visible and readable, and that is
// deliberate rather than lazy: the .gitignore this app ships ignores .env,
// which the app itself reads for $VARIABLES and offers in the tree. Hiding
// files because git does would break the feature the file exists for.
//
// Directories are where the cost is -- node_modules, target, dist, .venv -- and
// skipping one skips everything under it, which is also what git does: it never
// looks inside an excluded directory, so a negation inside one cannot bring it
// back.
//
// # What of the format is implemented
//
// Comments, blank lines, negation with "!", directory-only patterns with a
// trailing "/", anchoring (a pattern containing a slash is relative to the
// .gitignore holding it, one without matches at any depth), "*" and "?" and
// character classes within a segment, "**" across segments, and last-match-wins
// across the .gitignore files from the root down.
//
// Not implemented: .git/info/exclude, core.excludesFile, and escaped literals
// such as "\#". None of them shows up in the workspace of somebody who wanted a
// directory left alone.

// ignoreMatcher answers whether a directory is excluded, reading the .gitignore
// files above it and caching what it parsed.
type ignoreMatcher struct {
	root string

	mu     sync.Mutex
	byDir  map[string]*ignoreFile
	loaded map[string]time.Time
}

// ignoreFile is one parsed .gitignore, and the directory it governs.
type ignoreFile struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	segs     []string
	negate   bool
	anchored bool
}

func newIgnoreMatcher(root string) *ignoreMatcher {
	return &ignoreMatcher{
		root:   root,
		byDir:  map[string]*ignoreFile{},
		loaded: map[string]time.Time{},
	}
}

// IgnoresDir reports whether dir is excluded by a .gitignore at or above it.
//
// Files are read root-first so a deeper .gitignore overrides a shallower one,
// and within a file the last matching pattern wins, which is what makes "!"
// work.
func (m *ignoreMatcher) IgnoresDir(dir string) bool {
	if m == nil {
		return false
	}

	rel, err := filepath.Rel(m.root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}

	segments := strings.Split(filepath.ToSlash(rel), "/")

	ignored := false
	// Every directory from the root down to dir's parent may hold a .gitignore
	// that has something to say about dir.
	for i := range segments {
		owner := filepath.Join(append([]string{m.root}, segments[:i]...)...)
		file := m.load(owner)
		if file == nil {
			continue
		}
		subject := strings.Join(segments[i:], "/")
		if decided, isIgnored := file.match(subject); decided {
			ignored = isIgnored
		}
	}
	return ignored
}

// load returns the parsed .gitignore in dir, re-reading it when it has changed
// since it was cached. The watcher holds one matcher for a whole session, so a
// .gitignore somebody edits has to take effect without a restart.
func (m *ignoreMatcher) load(dir string) *ignoreFile {
	p := filepath.Join(dir, ".gitignore")

	var mtime time.Time
	if info, err := os.Stat(p); err == nil {
		mtime = info.ModTime()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if was, seen := m.loaded[dir]; seen && was.Equal(mtime) {
		return m.byDir[dir]
	}

	file := parseIgnoreFile(p)
	m.loaded[dir] = mtime
	m.byDir[dir] = file
	return file
}

// parseIgnoreFile returns nil when there is no file, or it holds no patterns.
func parseIgnoreFile(path string) *ignoreFile {
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
	if len(patterns) == 0 {
		return nil
	}
	return &ignoreFile{patterns: patterns}
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
	if line == "" {
		return ignorePattern{}, false
	}

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
	return p, true
}

// match reports whether the file has anything to say about subject, a
// slash-separated path relative to the directory holding it, and what it said.
// Later patterns override earlier ones.
func (f *ignoreFile) match(subject string) (decided, ignored bool) {
	segments := strings.Split(subject, "/")

	for _, p := range f.patterns {
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
