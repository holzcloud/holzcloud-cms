# Phase 23: the steps only the owner can take

Claude ran none of the commands below. The development proxy allows branch pushes but refuses `refs/tags` and the deletion of refs, and a release, a tag and a branch deletion are your decisions in any case. This is the place where the roadmap's "the tag is set" is completed by a person.

## 0. Before pushing

The heading `## 0.1.0 — DATE` of `CHANGELOG.md` carries the day the entry was written. If the release is made on another day, edit that one line and commit it. The workflow ignores the date; the admin screen and the embedded changelog show it.

Read `docs/beta.md` once. It is the contract the release page links to.

## 1. Publish main

```bash
git push origin main
```

This comes first, and the order matters. The local `main` is ahead of `origin/main` by the work of Phases 20 to 23 (90 commits when this file was written) and by the merge of upstream's README. The records under `deploy/` and `docs/beta.md` exist on GitHub only after this push. The release body links to `docs/beta.md` and `deploy/DEPLOY.md` on `main`, so until `main` is pushed those links lead nowhere. The Actions button, moreover, builds the head of the default branch, which would be the stale one.

## 2. Make the release

```bash
git push origin main:release
```

This is route 2 of `.github/workflows/release.yml`. The `release` branch holds no work of its own and is only ever fast-forwarded. The push starts the Release workflow, which reads the first version heading of `CHANGELOG.md` (0.1.0), runs the whole test suite, builds linux/amd64, mints the tag `v0.1.0` through the Releases API after the tests have passed, and publishes the release as a pre-release, because every `v0.*` tag is one.

The alternative for a person: GitHub, Actions, Release, Run workflow, with no inputs (route 3), after step 1. With no inputs it does the same thing at the head of the default branch.

The release body is the `CHANGELOG.md` section for 0.1.0 followed by GitHub's generated notes. There is no separate notes file. If the tests fail, nothing is tagged and nothing is published.

## 3. Check it

```bash
git ls-remote --tags origin v0.1.0
gh release view v0.1.0 --json isPrerelease,body
```

The first prints one line. In the second, `isPrerelease` is `true` and the body opens with the text of the CHANGELOG entry.

The job `holzcloud-ch` writes the version into the holzcloud.ch pages only when the `CMS_TOKEN` secret exists. Without it the job prints a notice and succeeds without doing so.

## 4. If something is wrong afterwards

A push to `release` at a version that is already out moves no tag and rewrites only the release text from `CHANGELOG.md`. That is how a corrected entry reaches the page. A published tag is never moved.

## 5. Tidy the branches

Open `docs/branches.md`. If `main` has moved since its Generated line, regenerate the table with the commands the page prints. Delete the branches marked Safe to delete in the GitHub UI, by the steps on that page. The two Review first branches, `backfill-releases` and `tagfix`, need a decision from you first. `release` stays.

Nothing was deleted by Claude, because the development proxy answers 403 to the deletion of a ref.

## Veto points

These are the assumptions the phase made on your behalf. Each one can be overridden; the cost is given.

- **A-1. The word alpha stays in the product and in `deploy/DEPLOY.md`.** The product carries it in `web.IsAlpha` (`internal/web/layoutdata.go`), three admin templates and three catalogue keys in four languages. `deploy/DEPLOY.md` carries it in the Updating section. Overriding the product costs new catalogue keys translated into de, es, fr and it, the old keys deleted by hand, `go run ./tools/i18n -schweiz`, and the tests of `internal/web` and `internal/admin`. Overriding `deploy/DEPLOY.md` costs a rerun of the Phase 22 evidence set (`backup-chain`, about 238 seconds and the Docker VM, then `fresh` and `lifecycle`) and a new `Evidence commit` line.
- **A-2. The heading date is the day the entry was written** (see step 0).
- **A-3. Every 0.x release stays a pre-release.** `release.yml` says so, and its comment still calls the software alpha. That comment was not edited, because the workflow file is outside this phase.
- **A-4. The CHANGELOG entry is the whole release body**, and it carries the upgrade note for operators on 0.0.25.

## Outside the repository

Pages on holzcloud.ch outside this repository may still say Alpha in their status boxes; the release job writes only the version number there. Not done by this phase.

## What this phase did not do

No tag, no push, no deletion of any branch, no change to a workflow, to the product's strings or to the Phase 22 evidence set.
