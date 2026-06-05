package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"text/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type SidebarNode struct {
	Label    string         `json:"label"`
	Slug     string         `json:"slug"`
	Path     string         `json:"path,omitempty"`
	HTMLFile string         `json:"htmlFile,omitempty"`
	Children []*SidebarNode `json:"children,omitempty"`
}

type PageData struct {
	Title       string
	Description string
	Sidebar     string
	Content     string
	Styles      string
	Scripts     string
	Canonical   string
	BaseURL     string
	PrevPage    *PageLink
	NextPage    *PageLink
}

type PageLink struct {
	Label string
	Href  string
}

type SearchEntry struct {
	Title string `json:"title"`
	Href  string `json:"href"`
	Body  string `json:"body"`
}

const siteURL = "https://select-db.com"

type buildConfig struct {
	rootDir       string
	docsDir       string
	sidebarPath   string
	templateDir   string
	cssPath       string
	themePath     string
	componentsDir string
	outDir        string
}

func newBuildConfig(rootDir string) buildConfig {
	docsDir := filepath.Join(rootDir, "docs")
	return buildConfig{
		rootDir:       rootDir,
		docsDir:       docsDir,
		sidebarPath:   filepath.Join(docsDir, "sidebar.txt"),
		templateDir:   filepath.Join(docsDir, "template"),
		cssPath:       filepath.Join(docsDir, "docs.css"),
		themePath:     filepath.Join(rootDir, "app", "backend", "graph", "defaults", ".theme"),
		componentsDir: filepath.Join(docsDir, "components"),
		outDir:        filepath.Join(docsDir, "site"),
	}
}

func main() {
	serve := flag.Bool("serve", false, "build, serve on :3333, and watch for changes")
	port := flag.String("port", "3333", "port for dev server")
	flag.Parse()

	rootDir, err := findRepoRoot()
	if err != nil {
		fatal("finding repo root: %v", err)
	}

	cfg := newBuildConfig(rootDir)

	if err := build(cfg); err != nil {
		fatal("%v", err)
	}

	if !*serve {
		return
	}

	// Watch and rebuild in background
	go watch(cfg)

	addr := ":" + *port
	fmt.Printf("serving at http://localhost%s\n", addr)
	// Read first page from _redirects for root redirect
	redirectTarget := "/getting-started/"
	if data, err := os.ReadFile(filepath.Join(cfg.outDir, "_redirects")); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 2 {
			redirectTarget = parts[1]
		}
	}
	fs := http.FileServer(http.Dir(cfg.outDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, redirectTarget, http.StatusFound)
			return
		}
		fs.ServeHTTP(w, r)
	})
	if err := http.ListenAndServe(addr, nil); err != nil {
		fatal("server: %v", err)
	}
}

