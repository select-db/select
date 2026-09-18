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

## Name collision

`code-review/` shadows the `code-review` skill that ships with Claude Code.
Project skills win, so `/code-review` in this repository runs the vendored one.
