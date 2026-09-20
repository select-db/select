# Brand assets

Images that represent the product somewhere this repository does not build.
Anything a build consumes lives beside that build instead: the application
icons are in `app/build/`, the site's mark and link preview card in `web/`.

| File | Where it goes |
| ---- | ------------- |
| `github-avatar.png` | The GitHub organisation and repository avatar |

`github-avatar.png` is `app/build/icon-default.png` flattened onto its own
tile colour, which squares off the rounded corners. GitHub crops an avatar to
a square and rounds it itself, so a corner radius baked into the file is drawn
once by us and once by them.
