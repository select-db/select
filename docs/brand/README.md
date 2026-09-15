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

## Cuts

| Draft | Role |
| ----- | ---- |
| `cartoon` | Primary mark |
| `cartoon-micro` | Small-size cut, below 24px |
| `cartoon-wave` | Waving pose, for docs and empty states |
| `cartoon-legs` | Swimmerets, fuller character |
| `cartoon-tile` | Mark on the off-white square |
| `cartoon-invert` | Knocked out of a brand-red square |

## Editing the geometry

Bodies are a centreline plus a width profile rather than hand-written
beziers, because at this scale a hand-drawn taper wobbles and the six cuts
have to stay the same animal. Three constraints are baked in:

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
