# Shrimp mark drafts

Drafts for replacing the bowtie mark with a friendly shrimp. Nothing here
ships yet: `web/logo.png`, `web/favicon.png` and `app/build/appicon.png` are
untouched.

`shrimp.py` generates every SVG in `drafts/`:

```sh
python3 docs/brand/shrimp.py
```

## Colours

The two the logo already uses, plus the same hue pushed either way:

| Token | Hex | Use |
| ----- | --- | --- |
| brand | `#AC2D31` | the body, sampled from `web/logo.png` |
| paper | `#FAFAFA` | eyes, and the tile behind the mark |
| pupil | `#7C1E22` | pupils and the smile |
| joint | `#C7484C` | shell lines |

## Variants

Two families. The mark and the character are separate artefacts, not one
drawing at two sizes, which is how Rust treats its logo and Ferris.

| Draft | Role |
| ----- | ---- |
| `curl` | The mark, full detail |
| `curl-reduced` | The mark below about 24px |
| `curl-open` | Shallower sweep, lighter weight |
| `curl-tight` | Tighter sweep, heaviest, most compact |
| `curl-badge` | Knocked out of a disc, for circular crops |
| `curl-mirror` | Facing into the wordmark |
| `mascot` | The character, with swimmerets |
| `mascot-wave` | Raised claw |
| `mascot-chibi` | Larger head, shorter body |
| `mascot-peek` | Cropped by a ledge, for empty states |
| `tile-red` | Knocked out of a brand-red square |
| `tile-paper` | On the off-white square the icon uses now |

## Silhouette complexity

Small-size legibility tracks the number of path commands in the silhouette,
so the generator is set up to control it. Comparable marks, each reduced to
one silhouette and counted the same way:

| | commands |
| - | - |
| Vercel | 3 |
| Linear | 21 |
| DBeaver | 39 |
| Bruno | 70 |
| Docker | 94 |
| Rust (Ferris) | 194 |

Abstract marks sit between 3 and 23; animal mascots between 39 and 194.
Keep the small cut under about 40. `curl-reduced` is 37 against `curl` at
167, and the two are near-identical at a glance -- almost all of that came
from the sample count, not from dropping features.

## Editing the geometry

Bodies are a centreline plus a width profile rather than hand-written
beziers, because at this scale a hand-drawn taper wobbles and the variants
have to stay the same animal. Four constraints are baked in:

- `cr()` emits one cubic per sample, so `n` is the main cost control. `n=34`
  spends 70 commands on the body outline before anything else is drawn;
  `n=11` looks the same at logo sizes.
- The spine sweeps about 220 degrees. More closes the curl into a ring and
  the silhouette stops reading as a shrimp.
- Width has to stay well under the spine radius. When it does not, the inner
  offset folds back on itself and the nonzero fill rule punches a hole
  through the body.
- Nothing comes to a point. The tail is round-capped strokes, not blades,
  and the head is a circle rather than a rostrum. That is most of what
  separates the cartoon from the earlier realistic drafts.

Choosing one means redrawing it against the wordmark for optical weight,
then regenerating the three PNGs above.
