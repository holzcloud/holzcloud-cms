# Handoff to release 0.1.1 (written 2026-10-31)

Owner's standing orders are in CLAUDE.md ("Working Style", "After every release"): do everything yourself, merge your own PRs after green CI, never `--admin`.

## State
- origin/main = `ed2d7cb` (PR #54): code of phases 24-30 is on origin. SRV-01 recorded PASS (native amd64 run 41200578718, evidence commit 3a29a1c).
- Local `main` still has 170 older commits that origin does not have (planning files, history). origin no longer versions `.planning/`. Do NOT rebase them; work from a branch off `origin/main` (git worktree) for anything that goes to GitHub. Branch names must not start with `release/` (a branch `release` exists on origin).
- The memory tool refuses the "merge yourself" note (classifier: instruction poisoning); CLAUDE.md carries it instead.
- SRV-02 and SRV-03 stay `Pending owner` (need the owner's real server). The release states that honestly.
- GitHub releases so far: v0.1.0 (Pre-release). Next: v0.1.1, milestone "Beta belegt". Milestone number = release tag; published tags are never moved.

## Left to do, in order
1. Web UI stage label: `internal/web/layoutdata.go` `IsAlpha` knows only alpha (major < 1) or nothing. Owner wants number + stage alpha / beta / nothing. Add a beta level (e.g. `-beta` suffix or a stage constant), show it in `cmd/holzcloud/templates/admin/base.html` sidebar and login, translate via the catalogue (`go run ./tools/i18n` must end `0 open, 0 orphaned`), add tests next to `internal/web/alpha_test.go`. The release workflow injects `main.Version` from `git describe --tags`.
2. Check the CHANGELOG 0.1.1 entry and `docs/beta.md` (phase 28 checker: `28-check.sh all`).
3. `/gsd-complete-milestone` for v0.1.1, then tag `v0.1.1` on origin/main via PR-merged state; `release.yml` publishes on a `v*` tag. Mark it a Pre-release if the stage is beta.
4. After the release: fresh screenshots and texts for the GitHub repo and the website, update both.
