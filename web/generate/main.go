package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
)

type SidebarNode struct {
	Label    string `json:"label"`
	Slug     string `json:"slug"`
	Path     string `json:"path,omitempty"`
	HTMLFile string `json:"htmlFile,omitempty"`
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
	// Markdown is the URL of this page's markdown twin: the same URL with the
	// trailing slash replaced by ".md". Announced in the head as a
	// rel="alternate", which is how a client that would rather read markdown
	// than parse a page finds it.
	Markdown string
	BaseURL  string
	PrevPage *PageLink
	NextPage *PageLink
	// Crumbs is the trail of sidebar sections above this page, ending with the
	// page itself. Empty for a page that sits at the top level, where a trail
	// would only repeat the title below it.
	Crumbs []PageLink
	// SourceURL is this page's markdown on GitHub, so every page can offer an
	// edit link. Empty when the build has no repository to point at.
	SourceURL string
}

type PageLink struct {
	Label string
	Href  string
}

// marketingPage is a hand-written page from web/site, as the rest of the build
// needs to know it: the URL it ended up at, and the title and description it
// declares in its own <head>. Read back out of the page rather than configured
// here, so the sitemap and llms.txt cannot describe it differently from the
// page itself.
type marketingPage struct {
	URL         string
	Title       string
	Description string
}

// A search hit is the section it points at, not the page: Title is the heading
// itself and Path is what sits above it, so the result can show the leaf and
// its ancestry as two lines rather than one long chain.
type SearchEntry struct {
	Title string `json:"title"`
	Path  string `json:"path,omitempty"`
	Href  string `json:"href"`
	Body  string `json:"body"`
}

const siteURL = "https://select-db.com"

// sourceURL is where a page's markdown lives on GitHub, so every page can
// offer an edit link. Pinned to dev, the repo's default branch and the one the
// published docs are built from -- main lags it and does not have these paths.
const sourceURL = "https://github.com/select-db/select/blob/dev/"

type buildConfig struct {
	rootDir       string
	webDir        string
	sidebarPath   string
	templateDir   string
	cssPath       string
	basePath      string
	siteDir       string
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
/*THEME*/
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
      /* Match the docs body size so generated endpoint descriptions (and the
         filterable-fields tables inside them) read at the same scale as the
         doc pages. Scalar's own heading/sidebar/code scale is left untouched. */
      --scalar-paragraph: var(--fs-body);
    }
    /* --fs-body lives in docs.css, not .theme; redeclare it for this standalone page. */
    :root { --fs-body: 1.0625rem; }
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

// themeMarker is the literal the marketing pages carry where the app's theme
// file is spliced in at build time.
const themeMarker = "/*THEME*/"

// starsMarker is where a marketing page carries the GitHub star count. The
// count is read once per build and written in, because the alternative is the
// page calling api.github.com from the reader's browser: a third-party request
// on every visit, for a number that changes by the hour. Substituted with
// nothing when the count cannot be read, so the button degrades to its verb.
const starsMarker = "<!--STARS-->"

// starsRepo is the repository the header's star button points at and counts.
const starsRepo = "select-db/select"

// starsMaxAge is how long a cached count is served before the build asks
// again. Long enough that a watch-mode rebuild never calls out, short enough
// that a deploy ships a current number.
const starsMaxAge = 6 * time.Hour

