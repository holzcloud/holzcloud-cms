---
phase: quick-260928-klh
plan: 01
subsystem: media
status: complete
tags: [media, webp, png, variants, srcset, thumbnails, cli]
requires: []
provides:
  - "hasTransparency(image.Image) bool; MakeVariants picks JPEG vs PNG from decoded pixels"
  - "Media.ThumbFilename filled by every store read via projection(table)"
  - "SaveVariants(ctx, dir, mediaID, width, height, variants) replaces the whole set and removes stale files after commit"
affects: [internal/media, internal/admin, internal/public, cmd/holzcloud]
tech-stack:
  added: []
  patterns:
    - "Format of derived copies decided from pixels (Opaque()), never from MIME type"
    - "Rows first, files second: stale files removed only after the transaction commits"
key-files:
  created:
    - internal/media/testdata/opaque-lossy.webp
    - internal/media/testdata/opaque-lossless.webp
    - internal/media/testdata/transparent.webp
    - cmd/holzcloud/cli_thumbnails_test.go
  modified:
    - internal/media/variants.go
    - internal/media/model.go
    - internal/media/store.go
    - internal/media/variant_store.go
    - internal/media/backfill.go
    - internal/media/variants_test.go
    - internal/admin/media.go
    - internal/admin/media_crop.go
    - internal/admin/media_crop_test.go
    - internal/admin/media_screen_test.go
    - internal/public/album_test.go
    - cmd/holzcloud/cli.go
    - docs/deployment.md
    - CHANGELOG.md
decisions:
  - "JPEG copies unless the decoded picture has at least one pixel that is not fully opaque (stdlib + x/image only, no WebP encoder)"
  - "Admin grid/picker read the stored thumb name from media_variants in the same query (correlated subquery), fall back to the original"
  - "SaveVariants replaces the whole set; stale files deleted only when the name is a bare name, not the original and not its crop source"
metrics:
  duration: "28 min"
  completed: 2026-09-28
actuals:
  tokens: 13800
  tasks: 3
  commits: 3
plan_head_before: f49319e87622e1fe1170b5950a8c6f66842a0741
plan_head_after: 24099fec2ed15e2887cef153837fd5bc4902202c
---

# Quick 260928-klh Plan 01: Opaque WebP/PNG uploads get JPEG size variants Summary

The format of the copies now depends on the decoded pixels (`hasTransparency` via `Opaque()`). An opaque WebP or PNG gets JPEG copies, so the 800px medium copy is kept and appears in the public srcset. The admin grid and the picker use the thumbnail name stored in `media_variants`. `SaveVariants` replaces the whole set and deletes stale files after commit, so `holzcloud thumbnails -force` replaces an install's old PNG copies without leaving anything behind.

## Tasks

