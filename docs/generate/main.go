package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	// Href marks a plain link node (a sidebar.txt value that is a URL, e.g.
	// /api/, rather than a .doc.md). It renders as an external link that opens
	// in a new tab, not a generated page.
	Href     string         `json:"href,omitempty"`
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
	cacheDir      string
	openAPIPath   string
	outDir        string
}

// Scalar bundle fetched (and cached) at build time for the API reference. Pinned
// for reproducibility; bump the version to upgrade. Fetching at build keeps the
// bundle out of the repo while the built site still serves it same-origin.
const (
	scalarVersion   = "1.36.2"
	scalarBundleURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@" + scalarVersion + "/dist/browser/standalone.js"
)

// externalIcon is the "open in new tab" glyph appended to link (Href) sidebar
// items so a new-tab jump is visually distinct from in-site navigation.
const externalIcon = ` <svg class="external-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="margin-left:0.3em;vertical-align:-0.1em;opacity:0.65"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>`

// apiReferencePageTmpl is the standalone, full-viewport Scalar page served at
// /api/ (outside the docs layout so Scalar owns the whole screen). The %s is the
// site's .theme CSS, inlined so the reference shares the docs design tokens; a
// mapping layer binds Scalar's --scalar-* variables to those tokens (theme:
// 'none' disables Scalar's built-in palette so ours wins). Light/dark follows
// the docs' stored preference (localStorage doc-theme, same origin); Scalar's
// own toggle is hidden so it can't drift from that.
const apiReferencePageTmpl = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Select API Reference</title>
  <link rel="icon" href="/favicon.png" />
  <style>
%s
    /* --ff lives in docs.css, not .theme; redeclare it here. */
    :root { --ff: system-ui, -apple-system, sans-serif; }
    /* Bind Scalar's tokens to ours. Our color tokens are keyed on
       :root[data-theme], so a single mapping serves both light and dark; the
       mode classes are included so it wins over Scalar's own definitions. */
    :root, .light-mode, .dark-mode {
      --scalar-background-1: var(--gray-100);
      --scalar-background-2: var(--gray-200);
      --scalar-background-3: var(--gray-300);
      --scalar-color-1: var(--gray-1000);
      --scalar-color-2: var(--gray-800);
      --scalar-color-3: var(--gray-700);
      --scalar-color-accent: var(--red);
      --scalar-border-color: var(--border-color);
      --scalar-sidebar-background-1: var(--gray-100);
      --scalar-sidebar-border-color: var(--border-color);
      --scalar-sidebar-color-1: var(--gray-1000);
      --scalar-sidebar-color-2: var(--gray-800);
      --scalar-sidebar-color-active: var(--red);
      --scalar-sidebar-item-hover-background: var(--gray-300);
      --scalar-sidebar-item-active-background: var(--gray-300);
      --scalar-sidebar-search-background: var(--gray-200);
      --scalar-sidebar-search-border-color: var(--border-color);
      --scalar-font: var(--ff);
      --scalar-font-code: ui-monospace, monospace;
      --scalar-radius: var(--br-sm);
      --scalar-radius-lg: var(--br-md);
    }
    html, body { height: 100%; }
    body { margin: 0; background: var(--gray-100); color: var(--gray-1000); }
    #app { height: 100%; }
    /* SELECT logo pinned to the TOP OF SCALAR'S SIDEBAR (like the docs header),
       not a full-width bar. Its width tracks Scalar's sidebar column, and the
       sidebar is padded so its search/nav start below the logo. Sizes match the
       docs (.logo-icon 22px, .logo-text 18px). */
    .api-topbar {
      position: fixed; top: 0; left: 0; z-index: 10; box-sizing: border-box;
      width: var(--scalar-sidebar-width, 280px); height: 3.25rem;
      display: flex; align-items: center; padding: 0 1rem;
      background: var(--gray-100); border-bottom: var(--bw) solid var(--border-color);
    }
    .api-topbar .logo { display: inline-flex; align-items: center; gap: 0.5rem; color: var(--gray-1000); text-decoration: none; }
    .api-topbar .logo-icon { width: 22px; height: 22px; border-radius: var(--br-xs); display: block; }
    .api-topbar .logo-text { height: 18px; width: auto; display: block; }
    .t-doc__sidebar { padding-top: 3.25rem; }
  </style>
  <script>
    (function () {
      var stored = localStorage.getItem('doc-theme');
      var system = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
      document.documentElement.setAttribute('data-theme', stored || system);
    })();
  </script>
