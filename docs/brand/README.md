# The mark

**Ringwolke** — annual rings cut into a cloud. Both halves of the name in one
figure: the wood and the cloud, drawn as the same thing rather than set beside
each other.

| File | Use |
|---|---|
| `holzcloud-mark.svg` | the standalone mark, forest green — README, project page, anywhere it stands on its own |
| `holzcloud-mark-indigo.svg` | the same mark in the admin's accent colour |
| `holzcloud-tile.svg` | the mark inside a rounded square, forest — app icons, avatars, social cards |
| `banner.png` | the banner at the top of the repository's README, 2560×640 |
| `holzcloud-social.png` | the card GitHub shows when the repository is shared (Settings → Social preview), 2560×1280 |

The admin's own tab icon is `cmd/holzcloud/assets/favicon.svg`, which is the tile
in indigo. It carries **two** rings where the standalone mark carries three: a
browser tab is 16 pixels wide, and the third ring at that size is not a ring but
a smear. Same figure, one detail fewer — the ordinary way a mark is made to
survive being small.

## Rules

- **One accent plus paper.** The rings are cut out of the cloud, not drawn on top
  of it, so the mark works in a single colour and reversed on a dark ground.
- **Do not add a fourth ring.** The spacing is set so the smallest ring stays a
  filled dot at 32 px; a fourth closes the gaps.
- **Do not outline the cloud.** The silhouette is the clip path; a stroke around
  it doubles the edge and thickens at small sizes.
- **Clear space** of half the mark's height on every side.
- **The banner and the social card are the reversed mark on ink**, the name in
  Manrope — the typeface the shipped themes carry — and a row of what the
  program is. The same arrangement as the holzkube-manager's, in this
  program's colours, so the two repositories read as one family. Both are
  rendered from HTML with the theme's own `manrope-latin.woff2` embedded and
  screenshotted at twice the size GitHub shows them; they are pictures in the
  repository, never something the program loads.

Colours: forest `#1F4F42` on paper `#F7F4EE`, indigo `#4F3ED1` on `#F6F4FF`,
and `#8FD0BA` on ink `#14201C` when reversed.

An operator can replace the mark for their own installation under *Brand* in the
admin — see [security](../security.md#the-brand-of-the-admin) for why an uploaded
SVG is checked rather than cleaned.
