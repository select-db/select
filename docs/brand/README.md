# Shrimp mark drafts

Drafts for replacing the bowtie mark with a shrimp. Nothing here ships yet:
`web/logo.png`, `web/favicon.png` and `app/build/appicon.png` are untouched.

`shrimp.py` generates every SVG in `drafts/`. Run it from anywhere:

```sh
python3 docs/brand/shrimp.py
```

The bodies are a centreline plus a width profile rather than hand-written
beziers, because at this scale a hand-drawn taper wobbles and the seven
variants have to stay the same animal. Two constraints are baked into the
geometry and worth knowing before editing it:

- The sweep is about 220 degrees. More closes the curl into a ring, and the
  silhouette stops reading as a shrimp.
- The rostrum is the leading point of one continuous teardrop. Drawn as its
  own triangle it reads as a broken antenna.

| Draft | Role |
| ----- | ---- |
| `curl` | Primary mark |
| `curl-micro` | Small-size cut of `curl`, below 24px |
| `curl-solid` | `curl` without shell joints, single fill |
| `curl-grad` | `curl` with an orange-to-red gradient |
| `mascot` | Character cut, for docs and empty states |
| `dive` | Uncurled alternate |
| `tile` | App icon: the mark knocked out of the red square |

Colours come from the app theme (`app/internal/graph/defaults/user/.theme`):
`--orange` `#F97316`, `--orange-dark` `#EA580C`, `--red` `#DC3535`.

Choosing one means redrawing it against the wordmark for optical weight, then
regenerating the three PNGs above.
