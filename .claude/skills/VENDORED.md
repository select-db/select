# Vendored skills

`codebase-design/` and `code-review/` are copied from
https://github.com/mattpocock/skills, path `skills/engineering/`, at commit
c55ee46073ed923f86ce59a5eb3b6d895095d1b7.

They are copied rather than installed as a marketplace plugin because remote
Claude Code containers clone this repository and nothing else, so a checked-in
copy is the only version a container sees.

Two local changes, both from this repository's own rules:

- The upstream `agents/openai.yaml` files are not copied; they are for another
  runner.
- One en dash in `codebase-design/DESIGN-IT-TWICE.md` became a hyphen, per the
  typography rule in the root `CLAUDE.md`.

To refresh, copy the upstream files again, reapply both changes, and update the
commit above.

## Unmet upstream dependencies

The copies are verbatim, so they still reference files that live in upstream
skills this repository does not vendor:

- `code-review/SKILL.md` reads `docs/agents/issue-tracker.md` to resolve issue
  references, and tells the reader to run `/setup-matt-pocock-skills` when it is
  absent. Neither exists here, so the Spec axis has no issue tracker to query and
  falls back to whatever spec it is handed.
- `codebase-design/DESIGN-IT-TWICE.md` cites a `CONTEXT.md` for domain
  vocabulary, which comes from the upstream `domain-modeling` skill.

Vendoring `setup-matt-pocock-skills` would close the first one.

## Name collision

`code-review/` shadows the `code-review` skill that ships with Claude Code.
Project skills win, so `/code-review` in this repository runs the vendored one.