func build(cfg buildConfig) error {
	tree, err := parseSidebar(cfg.sidebarPath)
	if err != nil {
		return fmt.Errorf("parsing sidebar.txt: %w", err)
	}

	if err := lint(cfg.rootDir, tree, cfg.sidebarPath); err != nil {
		return fmt.Errorf("lint: %w", err)
	}

	pages := collectPages(tree)

	themeCSS, err := os.ReadFile(cfg.themePath)
	if err != nil {
		return fmt.Errorf("reading .theme: %w", err)
	}

	docsCSS, err := os.ReadFile(cfg.cssPath)
	if err != nil {
		return fmt.Errorf("reading docs.css: %w", err)
	}

	combinedCSS := string(themeCSS) + "\n" + string(docsCSS)
	styles := string(`<link rel="stylesheet" href="/style.css">`)

	scriptsContent, err := bundleComponents(cfg.componentsDir)
	if err != nil {
		return fmt.Errorf("bundling components: %w", err)
	}
	scripts := string(`<script type="module" defer src="/bundle.js"></script>`)

	tmpl, err := template.ParseGlob(filepath.Join(cfg.templateDir, "*.html"))
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	if err := os.RemoveAll(cfg.outDir); err != nil {
		return fmt.Errorf("cleaning output dir: %w", err)
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "style.css"), []byte(combinedCSS), 0o644); err != nil {
		return fmt.Errorf("writing style.css: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "bundle.js"), []byte(scriptsContent), 0o644); err != nil {
		return fmt.Errorf("writing bundle.js: %w", err)
	}
	themeJS, err := os.ReadFile(filepath.Join(cfg.docsDir, "theme.js"))
	if err != nil {
		return fmt.Errorf("reading theme.js: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "theme.js"), themeJS, 0o644); err != nil {
		return fmt.Errorf("writing theme.js: %w", err)
	}
	favicon, err := os.ReadFile(filepath.Join(cfg.docsDir, "favicon.png"))
	if err != nil {
		return fmt.Errorf("reading favicon: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "favicon.png"), favicon, 0o644); err != nil {
		return fmt.Errorf("writing favicon: %w", err)
	}
	logo, err := os.ReadFile(filepath.Join(cfg.docsDir, "logo.png"))
	if err != nil {
		return fmt.Errorf("reading logo: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "logo.png"), logo, 0o644); err != nil {
		return fmt.Errorf("writing logo: %w", err)
	}

	var searchIndex []SearchEntry

	for i, page := range pages {
		src, err := os.ReadFile(filepath.Join(cfg.rootDir, page.Path))
		if err != nil {
			return fmt.Errorf("reading %s: %w", page.Path, err)
		}

		var buf bytes.Buffer
		if err := md.Convert(src, &buf); err != nil {
			return fmt.Errorf("converting %s: %w", page.Path, err)
		}

		var prev, next *PageLink
		if i > 0 {
			prev = &PageLink{Label: pages[i-1].Label, Href: "/" + pages[i-1].HTMLFile + "/"}
		}
		if i < len(pages)-1 {
			next = &PageLink{Label: pages[i+1].Label, Href: "/" + pages[i+1].HTMLFile + "/"}
		}

		data := PageData{
			Title:       page.Label,
			Description: extractDescription(string(src)),
			Sidebar:     renderSidebar(tree, page.HTMLFile),
			Content:     string(indentHTML(addHeadingAnchors(buf.String()), "        ")),
			Styles:      styles,
			Scripts:     string(scripts),
			Canonical:   siteURL + "/" + page.HTMLFile + "/",
			BaseURL:     siteURL,
			PrevPage:    prev,
			NextPage:    next,
		}

		outPath := filepath.Join(cfg.outDir, page.HTMLFile, "index.html")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("creating dir for %s: %w", outPath, err)
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", outPath, err)
		}
		if err := tmpl.ExecuteTemplate(f, "base.html", data); err != nil {
			f.Close()
			return fmt.Errorf("rendering %s: %w", page.HTMLFile, err)
		}
		f.Close()

		for _, entry := range buildSearchEntries(string(src), page.Label, "/"+page.HTMLFile+"/") {
			searchIndex = append(searchIndex, entry)
		}
	}

	if len(pages) > 0 {
		// Cloudflare Pages _redirects file (proper 301 instead of meta refresh)
		redirect := fmt.Sprintf("/ /%s/ 301\n", pages[0].HTMLFile)
		os.WriteFile(filepath.Join(cfg.outDir, "_redirects"), []byte(redirect), 0o644)
	}

	searchJSON, _ := json.Marshal(searchIndex)
	os.WriteFile(filepath.Join(cfg.outDir, "search-index.json"), searchJSON, 0o644)
	writeSitemap(cfg.outDir, pages)
	os.WriteFile(filepath.Join(cfg.outDir, "robots.txt"), []byte("User-agent: *\nAllow: /\nSitemap: "+siteURL+"/sitemap.xml\n"), 0o644)
	writeLLMsTxt(cfg, pages)
	writeLLMsFullTxt(cfg, pages)

	headers := `/*
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  Referrer-Policy: strict-origin-when-cross-origin
  Strict-Transport-Security: max-age=31536000; includeSubDomains
`
	os.WriteFile(filepath.Join(cfg.outDir, "_headers"), []byte(headers), 0o644)

	fmt.Printf("built %d pages in %s\n", len(pages), cfg.outDir)

	if err := verifyLinks(cfg.outDir); err != nil {
		return err
	}

	return nil
}

// watch polls for file changes and rebuilds when something changes
func watch(cfg buildConfig) {
	watchDirs := []string{
		cfg.docsDir,
	}

	lastMod := time.Now()

	for {
		time.Sleep(500 * time.Millisecond)

		var trigger string

		for _, dir := range watchDirs {
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && (info.Name() == "site" || info.Name() == "generate") {
					return filepath.SkipDir
				}
				if !info.IsDir() && info.ModTime().After(lastMod) {
					trigger = path
					return filepath.SkipAll
				}
				return nil
			})
			if trigger != "" {
				break
			}
		}

		// Also watch .doc.md files outside docs/
		if trigger == "" {
			filepath.Walk(cfg.rootDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "site" || info.Name() == "docs") {
					return filepath.SkipDir
				}
				if strings.HasSuffix(path, ".doc.md") && info.ModTime().After(lastMod) {
					trigger = path
					return filepath.SkipAll
				}
				return nil
			})
		}

		if trigger == "" {
			continue
		}

		fmt.Printf("change detected: %s, rebuilding...\n", trigger)
		lastMod = time.Now()
		if err := build(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "rebuild error: %v\n", err)
		}
	}
}