func newBuildConfig(rootDir string) buildConfig {
	webDir := filepath.Join(rootDir, "web")
	return buildConfig{
		rootDir:       rootDir,
		webDir:        webDir,
		sidebarPath:   filepath.Join(webDir, "sidebar.txt"),
		templateDir:   filepath.Join(webDir, "template"),
		cssPath:       filepath.Join(webDir, "theme", "docs.css"),
		basePath:      filepath.Join(webDir, "theme", "base.css"),
		themePath:     filepath.Join(rootDir, "app", "internal", "graph", "defaults", "user", ".theme"),
		componentsDir: filepath.Join(webDir, "components"),
		cacheDir:      filepath.Join(webDir, ".cache"),
		openAPIPath:   filepath.Join(rootDir, "backend", "internal", "apigen", "gen", "openapi.json"),
		siteDir:       filepath.Join(webDir, "site"),
		outDir:        filepath.Join(webDir, "dist"),
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
	fmt.Printf("serving http://localhost%s (landing) and http://localhost%s/docs/\n", addr, addr)
	// Plain file server: `/` is the landing page, an ordinary index.html in the
	// output root. It used to redirect to the first doc, from when docs were the
	// whole site.
	if err := http.ListenAndServe(addr, http.FileServer(http.Dir(cfg.outDir))); err != nil {
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

	baseCSS, err := os.ReadFile(cfg.basePath)
	if err != nil {
		return fmt.Errorf("reading base CSS: %w", err)
	}

	docsCSS, err := os.ReadFile(cfg.cssPath)
	if err != nil {
		return fmt.Errorf("reading docs.css: %w", err)
	}

	// App theme (colour tokens) → shared base (scale, reset, typography) →
	// surface layout. Every surface on the site stacks in this order.
	combinedCSS := string(themeCSS) + "\n" + string(baseCSS) + "\n" + string(docsCSS)
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
		// WithAttribute lets a block carry a class from the markdown, which is how
		// the "where to go next" list becomes a grid of cards without any of it
		// being HTML in the source. Standard goldmark syntax: `{.cards}`.
		goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithAttribute()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
		// GitHub's alert syntax, which the docs already write by hand. See
		// callout.go.
		goldmark.WithParserOptions(parser.WithASTTransformers(callouts)),
		// A fence can name the file it belongs in. See codeblock.go.
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(codeBlocks)),
		// A screenshot carries both theme cuts. See image.go.
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(images)),
		// A keystroke in a code span renders as keycaps. See kbd.go.
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(keystrokes)),
	)

	// Everything below writes into a staging directory that is swapped into
	// place at the end, because -serve keeps answering requests out of the
	// served one while a rebuild runs. Emptying it first meant a reload timed
	// inside a rebuild got a page whose stylesheet, scripts or screenshots did
	// not exist yet -- and a build that failed halfway left nothing at all.
	// cfg is a value, so pointing outDir at the staging directory here redirects
	// every write in this function and nothing outside it.
	served := cfg.outDir
	// A fresh directory per build, not a fixed name: `go run .` while `-serve`
	// is watching is an ordinary thing to do, and two builds sharing one
	// staging path write over each other and publish whichever finishes last,
	// half of it missing.
	staging, err := os.MkdirTemp(filepath.Dir(served), filepath.Base(served)+".staging-")
	if err != nil {
		return fmt.Errorf("creating staging dir: %w", err)
	}
	// Harmless once the rename below has moved it; a build that fails anywhere
	// after this leaves nothing behind, and leaves the served site untouched.
	defer func() { _ = os.RemoveAll(staging) }()
	cfg.outDir = staging
	if err := os.WriteFile(filepath.Join(cfg.outDir, "style.css"), []byte(combinedCSS), 0o644); err != nil {
		return fmt.Errorf("writing style.css: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "bundle.js"), []byte(scriptsContent), 0o644); err != nil {
		return fmt.Errorf("writing bundle.js: %w", err)
	}
	themeJS, err := os.ReadFile(filepath.Join(cfg.webDir, "theme.js"))
	if err != nil {
		return fmt.Errorf("reading theme.js: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "theme.js"), themeJS, 0o644); err != nil {
		return fmt.Errorf("writing theme.js: %w", err)
	}
	favicon, err := os.ReadFile(filepath.Join(cfg.webDir, "favicon.png"))
	if err != nil {
		return fmt.Errorf("reading favicon: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.outDir, "favicon.png"), favicon, 0o644); err != nil {
		return fmt.Errorf("writing favicon: %w", err)
	}
	logo, err := os.ReadFile(filepath.Join(cfg.webDir, "logo.png"))
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
			Markdown:    "/" + page.HTMLFile + ".md",
			BaseURL:     siteURL,
			PrevPage:    prev,
			NextPage:    next,
			Crumbs:      breadcrumbs(tree, page.HTMLFile),
			SourceURL:   sourceURL + page.Path,
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

		// The markdown twin: the source, verbatim, at the page's URL with ".md"
		// in place of the trailing slash. It is what the page was rendered from,
		// so there is nothing to keep in step — and its links are already
		// absolute site paths, so they work as well in markdown as in HTML.
		if err := os.WriteFile(filepath.Join(cfg.outDir, page.HTMLFile+".md"), src, 0o644); err != nil {
			return fmt.Errorf("writing %s.md: %w", page.HTMLFile, err)
		}

		for _, entry := range buildSearchEntries(string(src), page.Label, "/"+page.HTMLFile+"/") {
			searchIndex = append(searchIndex, entry)
		}
	}

	marketing, err := copySitePages(cfg, string(themeCSS), starCount(cfg.cacheDir))
	if err != nil {
		return fmt.Errorf("staging marketing pages: %w", err)
	}

	if err := collectShots(cfg); err != nil {
		return fmt.Errorf("staging screenshots: %w", err)
	}

	home := false
	for _, m := range marketing {
		if m.URL == "/" {
			home = true
		}
	}

	// Without a homepage the root still redirects into the docs. Once
	// site/index.html exists it is the root, and the redirect is dropped.
	if !home && len(pages) > 0 {
		redirect := fmt.Sprintf("/ /%s/ 301\n", pages[0].HTMLFile)
		os.WriteFile(filepath.Join(cfg.outDir, "_redirects"), []byte(redirect), 0o644)
	}

	searchJSON, _ := json.Marshal(searchIndex)
	os.WriteFile(filepath.Join(cfg.outDir, "search-index.json"), searchJSON, 0o644)
	writeSitemap(cfg.outDir, pages, marketing)
	os.WriteFile(filepath.Join(cfg.outDir, "robots.txt"), []byte("User-agent: *\nAllow: /\nSitemap: "+siteURL+"/sitemap.xml\n"), 0o644)
	writeLLMsTxt(cfg, pages, marketing)
	writeLLMsFullTxt(cfg, pages, marketing)

	headers := `/*
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  Referrer-Policy: strict-origin-when-cross-origin
  Strict-Transport-Security: max-age=31536000; includeSubDomains

# The markdown twins and llms.txt are meant to be read in the browser, not
# downloaded. Without an explicit type a static host serves .md as a download,
# and nosniff above means the browser will not second-guess it.
/*.md
  Content-Type: text/markdown; charset=utf-8
/llms.txt
  Content-Type: text/plain; charset=utf-8
/llms-full.txt
  Content-Type: text/plain; charset=utf-8
`
	os.WriteFile(filepath.Join(cfg.outDir, "_headers"), []byte(headers), 0o644)

	if err := verifyLinks(cfg.outDir); err != nil {
		return err
	}

	// Two renames rather than a copy: the served directory is replaced whole,
	// so a request either gets the previous build or this one.
	previous := staging + ".previous"
	if _, err := os.Stat(served); err == nil {
		if err := os.Rename(served, previous); err != nil {
			return fmt.Errorf("setting the previous build aside: %w", err)
		}
	}
	if err := os.Rename(staging, served); err != nil {
		return fmt.Errorf("publishing the build: %w", err)
	}
	if err := os.RemoveAll(previous); err != nil {
		return fmt.Errorf("removing the previous build: %w", err)
	}

	fmt.Printf("built %d pages in %s\n", len(pages), served)

	return nil
}

// watch polls for file changes and rebuilds when something changes
func watch(cfg buildConfig) {
	watchDirs := []string{
		cfg.webDir,
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
				// dist is what this build writes and .cache is what it downloads:
				// watching either makes every build trigger the next one. generate
				// is the builder's own source, which needs a restart, not a
				// rebuild. site is watched — editing a marketing page is the
				// commonest reason to be running this at all.
				if info.IsDir() && (info.Name() == "dist" || info.Name() == ".cache" || info.Name() == "generate") {
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
				if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == "web") {
					return filepath.SkipDir
				}
				// .doc.md and the screenshots beside it: both are inputs the
				// build reads from outside web/, and a recaptured figure is the
				// commonest reason to want a rebuild after running the shots.
				isInput := strings.HasSuffix(path, ".doc.md") ||
					(strings.HasSuffix(path, ".png") && filepath.Base(filepath.Dir(path)) == "shots")
				if isInput && info.ModTime().After(lastMod) {
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

	// Documentation is mounted under /docs/; the root is the marketing site.
	// Cross-references inside .doc.md files are written with the same prefix,
	// so moving the docs means updating both this line and those links.
	computeHTMLPaths(root, "docs")
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

// metaContent pulls one value out of a page's own <head> — the text between an
// opening and closing literal. Deliberately not a parser: two tags, on pages
// this repository writes, where a regex over the whole document would be the
// bigger surprise.
func metaContent(page []byte, open, close string) string {
	i := bytes.Index(page, []byte(open))
	if i < 0 {
		return ""
	}
	rest := page[i+len(open):]
	j := bytes.Index(rest, []byte(close))
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(string(rest[:j]))
}

func writeSitemap(outDir string, pages []*SidebarNode, marketing []marketingPage) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	// Marketing first: the homepage is the entry point, and a sitemap that
	// starts at the first doc page reads like the site has no front door.
	for _, m := range marketing {
		b.WriteString(fmt.Sprintf("  <url><loc>%s%s</loc></url>\n", siteURL, m.URL))
	}
	for _, p := range pages {
		b.WriteString(fmt.Sprintf("  <url><loc>%s/%s/</loc></url>\n", siteURL, p.HTMLFile))
	}
	b.WriteString("</urlset>\n")
	os.WriteFile(filepath.Join(outDir, "sitemap.xml"), []byte(b.String()), 0o644)
}

// tagline is the one-line summary at the top of llms.txt and llms-full.txt.
// Taken from the homepage's own meta description so there is one sentence
// describing this product, not two that drift.
func tagline(marketing []marketingPage) string {
	for _, m := range marketing {
		if m.URL == "/" && m.Description != "" {
			return m.Description
		}
	}
	return "An SQL client shaped like an IDE."
}

// writeLLMsTxt writes the /llms.txt index: what this site is, and every page on
// it. Doc entries link to the markdown twin rather than the page — a client
// following this file wants the prose, not the chrome around it.
func writeLLMsTxt(cfg buildConfig, pages []*SidebarNode, marketing []marketingPage) {
	var b strings.Builder
	b.WriteString("# SELECT\n\n")
	b.WriteString("> " + tagline(marketing) + "\n\n")

	if len(marketing) > 0 {
		b.WriteString("## Product\n\n")
		for _, m := range marketing {
			b.WriteString(fmt.Sprintf("- [%s](%s): %s\n", m.Title, m.URL, m.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Docs\n\n")
	for _, p := range pages {
		src, _ := os.ReadFile(filepath.Join(cfg.rootDir, p.Path))
		desc := extractDescription(string(src))
		b.WriteString(fmt.Sprintf("- [%s](/%s.md): %s\n", p.Label, p.HTMLFile, desc))
	}

	b.WriteString("\n## Optional\n\n")
	b.WriteString("- [Full documentation, one file](/llms-full.txt): every page above, concatenated.\n")
	b.WriteString("- [API reference](/api/): the HTTP API, rendered from /openapi.json.\n")

	os.WriteFile(filepath.Join(cfg.outDir, "llms.txt"), []byte(b.String()), 0o644)
}

func writeLLMsFullTxt(cfg buildConfig, pages []*SidebarNode, marketing []marketingPage) {
	var b strings.Builder
	b.WriteString("# SELECT\n\n")
	b.WriteString("> " + tagline(marketing) + "\n\n")
	for _, p := range pages {
		src, _ := os.ReadFile(filepath.Join(cfg.rootDir, p.Path))
		// The URL as well as the label: a model quoting this file can then cite
		// the page it came from rather than the concatenation.
		b.WriteString(fmt.Sprintf("---\n\n## %s\n\nSource: %s/%s/\n\n", p.Label, siteURL, p.HTMLFile))
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
			anchor := stack[len(stack)-1].slug
			entries = append(entries, SearchEntry{
				Title: parts[len(parts)-1],
				Path:  strings.Join(parts[:len(parts)-1], " › "),
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
			text := headingAttrs.ReplaceAllString(strings.TrimSpace(strings.TrimLeft(trimmed, "#")), "")
			text = strings.TrimSpace(text)
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

// goldmark's attribute syntax, which a heading carries to style what follows
// it (`## Where to go next {.cards}`). The renderer consumes it; this file
// reads the markdown itself, and without this the braces reached the search
// index as part of the heading.
var headingAttrs = regexp.MustCompile(`\s*\{[.#][^}]*\}\s*$`)

var calloutLabelRe = regexp.MustCompile(`\[!(?i:NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]`)

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
	//
	// Substituted, not Sprintf'd: the template is CSS, CSS is full of percent
	// signs, and `height: 100%;` reaching a format string came out as
	// `height: 100%!;(NOVERB)` on the shipped page. The same marker the
	// marketing pages use, which cannot misread its own content.
	page := strings.Replace(apiReferencePageTmpl, themeMarker, themeCSS, 1)
	if err := os.WriteFile(filepath.Join(apiDir, "index.html"), []byte(page), 0o644); err != nil {
		return err
	}
	spec, err := os.ReadFile(cfg.openAPIPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("note: openapi.json not found - run `apigen generate`; the API reference will be empty until it exists")
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

// starCount returns the GitHub star count as the markup the header expects, or
// "" when it cannot be read — offline, rate-limited, or the repository still
// private. A build never fails over it: a missing number is a smaller problem
// than a site that cannot be built on a plane.
func starCount(cacheDir string) string {
	cache := filepath.Join(cacheDir, "stars.json")

	if info, err := os.Stat(cache); err == nil && time.Since(info.ModTime()) < starsMaxAge {
		if n, err := readStars(cache); err == nil {
			return formatStars(n)
		}
	}

	n, err := fetchStars()
	if err != nil {
		fmt.Printf("star count unavailable (%v); ", err)
		// Stale beats absent: a number from this morning is still true enough
		// for a button, and this is the offline path.
		if n, err := readStars(cache); err == nil {
			fmt.Printf("using the cached one\n")
			return formatStars(n)
		}
		fmt.Printf("rendering the button without one\n")
		return ""
	}

	if err := os.MkdirAll(cacheDir, 0o755); err == nil {
		os.WriteFile(cache, []byte(fmt.Sprintf(`{"stargazers_count":%d}`, n)), 0o644)
	}
	return formatStars(n)
}

func fetchStars() (int, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + starsRepo)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var body struct {
		Count int `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Count, nil
}

func readStars(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var body struct {
		Count int `json:"stargazers_count"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return 0, err
	}
	return body.Count, nil
}

// formatStars renders the count the way GitHub's own button does: exact below a
// thousand, one decimal above it.
func formatStars(n int) string {
	switch {
	case n <= 0:
		return ""
	case n < 1000:
		return fmt.Sprintf("<b>%d</b>", n)
	default:
		return fmt.Sprintf("<b>%.1fk</b>", float64(n)/1000)
	}
}

// collectShots stages every product screenshot into dist/shots.
//
// A capture writes its images into a `shots/` directory beside itself, and a
// spec lives beside the code it photographs -- so they are scattered across the
// repo the way .doc.md files are, and this walks for them rather than reading
// one directory. The site serves them from a single flat /shots/, which is what
// a doc page writes in its markdown, so two files of the same name anywhere in
// the tree are a collision: the build says which two rather than letting the
// second quietly win.
func collectShots(cfg buildConfig) error {
	skip := map[string]bool{
		"node_modules": true, "build": true,
		".svelte-kit": true, ".git": true, "test-results": true,
	}
	// The build's own output, in every state it passes through: dist is the
	// served copy, dist.staging is the one being written, dist.previous is the
	// one on its way out. Walking into them finds this build's screenshots and
	// reports them as duplicates of the sources they were copied from.
	isOutput := func(path, name string) bool {
		return strings.HasPrefix(name, "dist") && filepath.Dir(path) == cfg.webDir
	}
	dst := filepath.Join(cfg.outDir, "shots")
	from := map[string]string{}

	err := filepath.WalkDir(cfg.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if skip[d.Name()] || isOutput(path, d.Name()) {
			return fs.SkipDir
		}
		if d.Name() != "shots" {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".png") {
				continue
			}
			src := filepath.Join(path, name)
			if prev, taken := from[name]; taken {
				rel := func(p string) string { r, _ := filepath.Rel(cfg.rootDir, p); return r }
				return fmt.Errorf("two screenshots named %s: %s and %s (the site serves one flat /shots/)", name, rel(prev), rel(src))
			}
			from[name] = src

			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
				return err
			}
		}
		return fs.SkipDir
	})
	if err != nil {
		return err
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

// A table's delimiter row. Spaces around the pipes are the usual way these are
// written and the reason the row used to survive into search excerpts as a run
// of dashes between two column headings.
var tableSepRe = regexp.MustCompile(`\|?[\s]*[-:]+[\s]*\|[-|:\s]+\|?`)

func stripMarkdown(s string) string {
	s = linkRe.ReplaceAllString(s, "$1")
	s = tableSepRe.ReplaceAllString(s, "")
	// The label goldmark reads off a callout's first line. It is chrome the
	// renderer turns into a heading, and in a search excerpt it is noise in
	// front of the sentence somebody was actually looking for.
	s = calloutLabelRe.ReplaceAllString(s, "")
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

// copySitePages copies the hand-written marketing pages from web/site into the
// build, substituting the app's theme file for the /*THEME*/ marker in each.
//
// The marketing pages share the app's colour tokens with the docs and nothing
// else: no shared template, no shared layout, no generated markup. They are
// plain HTML with their CSS inline, so a page arrives in one request and, at
// current size, inside the first TCP congestion window. Keeping the theme a
// build-time substitution is what stops the site's palette drifting from the
// product's.
//
// Reports the pages it staged, so the sitemap and llms.txt can list the
// marketing side of the site alongside the docs.
func copySitePages(cfg buildConfig, themeCSS, stars string) ([]marketingPage, error) {
	entries, err := os.ReadDir(cfg.siteDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pages []marketingPage
	var staged []string
	for _, e := range entries {
		name := e.Name()

		if e.IsDir() || !strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".draft.html") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(cfg.siteDir, name))
		if err != nil {
			return nil, err
		}
		if !bytes.Contains(src, []byte(themeMarker)) {
			return nil, fmt.Errorf("%s: missing %s marker (the app theme has nowhere to go)", name, themeMarker)
		}
		out := bytes.Replace(src, []byte(themeMarker), []byte(themeCSS), 1)
		out = bytes.ReplaceAll(out, []byte(starsMarker), []byte(stars))

		// index.html is the site root; any other page gets a directory so its
		// URL has no extension.
		dst := filepath.Join(cfg.outDir, name)
		url := "/"
		if name != "index.html" {
			slug := strings.TrimSuffix(name, ".html")
			dir := filepath.Join(cfg.outDir, slug)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
			dst = filepath.Join(dir, "index.html")
			url = "/" + slug + "/"
		}
		if err := os.WriteFile(dst, out, 0o644); err != nil {
			return nil, err
		}
		staged = append(staged, dst)
		pages = append(pages, marketingPage{
			URL:         url,
			Title:       metaContent(out, "<title>", "</title>"),
			Description: metaContent(out, `<meta name="description" content="`, `">`),
		})
	}

	if err := checkSiteBudget(cfg, staged); err != nil {
		return nil, err
	}

	// Directory order is alphabetical, which puts /download/ ahead of the
	// homepage. Lead with the front door in every listing that follows.
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].URL == "/" })

	return pages, nil
}

// pageBudget is the transfer size a marketing page must fit into, brotli'd.
//
// 14 KB is the initial congestion window: ten TCP segments, what a server may
// send before waiting for the client's first ACK. Under it the document arrives
// in one round trip; over it costs a second, which on mobile is 50-150ms of a
// blank screen. It is a wall, not a score — there is nothing to win by shaving
// a page from 6 KB to 3 KB, and plenty to lose if the copy suffers for it.
const pageBudget = 14 * 1024

// A page may link anywhere it likes — <a href> and <link rel=canonical> cost
// nothing. What matters is what the browser must *fetch* to render: media and
// script sources, stylesheets, CSS imports and url() references.
var (
	subresourceRe = regexp.MustCompile(`(?i)(?:\bsrc=|@import\s+|\burl\()\s*["']?((?:https?:)?//[^"')\s>]+)`)
	linkTagRe     = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	linkHrefRe    = regexp.MustCompile(`(?i)href=["']((?:https?:)?//[^"'\s>]+)`)
	// rel values that make the browser open a connection. canonical, alternate,
	// author and friends are metadata and never fetched.
	fetchingRelRe = regexp.MustCompile(`(?i)rel=["']?(stylesheet|preload|prefetch|preconnect|dns-prefetch|modulepreload)`)
)

// externalFetches lists every third-party request a page would make.
func externalFetches(html string) []string {
	var out []string
	for _, m := range subresourceRe.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	for _, tag := range linkTagRe.FindAllString(html, -1) {
		href := linkHrefRe.FindStringSubmatch(tag)
		if href != nil && fetchingRelRe.MatchString(tag) {
			out = append(out, href[1])
		}
	}
	return out
}

// checkSiteBudget holds the two rules that keep a marketing page fast, both of
// which regress silently: it must arrive in one round trip, and it must not
// depend on anyone else's server. Web fonts break both at once.
func checkSiteBudget(cfg buildConfig, paths []string) error {
	var problems []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, _ := filepath.Rel(cfg.outDir, path)

		var buf bytes.Buffer
		w := brotli.NewWriterLevel(&buf, brotli.BestCompression)
		if _, err := w.Write(data); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		size := buf.Len()
		if size > pageBudget {
			problems = append(problems, fmt.Sprintf(
				"  %s is %.1f KB brotli, over the %d KB budget by %d bytes",
				name, float64(size)/1024, pageBudget/1024, size-pageBudget))
			continue
		}
		fmt.Printf("  %-24s %5.1f KB brotli  %2d packets  (%d%% of budget)\n",
			name, float64(size)/1024, (size+1459)/1460, size*100/pageBudget)

		for _, url := range externalFetches(string(data)) {
			problems = append(problems, fmt.Sprintf(
				"  %s fetches %s from another origin: inline it or self-host it", name, url))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("marketing page budget:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

// breadcrumbs is the trail of sidebar sections above a page, ending with the
// page itself. A page at the top level gets none: "Docs / Getting Started"
// above an h1 reading "Getting Started" is furniture, not orientation.
//
// The last crumb is the page itself and is not a link; nor is a bare section
// heading like "Special Files", which has no page of its own to link to.
func breadcrumbs(tree []*SidebarNode, activePath string) []PageLink {
	var trail []PageLink

	var walk func(nodes []*SidebarNode, above []PageLink) bool
	walk = func(nodes []*SidebarNode, above []PageLink) bool {
		for _, n := range nodes {
			crumb := PageLink{Label: n.Label}
			if n.Path != "" {
				crumb.Href = "/" + n.HTMLFile + "/"
			}
			here := append(append([]PageLink{}, above...), crumb)

			if n.HTMLFile == activePath && n.Path != "" {
				if len(here) > 1 {
					// The last crumb is where the reader already is.
					here[len(here)-1].Href = ""
					trail = here
				}
				return true
			}
			if walk(n.Children, here) {
				return true
			}
		}
		return false
	}
	walk(tree, nil)

	return trail
}