</head>
<body>
  <header class="api-topbar">
    <a href="/" class="logo" title="Back to docs">
      <img src="/logo.png" alt="" class="logo-icon" />
      <svg class="logo-text" viewBox="0 -880 4487 1024" fill="currentColor" aria-label="Select"><g transform="scale(1, -1)"><path transform="translate(0, 0)" d="M710 525Q710 497 690 478Q671 458 643 458H243Q224 457 211.5 449.5Q199 442 193.0 432.5Q187 423 187 413Q187 403 193.0 393.5Q199 384 211.5 376.5Q224 369 243 368H527Q597 366 642.5 335.5Q688 305 707.5 265.5Q727 226 727 185Q727 145 707.0 105.5Q687 66 642.0 35.5Q597 5 527 5H521Q515 4 509 4H131Q103 4 83.5 23.5Q64 43 64 71Q64 99 84 118Q103 138 131 138H533Q552 137 564.5 145.0Q577 153 583.0 163.5Q589 174 589 185Q589 192 583.0 204.5Q577 217 564.5 225.0Q552 233 533 234H249Q179 236 133.5 266.0Q88 296 68.5 334.5Q49 373 49 413Q49 452 69.0 490.5Q89 529 134.0 559.0Q179 589 249 591H255Q261 592 267 592H643Q671 592 690.5 572.5Q710 553 710 525Z"/><path transform="translate(776, 0)" d="M249 592H295H635Q663 591 681.5 571.0Q700 551 700 523Q695 463 635 458H283Q245 456 220.0 429.0Q195 402 183.5 367.5Q172 333 172 297Q172 261 183.5 226.5Q195 192 220.0 165.0Q245 138 283 136H635Q663 135 681.5 115.0Q700 95 700 67Q699 40 680.5 21.5Q662 3 635 2H268H249Q179 6 133.5 55.5Q88 105 68.5 168.0Q49 231 49 297Q49 363 69.0 426.0Q89 489 134.0 538.5Q179 588 249 592ZM698 301Q698 273 678 254Q659 234 631 234H329Q301 234 281.5 253.5Q262 273 262 301Q262 329 281 349Q301 368 329 368H631Q659 368 678.5 348.5Q698 329 698 301Z"/><path transform="translate(1513, 0)" d="M130 608Q158 608 178.0 589.5Q198 571 199 543V302Q198 261 204.5 226.5Q211 192 236.0 165.0Q261 138 299 136H619Q647 135 665.5 115.0Q684 95 684 67Q683 40 664.5 21.5Q646 3 619 2H265Q195 6 149.5 49.5Q104 93 84 156Q65 219 64 290V543Q65 570 84.0 588.5Q103 607 130 608Z"/><path transform="translate(2229, 0)" d="M249 592H295H635Q663 591 681.5 571.0Q700 551 700 523Q695 463 635 458H283Q245 456 220.0 429.0Q195 402 183.5 367.5Q172 333 172 297Q172 261 183.5 226.5Q195 192 220.0 165.0Q245 138 283 136H635Q663 135 681.5 115.0Q700 95 700 67Q699 40 680.5 21.5Q662 3 635 2H268H249Q179 6 133.5 55.5Q88 105 68.5 168.0Q49 231 49 297Q49 363 69.0 426.0Q89 489 134.0 538.5Q179 588 249 592ZM698 301Q698 273 678 254Q659 234 631 234H329Q301 234 281.5 253.5Q262 273 262 301Q262 329 281 349Q301 368 329 368H631Q659 368 678.5 348.5Q698 329 698 301Z"/><path transform="translate(2966, 0)" d="M291 458Q253 456 227.5 429.0Q202 402 191.0 367.5Q180 333 180 297Q180 261 190.5 226.0Q201 191 227.0 164.5Q253 138 291 136H621Q684 130 690 67Q689 39 669.0 20.5Q649 2 621 2H302H256Q186 6 141.0 55.5Q96 105 76.5 168.0Q57 231 57 297Q57 346 68.0 394.0Q79 442 102.5 485.5Q126 529 165.5 560.0Q205 591 256 592H275H621Q684 586 690 523Q690 495 669.5 476.5Q649 458 621 458Z"/><path transform="translate(3685, 0)" d="M44 525Q44 553 63.5 572.5Q83 592 111 592H693Q721 592 741 573Q760 553 760 525Q760 497 740.5 477.5Q721 458 693 458H470V53Q464 -7 404 -12Q376 -12 355.5 6.5Q335 25 335 53V458H111Q83 458 64 478Q44 497 44 525Z"/></g></svg>
    </a>
  </header>
  <div id="app"></div>
  <script src="/api/scalar.standalone.js"></script>
  <script>
    var dark = document.documentElement.getAttribute('data-theme') === 'dark';
    Scalar.createApiReference('#app', {
      url: '/api/openapi.json',
      theme: 'none',
      darkMode: dark,
      hideDarkModeToggle: true,
    });
  </script>