func parseSidebar(path string) ([]*SidebarNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var root []*SidebarNode

	type stackEntry struct {
		indent int
		node   *SidebarNode
	}
	var stack []stackEntry

	for lineNum, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		indent := countIndent(line)
		trimmed := strings.TrimSpace(line)

		var label, filePath string
		if idx := strings.LastIndex(trimmed, ":"); idx != -1 {
			label = strings.TrimSpace(trimmed[:idx])
			filePath = strings.TrimSpace(trimmed[idx+1:])
		} else {
			label = trimmed
		}

		if label == "" {
			return nil, fmt.Errorf("sidebar.txt:%d: empty label", lineNum+1)
		}

		node := &SidebarNode{
			Label: label,
			Slug:  slugify(label),
			Path:  filePath,
		}

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			root = append(root, node)
		} else {
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, node)
		}

		stack = append(stack, stackEntry{indent: indent, node: node})
	}

	computeHTMLPaths(root, "")
	return root, nil
}

func computeHTMLPaths(nodes []*SidebarNode, prefix string) {
	for _, n := range nodes {
		p := n.Slug
		if prefix != "" {
			p = prefix + "/" + n.Slug
		}
		if n.Path != "" {
			n.HTMLFile = p
		}
		computeHTMLPaths(n.Children, p)
	}
}

func collectPages(nodes []*SidebarNode) []*SidebarNode {
	var pages []*SidebarNode
	var walk func([]*SidebarNode)
	walk = func(ns []*SidebarNode) {
		for _, n := range ns {
			if n.Path != "" {
				pages = append(pages, n)
			}
			walk(n.Children)
		}
	}
	walk(nodes)
	return pages
}

func renderSidebar(tree []*SidebarNode, activePath string) string {
	var b strings.Builder
	ind := strings.Repeat("  ", 4)
	b.WriteString(ind + "<ul>\n")
	renderSidebarNodes(&b, tree, activePath, 5)
	b.WriteString(ind + "</ul>\n")
	return string(b.String())
}

func renderSidebarNodes(b *strings.Builder, nodes []*SidebarNode, activePath string, depth int) {
	ind := strings.Repeat("  ", depth)
	for _, n := range nodes {
		hasChildren := len(n.Children) > 0
		isActive := n.HTMLFile == activePath

		if hasChildren {
			b.WriteString(fmt.Sprintf("%s<li>\n", ind))
			if n.Path != "" {
				activeClass := ""
				if isActive {
					activeClass = ` class="active"`
				}
				b.WriteString(fmt.Sprintf("%s  <a href=\"/%s/\"%s>%s</a>\n", ind, n.HTMLFile, activeClass, n.Label))
			} else {
				b.WriteString(fmt.Sprintf("%s  <span class=\"section\">%s</span>\n", ind, n.Label))
			}
			b.WriteString(fmt.Sprintf("%s  <ul>\n", ind))
			renderSidebarNodes(b, n.Children, activePath, depth+2)
			b.WriteString(fmt.Sprintf("%s  </ul>\n", ind))
			b.WriteString(fmt.Sprintf("%s</li>\n", ind))
		} else if n.Path != "" {
			activeClass := ""
			if isActive {
				activeClass = ` class="active"`
			}
			b.WriteString(fmt.Sprintf("%s<li><a href=\"/%s/\"%s>%s</a></li>\n", ind, n.HTMLFile, activeClass, n.Label))
		} else {
			b.WriteString(fmt.Sprintf("%s<li><span>%s</span></li>\n", ind, n.Label))
		}
	}
}