| Task | Name | Commit | Key files |
| ---- | ---- | ------ | --------- |
| 1 (tracer) | Pixel-based format choice, stored thumb name, WebP fixtures | bc6fc7a | variants.go, model.go, store.go, variants_test.go, testdata/*.webp |
| 2 | SaveVariants replaces the whole set; `thumbnails -force` swaps .png copies for .jpg | bd1aa8e | variant_store.go, backfill.go, admin/media.go, admin/media_crop.go, cli.go, cli_thumbnails_test.go |
| 3 | Admin grid test, CHANGELOG 0.0.6, `thumbnails [-force]` usage/docs, full gate | 24099fe | media_screen_test.go, CHANGELOG.md, cli.go, docs/deployment.md |

Tracer gate: after Task 1 I re-ran `<verify>` end to end and it passed before expanding. TDD: I wrote the tests for Tasks 1 and 2 first. Both failed to compile (missing `ThumbFilename`/`hasTransparency`, then the old `SaveVariants` signature), and both then passed after the implementation. Each task went into one combined commit, as quick tasks do.

## Fixtures

I generated them once with Python 3 + Pillow 11.1.0. No script is committed. Run from the repo root after `mkdir -p internal/media/testdata`:

```python
import math, random
from PIL import Image, ImageDraw
W,H=1000,560
random.seed(1)
im=Image.new("RGB",(W,H)); px=im.load()
cl=lambda v:max(0,min(255,v))
for y in range(H):
    for x in range(W):
        r=int(128+100*math.sin(x/97)*math.cos(y/53)); g=int(110+90*math.sin((x+y)/140)); b=int(90+60*math.cos(x/211))
        n=random.randint(-10,10)
        px[x,y]=(cl(r+n),cl(g+n),cl(b+n))
im.save("internal/media/testdata/opaque-lossy.webp","WEBP",quality=80,method=6)
im=Image.new("RGB",(W,H),(245,245,245)); d=ImageDraw.Draw(im)
d.rectangle([0,0,W-1,47],fill=(30,60,120))
i=0
for y in range(48,H,28):
    w=200+(i*137%700); d.rectangle([40,y,40+w-1,y+11],fill=(60,60,60)); i+=1
im.save("internal/media/testdata/opaque-lossless.webp","WEBP",lossless=True)
im=Image.new("RGBA",(W,H),(0,0,0,0)); d=ImageDraw.Draw(im)
d.ellipse([100,60,900,500],fill=(200,40,40,255))
im.save("internal/media/testdata/transparent.webp","WEBP",lossless=True)
```

Measured sizes with the real Go pipeline (CatmullRom, JPEG q82, png.Encode):

| File | Size |
| ---- | ---- |
| opaque-lossy.webp (original, 1000x560) | 82,070 B |
| 800px medium as JPEG (kept) | 44,466 B |
| 800px medium as PNG (what the old rule made, then dropped) | 528,303 B |
| 400px thumb JPEG / PNG | 8,534 B / 123,307 B |
| opaque-lossless.webp | 190 B |
| transparent.webp | 1,200 B |

The medium JPEG is 46% smaller than the original, well past the 20% threshold, so I did not change the noise amplitude.

## Live rollout

Run this only **after the new image is rolled out**. The old binary would write PNG copies again.

```
kubectl -n holzcloud exec deploy/holzcloud-cms -- /holzcloud thumbnails -force
```

Expected last line: `N images processed, M failed`. Any failed file is also listed above it on stderr, and it keeps its old set, which is still valid.

Check afterwards: fetch the page that embeds the 2400px WebP from the problem statement. Its `srcset` should now contain a `/media/1/<hash>-medium.jpg 800w` candidate (and `-large.jpg 1600w` if that one is smaller than the original) instead of only `<hash>-thumb.png 400w, <hash>.webp 2400w`. The media library cards should load `-thumb.jpg` for opaque pictures.

Cache window: public HTML is served with `max-age=300` and a content-hash ETag. For up to five minutes, a page cached in a browser before the run may ask for an old copy that `-force` has just deleted. Reloading fixes it. The CHANGELOG says this too.

## Deviations from Plan

### Auto-fixed Issues

None. The plan was executed as written, with the following choices within its latitude:

- `MakeVariants` now refuses a MIME type that `CanMakeVariants` rejects, as the plan asked. I added `TestMakeVariantsRefusesAnimation` to cover it; the plan did not list that test.
- In `SaveVariants` the width/height UPDATE runs after the INSERTs, inside the same transaction. The order has no effect on the result.
- `writeTestImage(transparent=true)` now writes an NRGBA PNG whose top quarter is alpha 0. `TestMakeVariantsKeepsTransparency` and `TestMakeVariantsRefusesOversizedImage` still pass.

## Observations left unchanged (out of scope)

- `internal/media/crop.go ApplyCrop` still uses its own MIME-based transparent flag, and re-encodes a cropped WebP as PNG bytes under the `.webp` name. This is pre-existing and, as the plan said, not a variant.
- T-q260928klh-06 (accepted): if encoding fails part-way through (for example a full disk), `MakeVariants` deletes the copies it wrote in that run. Some of those may share a name with a row that is still stored. This is pre-existing.
- `cmdThumbnails`: if `MakeVariants` succeeds and `Dimensions` then fails, the files just written are orphaned while the old rows stay valid. That is a leak, not a dangling row. It is pre-existing ordering and very unlikely, since `MakeVariants` has already read the header.

## Gate results

- `gofmt -l .`: empty
- `go mod tidy` + `git diff --exit-code go.mod go.sum`: unchanged
- `go vet ./...`: clean
- `go run ./tools/english`: "no German in the Go source outside the catalogues"
- `go run ./tools/cites -check`: 0 citations, 0 unresolvable
- `go run ./tools/i18n`: de/es/fr/it 1946 translated, 0 open, 0 orphaned
- `go test` (all packages except admin, `-v`): 52 packages ok, 0 FAIL; 1492 `--- PASS` lines (top-level tests and subtests), 0 `--- FAIL`, 0 SKIP
- `go test ./internal/admin/ -timeout 40m -v`: ok; 468 `--- PASS`, 0 `--- FAIL`, 0 SKIP (includes `TestTheGridShowsTheThumbnailThatExists`)

## Known Stubs

None.

## Threat Flags

None. The only new filesystem surface is the stale-file removal in `SaveVariants`, which is T-q260928klh-01. It is mitigated: bare names only, only inside `dir`, never the original or `SourceName(original)`. `TestRegenerationDeletesNothingItShouldNot` covers it.

## Self-Check: PASSED