</body>
</html>
`

func newBuildConfig(rootDir string) buildConfig {
	docsDir := filepath.Join(rootDir, "docs")
	return buildConfig{
		rootDir:       rootDir,
		docsDir:       docsDir,
		sidebarPath:   filepath.Join(docsDir, "sidebar.txt"),
		templateDir:   filepath.Join(docsDir, "template"),
		cssPath:       filepath.Join(docsDir, "docs.css"),
		themePath:     filepath.Join(rootDir, "app", "internal", "graph", "defaults", "user", ".theme"),
		componentsDir: filepath.Join(docsDir, "components"),
		cacheDir:      filepath.Join(docsDir, ".cache"),
		openAPIPath:   filepath.Join(rootDir, "backend", "internal", "apigen", "gen", "openapi.json"),
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
	if err := copyAPIAssets(cfg, string(themeCSS)); err != nil {
		return fmt.Errorf("staging API reference assets: %w", err)
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
		}
		// A URL value is a link node (opens in a new tab); anything else is a
		// path to a .doc.md the generator renders into a page.
		if strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
			node.Href = filePath
		} else {
			node.Path = filePath
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
		} else if n.Href != "" {
			b.WriteString(fmt.Sprintf("%s<li><a href=\"%s\" target=\"_blank\" rel=\"noopener\" class=\"external\">%s%s</a></li>\n", ind, n.Href, n.Label, externalIcon))
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

// copyAPIAssets stages the interactive API reference under /api/: the Scalar
// bundle (fetched+cached at build time, then served same-origin) and the
// generated OpenAPI spec. The spec is produced by `apigen generate` against a
// database, so it may be absent in a checkout that hasn't generated yet; the
// build proceeds without it (the reference renders once the spec exists) rather
// than failing.
func copyAPIAssets(cfg buildConfig, themeCSS string) error {
	apiDir := filepath.Join(cfg.outDir, "api")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		return err
	}
	cached := filepath.Join(cfg.cacheDir, "scalar-"+scalarVersion+".js")
	if err := ensureScalarBundle(cached); err != nil {
		return err
	}
	scalar, err := os.ReadFile(cached)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(apiDir, "scalar.standalone.js"), scalar, 0o644); err != nil {
		return err
	}
	// The standalone full-viewport reference page served at /api/ (the sidebar's
	// "Reference" link opens it in a new tab), themed with the site's tokens.
	page := fmt.Sprintf(apiReferencePageTmpl, themeCSS)
	if err := os.WriteFile(filepath.Join(apiDir, "index.html"), []byte(page), 0o644); err != nil {
		return err
	}
	spec, err := os.ReadFile(cfg.openAPIPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("note: openapi.json not found — run `apigen generate`; the API reference will be empty until it exists")
			return nil
		}
		return fmt.Errorf("reading openapi.json: %w", err)
	}
	return os.WriteFile(filepath.Join(apiDir, "openapi.json"), spec, 0o644)
}

// ensureScalarBundle downloads the pinned Scalar bundle into the build cache the
// first time and reuses it thereafter (so watch-mode rebuilds don't re-fetch).
func ensureScalarBundle(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // cached
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fmt.Printf("fetching Scalar bundle %s ...\n", scalarVersion)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(scalarBundleURL)
	if err != nil {
		return fmt.Errorf("downloading scalar bundle: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading scalar bundle: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading scalar bundle: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
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
