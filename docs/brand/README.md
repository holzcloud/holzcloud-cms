# The mark

**Wood plus element.** A wooden plank carries each project's element: the
holzcloud projects share the plank and differ in what stands on it. For this
CMS it is a mint cloud with annual rings cut into it, the cloud and the wood
of the name in one figure, standing on the plank that ties it to its siblings
(the cloud-crowned tree of holzcloud itself, the cube of holzkube-manager).

| File | Use |
|---|---|
| `holzcloud-mark.svg` | the standalone mark, mint cloud on the plank, for a dark ground: README, project page, anywhere it stands on its own |
| `holzcloud-mark-indigo.svg` | the same mark with the cloud in the admin's accent colour and the rings in paper |
| `holzcloud-tile.svg` | the mark inside a rounded square on ink: app icons, avatars |
| `banner.png` | the banner at the top of the repository's README, 2560×640 |
| `holzcloud-social.png` | the card GitHub shows when the repository is shared (Settings → Social preview), 2560×1280 |

The admin's own tab icon is `cmd/holzcloud/assets/favicon.svg`, which is the
tile: the ink square keeps the mint and the wood readable on a light tab bar as
well as on a dark one.

## Rules

- **The plank is the same in every project.** Its colours are fixed:
  `#C98B4F` for the board, `#E0A869` for the lit top edge, `#9B6534` for the
  grain and the knot. Only the element above it changes colour.
- **Three rings and no fourth.** The rings are drawn in the ground colour on top
  of the cloud and clipped to it; a fourth closes the gaps at small sizes.
- **Do not outline the cloud or the plank.** A stroke around either doubles the
  edge and thickens at small sizes.
- **Clear space** of half the mark's height on every side.
- **The banner and the social card are the mark on ink**, the name in Manrope,
  the typeface the shipped themes carry, and a row of what the program is. The
  same arrangement as the holzkube-manager's, in this program's colours, so the
  repositories read as one family. Both are rendered from HTML with the
  theme's own `manrope-latin.woff2` embedded and screenshotted at twice the
  size GitHub shows them; they are pictures in the repository, never something
  the program loads.

Colours: mint `#8FD0BA` on ink `#0B1411`, indigo `#4F3ED1` with paper
`#F6F4FF`, and the plank as above.

An operator can replace the mark for their own installation under *Brand* in the
admin — see [security](../security.md#the-brand-of-the-admin) for why an uploaded
SVG is checked rather than cleaned.
