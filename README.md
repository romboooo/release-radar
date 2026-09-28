# release-radar

Minimal GitHub release tracker.

## CLI

```sh
rradar add owner/repo
rradar list
rradar check
rradar history
rradar delete owner/repo
```

## TUI

Run without arguments:

```sh
rradar
```

Keys:

- `j` / `k` or arrows: move selection
- `tab` / left / right: switch views
- `a`: add repository
- `d`: delete selected repository
- `c`: check releases
- `r`: refresh current view
- `q`: quit