func nodeContains(n *SidebarNode, path string) bool {
	if n.HTMLFile == path {
		return true
	}
	for _, c := range n.Children {
		if nodeContains(c, path) {
			return true
		}
	}
	return false
}

func lint(rootDir string, tree []*SidebarNode, sidebarPath string) error {
	pages := collectPages(tree)
	seen := make(map[string]bool)

	for _, p := range pages {
		abs := filepath.Join(rootDir, p.Path)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			return fmt.Errorf("%s: file not found: %s", sidebarPath, p.Path)
		}
		if seen[p.Path] {
			return fmt.Errorf("%s: duplicate file reference: %s", sidebarPath, p.Path)
		}
		seen[p.Path] = true
	}

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "site") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".doc.md") {
			rel, _ := filepath.Rel(rootDir, path)
			if !seen[rel] {
				fmt.Fprintf(os.Stderr, "warning: orphan doc file not in sidebar.txt: %s\n", rel)
			}
		}
		return nil
	})

	return nil
}

func bundleComponents(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var b strings.Builder
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("// --- %s ---\n", e.Name()))
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String(), nil
}

func writeSitemap(outDir string, pages []*SidebarNode) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		b.WriteString(fmt.Sprintf("  <url><loc>%s/%s/</loc></url>\n", siteURL, p.HTMLFile))
	}
	b.WriteString("</urlset>\n")
	os.WriteFile(filepath.Join(outDir, "sitemap.xml"), []byte(b.String()), 0o644)
}

func writeLLMsTxt(cfg buildConfig, pages []*SidebarNode) {
	var b strings.Builder
	b.WriteString("# SELECT\n\n")
	b.WriteString("> Database management tool for developers who prefer working close to SQL.\n\n")
	b.WriteString("## Docs\n\n")
	for _, p := range pages {
		src, _ := os.ReadFile(filepath.Join(cfg.rootDir, p.Path))
		desc := extractDescription(string(src))
		b.WriteString(fmt.Sprintf("- [%s](/%s/): %s\n", p.Label, p.HTMLFile, desc))
	}
	os.WriteFile(filepath.Join(cfg.outDir, "llms.txt"), []byte(b.String()), 0o644)
}

