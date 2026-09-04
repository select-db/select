# web/ — select-db.com

Everything served from `select-db.com`, built by one generator into one static
site.

| Path | What it is |
|------|------------|
| `generate/` | The site generator (Go + goldmark). `./dev.sh web build` builds; `./dev.sh web start` builds, serves on :3333 and watches. |
| `sidebar.txt` | Docs manifest: the nav tree, mapping labels to `.doc.md` files anywhere in the repo. |
| `content/` | Doc pages that belong to no code module (getting started, …). Every other `.doc.md` lives next to the code it documents. |
| `template/` | Page shell: `base.html`, `head.html`, `sidebar.html`, `page-nav.html`. |
| `components/` | Browser components loaded on doc pages (search, TOC, sidebar, code blocks). |
| `theme/base.css` | Shared tokens, reset and base typography. Used by every surface. |
| `theme/docs.css` | Docs-site layout only. |
| `site/` | Marketing pages: hand-written HTML, one file per page, CSS inline. |
| `content/` and everywhere else | `*.shot.ts` and `shots/` sit beside the code they photograph. See [Screenshots](#screenshots). |
| `dist/` | Build output. Git-ignored. |

## Machine-readable surfaces

Everything on the site exists in a form something other than a browser can
read, because half the traffic to a developer tool's docs now arrives through
one.

| URL | What it is |
|-----|------------|
| `/<any doc page>.md` | The page's markdown twin — the `.doc.md` it was rendered from, verbatim. The rule is the page URL with the trailing slash replaced by `.md`: `/docs/sql/variables/` → `/docs/sql/variables.md`. Announced in each page's head as `<link rel="alternate" type="text/markdown">`. |
| `/llms.txt` | The site index in the [llms.txt](https://llmstxt.org) format: the tagline, the marketing pages, every doc page with a one-line description, and pointers to the rest. Doc entries link to the twins, not the pages. |
| `/llms-full.txt` | Every doc page concatenated, each under its source URL. |
| `/sitemap.xml` | Every page, homepage first. |
| JSON-LD | `TechArticle` on doc pages, from the template; `SoftwareApplication` with the shipping editions' prices on the homepage, written inline in `site/index.html` beside the prose it mirrors. |

There is nothing to keep in step: a twin **is** its source file, and the
llms.txt tagline is read out of the homepage's own meta description. The one
hand-maintained duplicate is the homepage's JSON-LD, which lives in the same
file as the pricing table so a change to one shows up next to the other in the
diff.

`_headers` gives `.md` an explicit `text/markdown` content type. Without it a
static host serves markdown as a download, and the `X-Content-Type-Options:
nosniff` on every response stops the browser from guessing.

## URL namespaces

| Prefix | Source |
|--------|--------|
| `/` | Marketing (`site/`) |
| `/docs/*` | Generated documentation |
| `/api/` | API reference (Scalar + the generated OpenAPI spec) |

**Write cross-references with the full path.** In a `.doc.md`, link to
`/docs/workspace/roles/` — the URL the page actually has. Nothing rewrites links
at build time: what you write is what ships, and the build fails if the target
does not exist.

Moving the docs to a different prefix therefore means two changes: the
`computeHTMLPaths(root, "docs")` call in `generate/main.go`, and the links in
the `.doc.md` files. That is deliberate — a find-and-replace you can see beats a
transform you have to know about.

## Marketing pages

Hand-written, deliberately. They share **CSS variables** with the docs and
nothing else — no shared template, no shared layout, no generated markup. A
landing page and a doc page want different structure, and pretending otherwise
costs more than it saves.

Each page is a single self-contained file: CSS inline in `<head>`, system fonts,
no web fonts, no framework, no third-party request. The build copies
`site/*.html` into `dist/` and substitutes into two markers. `index.html`
becomes the site root; `foo.html` becomes `/foo/`. `*.draft.html` files are
skipped, so a work-in-progress page can sit beside a live one.

| Marker | Replaced with |
|--------|---------------|
| `/*THEME*/` | The app's `.theme` file, verbatim. Required — a page without it fails the build. |
| `<!--STARS-->` | The GitHub star count, read once per build and cached for six hours. Empty when it cannot be read, so the button degrades to its label; a build never fails over it. |

The count is baked in rather than fetched from the browser for the reason in the
budget below: the page makes no third-party request, and a star count is not
worth a DNS lookup, a TCP connection and a TLS handshake on every visit.

**The theme file is not the whole token set.** The app defines `--border` and
`--shadow-subtle` in its own stylesheet, not in `.theme`, so a marketing page
has to restate them — as `theme/base.css` already does for the docs. An
undefined custom property makes every rule that uses it invalid, and CSS drops
invalid rules silently: the symptom is a page with no borders and no error.

### Performance budget

Enforced by the build, on marketing pages only — the docs have a different
profile (shared stylesheet, components) and are not held to this.

| Rule | Limit | Why |
|------|-------|-----|
| Document, brotli | **14 KB** | The initial congestion window: ten TCP segments, what a server may send before waiting for the first ACK. Under it the page arrives in one round trip. Over it costs a second RTT — 50–150 ms of blank screen on mobile. |
| Third-party fetches | **0** | Every external origin is a DNS lookup, a TCP connection and a TLS handshake before a byte of content. A web font costs all three, twice. |

The build prints each page's size and fails on either rule:

```
  download/index.html        2.2 KB brotli   2 packets  (15% of budget)
  index.html                 5.9 KB brotli   5 packets  (42% of budget)
```

**14 KB is a wall, not a score.** There is nothing to win by shaving a page from
6 KB to 3 KB — both land in the same flight — and plenty to lose if the copy
suffers for it. Headroom exists so pages can be written properly.

Links are not fetches: `<a href>` and `<link rel=canonical>` may point anywhere.
The rule is about what the browser must download to render.

### Images

The hero is the largest thing on the page and the LCP element, so it gets
`fetchpriority="high"` and is **not** lazy-loaded: it is above the fold, and
deferring it would delay the one paint that decides how fast the page feels.
Anything below the fold gets `loading="lazy"`.

| Rule | Limit | Now |
|------|-------|-----|
| LCP image | ≤ 300 KB | 268 KB |
| Total first view | ≤ 500 KB | ~275 KB |
| Requests, first view | ≤ 8 | 2 |
| CLS | 0 | every source carries `width`/`height` |

The limit is 300 KB rather than 150 because the page serves one image from a
CDN, cached and compressed at the edge, and the alternative was capturing at a
density that visibly softens a screenshot of text. Two framings and two themes
mean four files, but a viewer only ever fetches one.

Still worth doing: derive WebP or AVIF at build time from the committed PNGs.
The PNG stays the baseline that makes pixel-diffing work in review, and the
same picture ships at roughly a third of the bytes. It needs an encoder on the
build machine, which is why it is not wired up yet.

## Screenshots

Every product picture on the site is captured from the running application, not
mocked. One spec per picture, **beside the code it photographs** and **named after the
`.doc.md` page it illustrates**, so a feature, its documentation and its picture
sit in one directory, share one name and change in one diff:

```
app/frontend/src/lib/components/views/Chat/
  ai-chat.doc.md          the page
  ai-chat.shot.ts         the shot
  shots/chat.light.png    what it wrote
```

The spec writes into a `shots/` directory beside itself (`shotsDirFor` reads
the spec's own path), and the build collects every one of them into a single
flat `/shots/`. Two images of the same name anywhere in the repo are therefore a
collision, and the build says which two rather than letting one win. A doc page
writes the light path in its markdown and the generator emits a `<picture>`
carrying both cuts.

The specs under `site/` have no `.doc.md` to be named after: their pictures only
ever appear on the landing page, which is the code they photograph as far as
this repo is concerned. `Settings/team.shot.ts` is the other exception -- it
feeds the users, roles and groups pages, so no single one of them names it.

**The root `package.json` exists for this.** Playwright decides ESM or CJS per
file from the nearest `package.json`, and the harness the specs share lives
under `app/frontend/`, which is ESM. Without a root declaring the same, a spec
in `app/internal/` loads as CJS and fails on the first `import`. The file holds
nothing else; there is no npm project at the root.

Regenerate them with:

```
./dev.sh web shots
```

That runs `wails3 task shots`, not `npm run shots`. Playwright launches
`build/bin/select-server` without ever building it — so a stale binary does not
fail the run, it republishes pictures of the previous build. The task depends on
`build:server`, which is the whole reason it exists. `npm run shots` is only for
a re-run when the build is already current.

They never run in CI. The suite is gated behind `SHOTS=1` and its own Playwright
project, so an ordinary test run cannot write to the repo. Captures are a
deliberate act: run them, look at the diff, commit the images.

All captures drive one seeded workspace (`app/internal/cmd/e2eseed`), so a spec
that changes state changes what every later spec photographs. Leave the
workspace as you found it, and prefer a state the seed already provides over one
you create.

## Colours come from the app

`theme/base.css` defines the scale (spacing, type, radii, borders). The colour
tokens it uses — `--gray-0` … `--gray-1000`, `--blue` — are read at build time
from `app/internal/graph/defaults/user/.theme`, the app's own default theme. The
site and the product cannot drift apart.

## Checks

The build fails on a broken internal link and on a `sidebar.txt` entry pointing
at a file that does not exist. Both run on every build, including `-serve`.
