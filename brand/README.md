# Brand assets

Images that represent the product somewhere this repository does not build.
Anything a build consumes lives beside that build instead: the application
icons are in `app/build/`, the site's mark and link preview card in `web/`.

| File | Where it goes |
| ---- | ------------- |
| `github-avatar.png` | The GitHub organisation and repository avatar |

Both this and the macOS icon source in `app/build/` are reframings of
`app/build/icon-default.png`, derived by `icons.mjs` rather than drawn again:

```
./dev.sh app icons
```

macOS draws a legacy `.icns` exactly as authored, and its convention is an
824px body centred on a 1024 canvas with a soft shadow. Windows and Linux want
the artwork to fill the canvas, which `icon-default.png` already does. GitHub
crops an avatar square and rounds it itself, so that one is the same mark
flattened onto its tile colour, with the corners squared off.

Run it after changing the mark, and commit what changed.
