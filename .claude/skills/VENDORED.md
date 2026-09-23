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

## simplify

`simplify/` is the skill that ships inside Claude Code 2.1.281, which keeps no
copy on disk: its text was taken from what the Skill tool loads. It is checked
in so the review gate's skills are the same in every container and in the
fixer workflow, whatever Claude Code version they install.

Two local changes:

- The em dashes became colons, commas or semicolons, per `CLAUDE.md`.
- The fallback diff base is `origin/dev`, where pull requests go, not `main`.

To refresh, load the bundled skill in a checkout without this directory and
copy its text again.

## Name collision

`code-review/` and `simplify/` shadow the skills of the same name that ship
with Claude Code. Project skills win, so both run the vendored copy here.
