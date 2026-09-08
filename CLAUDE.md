# Working in this repository

## Typography: no smart punctuation, anywhere

Never write these characters in this repository. Not in code, not in comments,
not in `.doc.md` files, not in commit messages, not in test names or fixtures:

| Do not write | Write instead |
| ------------ | ------------- |
| `—` em dash  | ` -- `, or rewrite the sentence with a comma or a colon |
| `–` en dash  | `-` |
| `…` ellipsis | `...` |
| `’` `‘`      | `'` |
| `“` `”`      | `"` |

They mark text as machine-written, they are a nuisance to type, and they break
grep: someone searching for `don't` does not find `don’t`. Prose reads fine
without them.

Rewriting the sentence is usually better than substituting a dash. A pair of em
dashes around an aside is nearly always a comma, a colon, or two sentences.

Check before committing. The `(*UTF)` is load-bearing: without it PCRE reads
the escapes as bytes and refuses the pattern.

```sh
git diff --cached | grep -nP '(*UTF)^\+.*[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]'
```

No output means the staged diff is clean.

## Comments

Comments say why, not what. The code already says what it does; a comment earns
its place by explaining the constraint, the failure it avoids, or the decision
that is not obvious from reading the lines below it. Match the density of the
file you are in rather than adding a header to every function.

## Compose what exists before adding a layer

Most of what looks like a missing capability is a composition of parts that are
already here. Check for that first. The cost of a new part is not the code, it
is the second place a rule now lives, and the day the two copies disagree.

A worked example from this repository. Naming a database directory looked like
it needed Go: picking a free name means reading the folder it is going into. But
`fs_provider.Rename` already refused a name another entry in the folder had,
already refused one that would leave the workspace, and already no-opped a
rename to the same place. Deleting the new service methods and calling `Mkdir`,
`Write` and `Rename` halved the change, left the generated bindings untouched,
and gave folders and databases one set of naming rules rather than two that
could drift apart.

Before writing a new service method, store, helper or abstraction, ask:

- **Does something already do this, or do two things compose into it?** Read
  the code you are about to duplicate. The guard you are about to write is
  often already in the function you were going to call.
- **Is the new thing an existing thing plus a marker?** A database is a folder
  holding a `db.config.json`. Where that is true, make it behave like the
  existing thing. A special case needs its own validation, error messages,
  tests and documentation; a composition inherits all four.
- **Does the rule this encodes live anywhere else?** Two copies of a naming
  rule, a permission check or a path guard will diverge, and the bug will be
  reported against whichever copy the user happened to reach.
- **Would deleting it lose behaviour, or only lose code?** If only code, it was
  not a layer.

Composing is not always right. Add the new part when the operation has to be
atomic, when it needs something the frontend cannot see, or when the
composition would duplicate a rule rather than reuse one. Say which of those it
is in the commit message.

### Bindings specifically

The frontend talks to Go through the generated bindings in
`app/frontend/src/lib/bindings`. A new bound method is API surface to keep, to
document and to regenerate, so it wants the questions above answered first.
Regenerate with `wails3 task generate:bindings`, and never hand edit the
generated files.