func writeLLMsFullTxt(cfg buildConfig, pages []*SidebarNode) {
	var b strings.Builder
	b.WriteString("# SELECT\n\n")
	b.WriteString("> Database management tool for developers who prefer working close to SQL.\n\n")
	for _, p := range pages {
		src, _ := os.ReadFile(filepath.Join(cfg.rootDir, p.Path))
		b.WriteString(fmt.Sprintf("---\n\n## %s\n\n", p.Label))
		b.WriteString(string(src))
		b.WriteString("\n\n")
	}
	os.WriteFile(filepath.Join(cfg.outDir, "llms-full.txt"), []byte(b.String()), 0o644)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func countIndent(line string) int {
	count := 0
	for _, c := range line {
		if c == ' ' {
			count++
		} else if c == '\t' {
			count += 2
		} else {
			break
		}
	}
	return count
}

var headingRe = regexp.MustCompile(`(</h[1-3]>)`)
var headingIDRe = regexp.MustCompile(`<h[1-3]\s+id="([^"]+)"`)

func addHeadingAnchors(s string) string {
	ids := headingIDRe.FindAllStringSubmatch(s, -1)
	i := 0
	return headingRe.ReplaceAllStringFunc(s, func(closing string) string {
		if i >= len(ids) {
			return closing
		}
		id := ids[i][1]
		i++
		return fmt.Sprintf(` <a class="anchor" href="#%s">#</a>%s`, id, closing)
	})
}

func indentHTML(s string, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	inPre := false
	for i, line := range lines {
		if strings.Contains(line, "<pre") {
			inPre = true
		}
		if !inPre && line != "" {
			lines[i] = prefix + line
		}
		if strings.Contains(line, "</pre>") {
			inPre = false
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// buildSearchEntries splits markdown into sections by heading.
// Each entry gets a title like "Page > Section > Subsection" and an href with anchor.
func buildSearchEntries(md string, pageTitle string, baseHref string) []SearchEntry {
	lines := strings.Split(md, "\n")
	var entries []SearchEntry

	type heading struct {
		level int
		text  string
		slug  string
	}
	var stack []heading
	var bodyLines []string
	inCodeBlock := false

	flush := func() {
		body := stripMarkdown(strings.Join(bodyLines, "\n"))
		if strings.TrimSpace(body) == "" {
			bodyLines = nil
			return
		}
		if len(stack) == 0 {
			entries = append(entries, SearchEntry{
				Title: pageTitle,
				Href:  baseHref,
				Body:  body,
			})
		} else {
			parts := []string{pageTitle}
			// Skip h1 if it matches page title
			for _, h := range stack {
				if h.level == 1 && h.text == pageTitle {
					continue
				}
				parts = append(parts, h.text)
			}
			title := strings.Join(parts, " > ")
			anchor := stack[len(stack)-1].slug
			entries = append(entries, SearchEntry{
				Title: title,
				Href:  baseHref + "#" + anchor,
				Body:  body,
			})
		}
		bodyLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			bodyLines = append(bodyLines, line)
			continue
		}

		level := 0
		if strings.HasPrefix(trimmed, "### ") {
			level = 3
		} else if strings.HasPrefix(trimmed, "## ") {
			level = 2
		} else if strings.HasPrefix(trimmed, "# ") {
			level = 1
		}

		if level > 0 {
			flush()
			text := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, heading{level: level, text: text, slug: slugify(text)})
		} else {
			bodyLines = append(bodyLines, line)
		}
	}
	flush()

	return entries
}

var hrefRe = regexp.MustCompile(`href="(/[^"#]*)"`)

func verifyLinks(outDir string) error {
	// Collect all valid paths
	valid := make(map[string]bool)
	valid["/"] = true
	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(outDir, path)
		valid["/"+filepath.ToSlash(rel)] = true
		// /foo/index.html is reachable as /foo/ and /foo
		if strings.HasSuffix(rel, "/index.html") {
			dir := "/" + filepath.ToSlash(strings.TrimSuffix(rel, "/index.html")) + "/"
			valid[dir] = true
			valid[strings.TrimSuffix(dir, "/")] = true
		}
		return nil
	})

	var broken []string

	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(outDir, path)
		matches := hrefRe.FindAllSubmatch(data, -1)
		for _, m := range matches {
			href := string(m[1])
			if !valid[href] {
				broken = append(broken, fmt.Sprintf("  %s -> %s", rel, href))
			}
		}
		return nil
	})

	if len(broken) > 0 {
		return fmt.Errorf("broken links found:\n%s", strings.Join(broken, "\n"))
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func extractDescription(md string) string {
	lines := strings.Split(md, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(trimmed) > 160 {
			return trimmed[:157] + "..."
		}
		return trimmed
	}
	return ""
}

var linkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
var tableSepRe = regexp.MustCompile(`\|?[-:]+\|[-|:]+\|?`)

func stripMarkdown(s string) string {
	s = linkRe.ReplaceAllString(s, "$1")
	s = tableSepRe.ReplaceAllString(s, "")
	for _, c := range []string{"#", "*", "`", "|", ">"} {
		s = strings.ReplaceAll(s, c, "")
	}
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a git repository")
		}
		dir = parent
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
