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

Check before committing:

```sh
git diff --cached | grep -nP '^\+.*[\x{2014}\x{2013}\x{2026}\x{2018}\x{2019}\x{201C}\x{201D}]'
```

## Comments

Comments say why, not what. The code already says what it does; a comment earns
its place by explaining the constraint, the failure it avoids, or the decision
that is not obvious from reading the lines below it. Match the density of the
file you are in rather than adding a header to every function.

## Bindings

The frontend talks to Go through the generated bindings in
`app/frontend/src/lib/bindings`. Prefer composing what exists over adding a new
bound method: a new service method is new API surface to keep, to document and
to regenerate, and most operations are a sequence of the filesystem calls that
are already there. Regenerate with `wails3 task generate:bindings` (never hand
edit the generated files).
