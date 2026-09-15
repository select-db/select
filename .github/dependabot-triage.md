# Dependabot PR triage

Last refreshed 2026-08-22 against `dev` @ `a9117ed`.

Working notes from a sweep of the open Dependabot queue: what each PR carries,
what blocks it, and the ordering constraints this monorepo imposes.

## Open blocker: `toolkit/go.mod` on go 1.25.12

`ff73a9a` bumped `app`, `backend` and `dialect` to `go 1.25.13` for the
stdlib advisories but missed `toolkit`. `toolkit` is in the security matrix and
every push to `dev` sets its plan flag, so `Security (toolkit)` fails
govulncheck on GO-2026-6090, GO-2026-6089 and GO-2026-5972 — the only failing
job, and enough to fail `CI OK` and skip the desktop builds and staging
release. Fixed in this branch.

This also blocks **#124** and **#144**, which touch `.github/` and therefore
fan out to the full matrix. PR #144's run is the proof: every job green except
`Security (toolkit)`.

## Ordering constraints worth remembering

**Cross-module `replace` coupling.** `app` and `backend` both `replace`
dialect and toolkit with local paths, so a bump inside `dialect` must land with
its consumers — merging the shared library first leaves them unbuildable
(`go: updates to go.mod needed`).

| Merged alone | app | backend |
|---|---|---|
| #125 (sqlite, dialect) | broken | OK |
| #128 (compress, dialect) | broken | broken |
| #135 (x/crypto, dialect) | broken | broken |

- **#125** → after #126 (merged).
- **#128** → after #127 and #129 (both merged).
- **#135** → after #138 (merged), *plus* a hand-made bump of `app`'s indirect
  `golang.org/x/crypto` to v0.55.0 (`cd app && go mod tidy`). Dependabot opened
  no app PR because x/crypto is indirect there, so #135 alone still leaves
  `app` unbuildable.

**Same-module PRs collide.** Within a module, the second and later PRs conflict
on `go.mod` once the first lands. Merge one at a time and let Dependabot rebase
between — this is what left #134, #136 and #141 needing a rebase.

**Both npm PRs cannot be merged back-to-back.** Git merges the two lockfiles
cleanly line-wise but semantically inconsistently, and CI then hard-fails:

```
npm error `npm ci` can only install packages when your package.json and
npm error package-lock.json are in sync.
npm error Invalid: lock file's @typescript-eslint/types@8.64.0
npm error does not satisfy @typescript-eslint/types@8.67.0
```

The end state is fine once the lockfile is regenerated. Merge one, let
Dependabot rebase the other, then merge it.

## Remaining queue

| PR | Bump | Status |
|----|------|--------|
| #134 | testify 1.11.1 → 1.12.1 (app) | rebase requested — conflicts after #126/#129 |
| #136 | testify 1.11.1 → 1.12.1 (backend) | rebase requested — conflicts after #127/#130/#138 |
| #141 | typescript-eslint 8.60.1 → 8.67.0 | rebase requested — conflicts after #140 |
| #125 | sqlite 1.54.0 → 1.56.0 (dialect) | needs #126 (merged) — ready once rebased |
| #128 | compress 1.19.1 → 1.19.2 (dialect) | needs #127/#129 (merged) — ready once rebased |
| #135 | x/crypto 0.54.0 → 0.55.0 (dialect) | needs #138 (merged) **and** `go mod tidy` in app |
| #137 | wails 2.10.1 → 2.15.0 (app) | on hold — superseded by in-flight wails 3 work |
| #144 | astral-sh/setup-uv 9.0.0 → 10.0.1 | blocked on this branch's toolkit fix |
| #124 | github/codeql-action v4 → v4.37.3 | rework — see below |

**#144** is correctly SHA-pinned (`20cfd1bf…` matches the v10.0.1 tag). It is a
major bump, but v10's only breaking change is that `enable-cache: auto` now
disables the cache for `pull_request_target`, `workflow_run` and `release` —
none of which this workflow uses.

**#124** swaps a floating major tag for a floating minor tag, contradicting the
SHA-pinning convention its own inline comment states, and is now several
patches stale (v4.37.8 is current). Prefer a SHA pin:

```yaml
uses: github/codeql-action/init@37f2634a92ba38a0926ef79a0748ac8ae7d95ab2 # v4.37.8
uses: github/codeql-action/analyze@37f2634a92ba38a0926ef79a0748ac8ae7d95ab2 # v4.37.8
```

**#137** also needs `WAILS_VERSION` in `.github/workflows/ci.yml` bumped in the
same PR — PR CI cannot catch that skew, because the desktop jobs only run on
`workflow_dispatch` or a push to `dev`/a tag.

## What the dialect bumps are worth

The two still-pending dialect bumps carry the most user-facing value:

- **#125 (sqlite)** — 1.56.0 patches a data-corruption bug in SQLite 3.53.3's
  journal rollback affecting multi-database transactions. Reachable here:
  `dialect/sqlite/foreign_key_lookup.go` treats attached databases as schemas,
  so a user with `ATTACH`ed databases writing across them hits exactly that
  scenario. (In `app`, which never issues `ATTACH`, the same bump is hygiene.)
- **#135 (x/crypto)** — 0.55.0 is largely SSH work, and `dialect/engine/ssh.go`
  is the bastion tunnel (client-side `ssh.Dial`). The relevant fixes: window
  credit returned for discarded extended data, stderr drained on forwarded
  TCP/Unix channels, a channel-initialisation race, idempotent channel close,
  no longer dropping the connection on an unparseable public key, and the RSA
  modulus limit raised to 16384 bits.

## Merged

**2026-08-22** — #126 (sqlite 1.56.0, app), #129 (compress 1.19.2, app),
#127 (compress 1.19.2, backend), #130 (okms-sdk-go 0.5.4, backend — drops
RSA1_5 from the key-wrapping algorithms), #138 (x/crypto 0.55.0, backend),
#140 (eslint-plugin-svelte 3.23.0).

**2026-08-20** — #118 (dompurify 3.4.12), #122 (@sveltejs/adapter-static
3.0.10), #123 (eslint-config-prettier 10.1.8).

## Closed

**2026-08-22** — #139, #142, #143 (@tanstack/ai-client / -openai / -grok).
`b7d9e32` removed the whole family; these were opened against the pre-removal
`dev`, no longer merged, and would have resurrected the dependency.
