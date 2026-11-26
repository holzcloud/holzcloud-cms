#!/usr/bin/env bash
# The gate of phase 34: the promises of 0.1.5 that rest on the records of phase 34, and
# the evidence those records bind.
#
#   bash deploy/test/34-check.sh RULE...
#
# RULE is one of frozen, quotes, coverage, ledger, nojs, record, stable, all, selftest:
#
# frozen    the evidence still holds: each record of RECORDS has exactly one
#           `Evidence commit:` line naming a commit of ROOT, and between that commit and
#           the judged tree (AT, else the working tree) the bound files of its driver did not change and the source of the binary
#           (cmd, internal, go.mod, go.sum, changelog.go, CHANGELOG.md) changed in nothing
#           but CHANGELOG.md, and there only by new version sections: one block of lines
#           inserted directly above the older first `## ` line, whose level-two headings are
#           all `## X.Y.Z — YYYY-MM-DD` of a version no older heading has, with every older
#           line and everything above the first heading byte for byte as it was. RECORD_AT
#           must be an ancestor of AT (HEAD without it) and must hold the full binary source (CHANGELOG.md
#           included) of every evidence commit. The bound files
#           are read from the `BOUND_FILES=` line of the driver that made the record
#           (deploy/test/upgrade-verify.sh for the three container records,
#           deploy/test/browser-verify.sh for deploy/BROWSER-RUN.md,
#           deploy/test/backup-verify.sh for deploy/BACKUP-VERIFY-RUN.md), so the list here
#           cannot drift from the list the driver refuses to run without. And the files the
#           phase changed (BASE...TIP) are exactly PHASE_FILES. BASE is required; the rule
#           refuses to run without it. TIP is HEAD unless set; after the merge of phase 34
#           it is set to that merge, as 32-check.sh does for phase 32, and it only moves
#           the list of the phase's files: the bound files and the binary's source are
#           always compared with the judged tree, AT or else the working tree.
# quotes   every required row of docs/beta.md (REQUIRED_ROWS) exists once, names one of
#           the records of 0.1.5 (RECORDS) in its Record column, and every phrase of its
#           Cited words column is found in that record by grep -F, byte for byte. A phrase
#           is a sentence between straight double quotes or a backticked token outside
#           them. A phrase that spans a line break of the record, or that holds a backtick
#           and is cited as a backticked token, is not found, and the rule fails.
# coverage  every limit of docs/beta.md marked (0.1.5) names exactly one record with
#           `Record: deploy/X`, and X is one of RECORDS, so no limit of 0.1.5 rests on
#           nothing; and every sentence such a limit quotes between straight double quotes
#           is found in the record it names, byte for byte. deploy/test/32-check.sh checks
#           the quotes of the limits that name deploy/BACKUP-VERIFY-RUN.md as well; since
#           phase 34 it no longer refuses a limit that names no record, and this rule does.
# ledger    the four limits of 0.1.1 that the records of phase 34 answer (LIMIT_1 to
#           LIMIT_4) each begin the first cell of a row of the ledger table of 0.1.5
#           (header `| 0.1.1 said | 0.1.5 says | Promise rows |`), each of those rows names
#           promise rows that exist, and no bullet of the section *What X does not promise*
#           still begins with one of them; the bullet of 0.1.1 that said an older binary
#           must not be started against a migrated database is gone, *The product itself*
#           names deploy/ROLLBACK-RUN.md, and *What beta means here* names P-30.
# nojs      the tracked JavaScript of the repository (`git ls-files` of *.js, *.mjs and
#           *.cjs) is exactly JS_ALLOWED, the htmx file: the browser record of phase 34
#           ran a Playwright script, and that script is pasted into the record, never
#           committed as a file.
# record    when E holds amd64-t34-summary.txt and mac-t34-uat, the record-check of
#           deploy/test/upgrade-verify.sh for each of its three records and that of
#           deploy/test/browser-verify.sh prove the records against their scratch files.
#           They run in a temporary git worktree at RECORD_AT, removed again, after the
#           rule has proved that RECORD_AT is an ancestor of AT (HEAD without it), that the
#           binary's source differs from it at most by new version sections of CHANGELOG.md,
#           and that the bound files of both drivers are equal at RECORD_AT and in the judged
#           tree, so the driver code that runs is byte for byte the code that is judged. When E32 holds amd64-t32-src, the
#           gate of phase 32 runs whole at ROOT after its
#           merge (`BASE=e1116b3 TIP=93bfb80 E="$E32" bash deploy/test/32-check.sh all`),
#           which runs the record-check of deploy/test/backup-verify.sh. Every one must end
#           OK, and the gate of phase 32 must end `32-CHECK OK (` with no skipped rule
#           (the count of its rules is its own business). A set that is only partly there
#           fails the rule: E with only one of amd64-t34-summary.txt and mac-t34-uat, E32
#           with files named amd64-t32-* and no amd64-t32-src. Only when a set is missing
#           whole does the rule print `SKIP record: …` and count as skipped, never as
#           passed; see the last line below for what a skip does to the exit status.
# stable    docs/stable.md (STABLE) says what 1.0 will promise and what is missing: its first
#           line is STABLE_TITLE; its level-two headings are exactly the five areas of
#           STABLE_AREAS and then those of STABLE_REST, in that order; each area has
#           `### What 1.0 promises` with at least one line of prose, then
#           `### What is still missing` with at least one `- [ ] ` item, nothing in it but
#           `- [ ] ` items, and no `- [x]`; every `- [ ] ` item names a requirement ID
#           (`[A-Z]{2,4}-[0-9]{2}`), an owner step (`OS-[0-9]+`) or a backticked path with a
#           slash, so that no item stands on nothing; `## Outside the 1.0 promise` names
#           `plugin ABI`, `SDK` and `MCP`; every `deploy/….md` and `docs/….md` path the
#           page names exists in ROOT; and the README (README) links `docs/stable.md`.
#           The page is a promise not yet given, written in the future tense, and what it
#           states as present is cited by these checks only as far as a path can be: the
#           rule cannot read a sentence, and the review against D-01 to D-05 does.
# all       frozen, quotes, coverage, ledger, nojs, record, stable, in that order.
# selftest  one negative case per rule, each beside a control that passes, on copies in a
#           temporary directory; the real files are never written.
#
# Why RECORD_AT. A release adds its CHANGELOG.md entry after the evidence commits, and the
# records prove nothing about the text of the changelog (the owner's decision of 2026-11-24;
# the seventh command of docs/beta.md already leaves it out). The drivers embed CHANGELOG.md
# in their binary source, compare it with HEAD, and are bound files of their own evidence,
# so they cannot change. Their record-checks therefore run where HEAD is RECORD_AT, after
# this gate has proved the step from RECORD_AT to the judged tree.
#
# Why a second gate. deploy/test/32-check.sh is the gate of phase 32 and checks the rows
# that rest on deploy/BACKUP-VERIFY-RUN.md; it stays as it is, because it is the gate of
# record of that phase. Each phase's gate checks its own rows. This one is its sibling: the
# same shape, the same reading of a row and of a phrase, for the rows that rest on the
# records phase 34 made.
#
# Environment:
#   ROOT     the repository; default: the repository that holds this file. Every git
#            command of this file runs as `git -C "$ROOT"`, so the gate works from a
#            worktree and from a copy of the file elsewhere.
#   REC_ROOT where the records are read from; default ROOT. The names in RECORDS are
#            relative to it.
#   BASE     the commit the phase branched from (frozen only), for example
#            BASE=$(git merge-base origin/main HEAD)
#   TIP      the end of the phase's change (frozen only); default HEAD
#   AT       the commit this gate judges as the state of the repository (frozen and record:
#            the bound files, the source of the binary, the ancestry of RECORD_AT); default
#            empty, which means the working tree of ROOT, untracked files included. After a
#            release the live HEAD moves on, so a gate of that release is proved by naming
#            the release commit (AT=v0.1.5); without AT a local run sees an edit that is not
#            committed yet (IN-04 of the review of phase 34). The files the phase changed
#            (BASE...TIP) are always commits.
#   RECORD_AT  the newest commit whose binary source is that of the evidence commits (the
#            head of main when 0.1.5 was audited), default daf307f; record runs the
#            drivers' record-checks there, and frozen proves it
#   BETA     the page of promises; default $ROOT/docs/beta.md
#   E        the scratch root of the evidence sets of phase 34, amd64-t34-* and mac-t34-*
#            (record only); default $ROOT/deploy/test/out
#   E32      the scratch root of the evidence set of phase 32, amd64-t32-* (record only);
#            default $ROOT/deploy/test/out
#   PHASE_FILES  the files the phase changed, one per line; default the list below. Only
#            the selftest sets it.
#   STABLE   the page of what 1.0 will promise; default $ROOT/docs/stable.md
#   README   the file that must link it; default $ROOT/README.md
#   STABLE_AREAS, STABLE_REST  the level-two headings of the page, one per line: the areas
#            that carry a promise and a checklist, and the sections after them; defaults
#            below.
#   JS_ALLOWED  the tracked JavaScript allowed (nojs only); default
#            cmd/holzcloud/assets/htmx.min.js. Only the selftest sets it to empty.
#
# The last line is `34-CHECK OK (N rules)`, `34-CHECK OK (N rules, S skipped)`,
# `34-CHECK NOT PROVEN (0 rules, S skipped)` or `34-CHECK FAILED (K of N)`; the selftest
# ends `SELFTEST OK (N cases)` or `SELFTEST FAILED (K of N)`. A skipped rule is not among
# the N. Exit status: 0 every rule that ran passed and at least one ran, 1 a rule failed,
# 2 usage or a rule could not run, 77 every requested rule was skipped, so nothing was
# proven (CI treats 77 as skipped, not as green). When some rules passed and one was
# skipped, the exit status is 0, because what ran did pass, but the line before the last
# is `NOT PROVEN: RULE...` naming each skipped rule, so a log that ends `34-CHECK OK (6
# rules, 1 skipped)` also says which promise was not checked. `all` in a checkout without
# the scratch roots is exactly that case: it proves the prose rules and not the binding of
# the records to their scratch files.
#
# bash 3.2 and BWK awk (macOS): no mapfile, no associative arrays, no gawk extensions.
set -u
export LC_ALL=C

HERE=$(cd "$(dirname "$0")" && pwd)
if [ -z "${ROOT:-}" ]; then
  ROOT=$(git -C "$HERE" rev-parse --show-toplevel 2> /dev/null) || ROOT=$(cd "$HERE/../.." && pwd)
fi
ME="$HERE/$(basename "$0")"
REC_ROOT=${REC_ROOT:-$ROOT}
BETA=${BETA:-$ROOT/docs/beta.md}
JS_ALLOWED=${JS_ALLOWED-cmd/holzcloud/assets/htmx.min.js}
STABLE=${STABLE:-$ROOT/docs/stable.md}
README=${README:-$ROOT/README.md}
STABLE_TITLE='# What 1.0 will promise, and what it still lacks'
STABLE_AREAS=${STABLE_AREAS-"Template contract
Data contract
Migrations
Upgrade and rollback
Supported platforms"}
STABLE_REST=${STABLE_REST-"Outside the 1.0 promise
Not yet decided"}
E=${E:-$ROOT/deploy/test/out}
E32=${E32:-$ROOT/deploy/test/out}
BASE=${BASE:-}
TIP=${TIP:-HEAD}
AT=${AT:-}
RECORD_AT=${RECORD_AT:-daf307f}
# The drivers now run in another directory, so a path given relative is made absolute here,
# as the caller meant it.
for v in ROOT REC_ROOT E E32; do
  eval "x=\${$v}"
  case "$x" in /*) ;; *) eval "$v=\"\$PWD/\$x\"" ;; esac
done

# The source of the binary under test, as the drivers name it (BINARY_SOURCE). CHANGELOG.md
# is the one path src_since lets a release change, and only by new version sections.
BINARY_SRC="cmd internal go.mod go.sum changelog.go CHANGELOG.md"

# Every file phase 34 changed, sorted as `sort` prints them under LC_ALL=C. The frozen rule
# compares the real change BASE...TIP with it, name for name.
PHASE_FILES=${PHASE_FILES:-"README.md
cmd/holzcloud/cli.go
cmd/holzcloud/deploy_docs_test.go
cmd/holzcloud/main.go
cmd/holzcloud/schema_guard_test.go
deploy/BACKUP-VERIFY-RUN.md
deploy/BROWSER-RUN.md
deploy/DEPLOY.md
deploy/MIGRATION-RESTORE-RUN.md
deploy/ROLLBACK-RUN.md
deploy/UPDATE-CHAIN-RUN.md
deploy/test/32-check.sh
deploy/test/34-check.sh
deploy/test/README.md
deploy/test/browser-verify.sh
deploy/test/upgrade-verify.sh
docs/beta.md
docs/stable.md
internal/db/db.go
internal/db/partial_migration_test.go
internal/plugin/runtime.go
internal/plugin/runtime_test.go"}

# The records of 0.1.5, relative to REC_ROOT.
RECORDS="deploy/MIGRATION-RESTORE-RUN.md deploy/UPDATE-CHAIN-RUN.md deploy/ROLLBACK-RUN.md deploy/BROWSER-RUN.md deploy/BACKUP-VERIFY-RUN.md"
# The rows of docs/beta.md that must exist and cite a record of 0.1.5.
REQUIRED_ROWS="P-27 P-28 P-29 P-30 P-31 P-32 P-33 E-9 E-10 E-11 E-12"

# The four limits of 0.1.1 that the records of phase 34 answer, by their first words.
LIMIT_1='A restore that crosses a migration was not played'
LIMIT_2='Five or more successive updates and the pruning of the pre-upgrade snapshots were not played'
LIMIT_3='A rollback after a failed migration was not played'
LIMIT_4='The compulsory two-factor enrolment and the shop and form flows through a browser were not played'
# The bullet of 0.1.1 under *The product itself* that phase 34 rewrote, by its first sentence.
SCHEMA_OLD='- The schema and the migrations are not promised stable, and an older binary must not be started against a migrated database.'
LEDGER_HEAD='| 0.1.1 said | 0.1.5 says | Promise rows |'

TMPROOT=$(mktemp -d) || exit 2
# WT is the worktree of RECORD_AT while the record rule has one. It is removed by name, never
# with git's prune subcommand, which would touch worktrees that are not this gate's.
WT=""
cleanup() {
  [ -z "$WT" ] || git -C "$ROOT" worktree remove --force "$WT" > /dev/null 2>&1
  rm -rf "$TMPROOT"
}
trap cleanup EXIT

RULE_FAIL=0
SKIP=77
fail() { echo "FAIL: $*"; RULE_FAIL=$((RULE_FAIL + 1)); }
ok() { echo "ok: $*"; }
need() { [ -f "$1" ] || { fail "missing file: $1"; return 2; }; }

# ------------------------------------------------------------- helpers ----

# cell ROW N : the N-th cell of a table row, trimmed.
cell() { printf '%s\n' "$1" | awk -F'|' -v n="$(($2 + 1))" '{ x = $n; gsub(/^ +| +$/, "", x); print x }'; }
# quotes_of TEXT : each "quoted" span, one per line; nonzero when the quotes do not pair up.
quotes_of() {
  printf '%s\n' "$1" | awk '{
    n = split($0, a, "\"")
    if (n % 2 == 0) bad = 1
    for (i = 2; i < n; i += 2) print a[i]
  } END { exit bad ? 1 : 0 }'
}
# unquoted_of TEXT : what is outside the quotes.
unquoted_of() { printf '%s\n' "$1" | awk '{ n = split($0, a, "\""); s = ""; for (i = 1; i <= n; i += 2) s = s " " a[i]; print s }'; }
# ticks_of TEXT : each backticked token without its backticks.
ticks_of() { printf '%s\n' "$1" | grep -o '`[^`]*`' | sed 's/^`//; s/`$//'; }
# phrases_of TEXT : the quoted sentences, then the backticked tokens outside them.
phrases_of() {
  quotes_of "$1"
  ticks_of "$(unquoted_of "$1")"
}
short() { printf '%s' "$1" | cut -c1-100; }
# is_record NAME : NAME is one of RECORDS.
is_record() { case " $RECORDS " in *" $1 "*) return 0 ;; esac; return 1; }

# driver_of RECORD : the driver that made the record and names its bound files.
driver_of() {
  case "$1" in
    deploy/MIGRATION-RESTORE-RUN.md | deploy/UPDATE-CHAIN-RUN.md | deploy/ROLLBACK-RUN.md) echo deploy/test/upgrade-verify.sh ;;
    deploy/BROWSER-RUN.md) echo deploy/test/browser-verify.sh ;;
    deploy/BACKUP-VERIFY-RUN.md) echo deploy/test/backup-verify.sh ;;
    *) return 1 ;;
  esac
}
# bound_of DRIVER : the files of the driver's single `BOUND_FILES="…"` line, as the judged
# tree holds the driver.
bound_of() {
  local src="$TMPROOT/bound-driver"
  if [ -n "$AT" ]; then
    git -C "$ROOT" show "$AT:$1" > "$src" 2> /dev/null || return 1
  else
    cp "$ROOT/$1" "$src" 2> /dev/null || return 1
  fi
  [ "$(grep -c '^BOUND_FILES=' "$src")" = 1 ] || return 1
  sed -n 's/^BOUND_FILES="\(.*\)"$/\1/p' "$src"
}
# not_in A B : the lines of A that are not lines of B.

# changelog_added OLD NEW : NEW is OLD, or OLD with one block of lines inserted directly
# above OLD's first level-two heading, in which every level-two heading is a new version
# heading (## X.Y.Z — YYYY-MM-DD, a version no older heading has). Prints one reason line
# and returns 1 when it is not. The prefix and the suffix are compared byte for byte, so
# an older line cannot be edited, moved or removed on the way.
changelog_added() {
  local old="$1" new="$2" k nold nnew a h v ere
  ere='^## [0-9]+\.[0-9]+\.[0-9]+ — [0-9]{4}-[0-9]{2}-[0-9]{2}$'
  cmp -s "$old" "$new" && return 0
  k=$(grep -n '^## ' "$old" | head -n 1 | cut -d: -f1)
  [ -n "$k" ] || { echo "CHANGELOG.md: the older file has no version heading"; return 1; }
  nold=$(wc -l < "$old" | tr -d ' ')
  nnew=$(wc -l < "$new" | tr -d ' ')
  a=$((nnew - nold))
  head -n "$((k - 1))" "$old" > "$TMPROOT/cl-a"
  head -n "$((k - 1))" "$new" > "$TMPROOT/cl-b"
  cmp -s "$TMPROOT/cl-a" "$TMPROOT/cl-b" || { echo "CHANGELOG.md: a line above its first version heading changed"; return 1; }
  if [ "$a" -le 0 ]; then
    echo "CHANGELOG.md: the change is not new lines directly above its first version heading; an older line changed, moved or was removed"
    return 1
  fi
  tail -n "+$k" "$old" > "$TMPROOT/cl-a"
  tail -n "+$((k + a))" "$new" > "$TMPROOT/cl-b"
  cmp -s "$TMPROOT/cl-a" "$TMPROOT/cl-b" || { echo "CHANGELOG.md: the change is not new lines directly above its first version heading; an older line changed, moved or was removed"; return 1; }
  sed -n "${k},$((k + a - 1))p" "$new" > "$TMPROOT/cl-block"
  head -n 1 "$TMPROOT/cl-block" | grep -qE "$ere" || { echo "CHANGELOG.md: the added lines do not begin with a version heading (## X.Y.Z — YYYY-MM-DD)"; return 1; }
  while IFS= read -r h; do
    printf '%s\n' "$h" | grep -qE "$ere" || { echo "CHANGELOG.md: an added level-two heading is not a version heading: $h"; return 1; }
    v=$(printf '%s\n' "$h" | awk '{ print $2 }')
    if V="$v" awk 'BEGIN { p = "## " ENVIRON["V"] " " } index($0, p) == 1 { f = 1 } END { exit f ? 0 : 1 }' "$TMPROOT/cl-a"; then
      echo "CHANGELOG.md: the added heading $h names a version an older heading already has"
      return 1
    fi
  done < <(grep '^## ' "$TMPROOT/cl-block")
  return 0
}
# at_ref : the ref of the judged tree for the ancestry checks, AT or else HEAD.
at_ref() { printf '%s' "${AT:-HEAD}"; }
# judged : the judged tree, as the messages name it.
judged() { if [ -n "$AT" ]; then printf 'AT %s' "$AT"; else printf 'the working tree'; fi; }
# at_ok RULE : AT, when it is set, is a commit of ROOT. A gate that cannot read the tree it was
# asked to judge must not say that nothing moved.
at_ok() {
  [ -z "$AT" ] && return 0
  git -C "$ROOT" rev-parse -q --verify "$AT^{commit}" > /dev/null || { fail "$1: AT '$AT' is not a commit of $ROOT"; return 1; }
}
# moved_since FROM PATH... : the names of the PATHs that differ between the commit FROM and the
# judged tree. With AT that is the commit AT. Without it, it is the working tree of ROOT
# (what is staged and what is edited), and a file git does not track yet counts as moved:
# an edit that is not committed is exactly what a local run must not pass over.
moved_since() {
  local from="$1"
  shift
  if [ -n "$AT" ]; then
    git -C "$ROOT" diff --name-only "$from" "$AT" -- "$@"
  else
    git -C "$ROOT" diff --name-only "$from" -- "$@"
    git -C "$ROOT" ls-files --others --exclude-standard -- "$@"
  fi
}
# src_since FROM : the source of the binary (BINARY_SRC) changed between FROM and the judged
# tree in nothing but new version sections of CHANGELOG.md. Prints the reason and returns 1 if
# not.
src_since() {
  local from="$1" names other
  # shellcheck disable=SC2086
  names=$(moved_since "$from" $BINARY_SRC)
  [ -n "$names" ] || return 0
  other=$(printf '%s\n' "$names" | grep -vxF CHANGELOG.md | tr '\n' ' ' | sed 's/ $//')
  if [ -n "$other" ]; then
    echo "beyond CHANGELOG.md: $other"
    return 1
  fi
  git -C "$ROOT" show "$from:CHANGELOG.md" > "$TMPROOT/cl-old" 2> /dev/null || { echo "CHANGELOG.md: not in $from"; return 1; }
  if [ -n "$AT" ]; then
    git -C "$ROOT" show "$AT:CHANGELOG.md" > "$TMPROOT/cl-new" 2> /dev/null || { echo "CHANGELOG.md: not in $AT"; return 1; }
  else
    cp "$ROOT/CHANGELOG.md" "$TMPROOT/cl-new" 2> /dev/null || { echo "CHANGELOG.md: not in the working tree"; return 1; }
  fi
  changelog_added "$TMPROOT/cl-old" "$TMPROOT/cl-new"
}
not_in() { printf '%s\n' "$1" | while IFS= read -r l; do [ -n "$l" ] || continue; printf '%s\n' "$2" | grep -qxF -- "$l" || printf '%s ' "$l"; done; }

# section2 HEADING-ERE < FILE : the lines under a level-two heading, up to the next one.
section2() {
  awk -v re="$1" '
    /^```/ { if (on) print; fence = !fence; next }
    fence { if (on) print; next }
    /^## / { if (on) exit; if ($0 ~ re) { on = 1; next } }
    on { print }
  '
}
# section3 HEADING-ERE < FILE : the lines under a level-three heading, up to the next
# heading of level two or three.
section3() {
  awk -v re="$1" '
    /^```/ { if (on) print; fence = !fence; next }
    fence { if (on) print; next }
    /^###? / { if (on) exit; if ($0 ~ re) { on = 1; next } }
    on { print }
  '
}
# ledger_rows < FILE : the data rows of the table whose header is LEDGER_HEAD.
ledger_rows() {
  H="$LEDGER_HEAD" awk 'BEGIN { h = ENVIRON["H"] }
    $0 == h { on = 1; n = 0; next }
    on && !/^\|/ { exit }
    on { n++; if (n > 1) print }'
}

# --------------------------------------------------------------- rules ----

rule_frozen() {
  if [ -z "$BASE" ]; then
    fail "frozen: BASE is required (the commit the phase branched from, e.g. the merge base of origin/main and HEAD)"
    return 2
  fi
  git -C "$ROOT" rev-parse -q --verify "$BASE^{commit}" > /dev/null || { fail "frozen: BASE '$BASE' is not a commit of $ROOT"; return 2; }
  git -C "$ROOT" rev-parse -q --verify "$TIP^{commit}" > /dev/null || { fail "frozen: TIP '$TIP' is not a commit of $ROOT"; return 2; }
  local rec f n sha drv bound moved got want held="" nb reason
  at_ok frozen || return 2
  git -C "$ROOT" rev-parse -q --verify "$RECORD_AT^{commit}" > /dev/null || { fail "frozen: RECORD_AT '$RECORD_AT' is not a commit of $ROOT"; return 2; }
  git -C "$ROOT" merge-base --is-ancestor "$RECORD_AT" "$(at_ref)" || fail "frozen: RECORD_AT $RECORD_AT is not an ancestor of $(at_ref)"
  for rec in $RECORDS; do
    f="$REC_ROOT/$rec"
    need "$f" || continue
    n=$(grep -c '^Evidence commit: ' "$f")
    [ "$n" = 1 ] || { fail "frozen: $rec has $n 'Evidence commit:' lines, want exactly one"; continue; }
    sha=$(sed -n 's/^Evidence commit: \([0-9a-f]*\)$/\1/p' "$f")
    if [ -z "$sha" ] || ! git -C "$ROOT" rev-parse -q --verify "$sha^{commit}" > /dev/null; then
      fail "frozen: the Evidence commit '$sha' of $rec is not a commit of $ROOT"
      continue
    fi
    drv=$(driver_of "$rec") || { fail "frozen: no driver is known for $rec"; continue; }
    bound=$(bound_of "$drv") || { fail "frozen: $drv has no single BOUND_FILES= line"; continue; }
    [ -n "$bound" ] || { fail "frozen: the BOUND_FILES= line of $drv is empty"; continue; }
    # shellcheck disable=SC2086
    moved=$(moved_since "$sha" $bound | tr '\n' ' ')
    if [ -n "$moved" ]; then
      fail "frozen: $rec: a bound file moved since the evidence commit $sha (judged: $(judged)): $moved"
    fi
    if ! reason=$(src_since "$sha"); then
      fail "frozen: $rec: the source of the binary moved since the evidence commit $sha (judged: $(judged)): $reason"
    fi
    # shellcheck disable=SC2086
    if ! git -C "$ROOT" diff --quiet "$sha" "$RECORD_AT" -- $BINARY_SRC; then
      moved=$(git -C "$ROOT" diff --name-only "$sha" "$RECORD_AT" -- $BINARY_SRC | tr '\n' ' ')
      fail "frozen: RECORD_AT $RECORD_AT does not hold the binary of the evidence commit $sha of $rec: $moved"
    fi
    nb=$(printf '%s\n' $bound | grep -c .)
    held="$held, ${rec#deploy/} at $sha ($nb bound files of $(basename "$drv"))"
  done
  got=$(git -C "$ROOT" diff --name-only "$BASE...$TIP" | sort)
  want=$(printf '%s\n' "$PHASE_FILES" | grep . | sort)
  if [ "$got" != "$want" ]; then
    fail "frozen: the files changed in $BASE...$TIP (TIP $TIP) are not the files of the phase; extra: $(not_in "$got" "$want")missing: $(not_in "$want" "$got")"
  fi
  [ "$RULE_FAIL" = 0 ] && ok "frozen: judged: $(judged); the bound files unchanged and $BINARY_SRC unchanged but for new version sections of CHANGELOG.md since the evidence commit of each record (${held#, }), RECORD_AT $RECORD_AT holds each binary, and $BASE...$TIP changed exactly the $(printf '%s\n' "$want" | grep -c .) files of the phase"
}

rule_quotes() {
  need "$BETA" || return 2
  local id row rec cited p n np=0 nr=0
  for id in $REQUIRED_ROWS; do
    n=$(grep -c "^| $id |" "$BETA")
    if [ "$n" != 1 ]; then fail "quotes: $id: $n rows in $BETA, want exactly one"; continue; fi
    nr=$((nr + 1))
    row=$(grep "^| $id |" "$BETA")
    rec=$(cell "$row" 3 | tr -d '` ')
    if ! is_record "$rec"; then fail "quotes: $id: its Record column names '$rec', which is not a record of 0.1.5 ($RECORDS)"; continue; fi
    if [ ! -f "$REC_ROOT/$rec" ]; then fail "quotes: $id: the record $rec is missing under $REC_ROOT"; continue; fi
    cited=$(cell "$row" 4)
    if ! quotes_of "$cited" > /dev/null; then fail "quotes: $id: the double quotes of its Cited words do not pair up"; continue; fi
    n=0
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      n=$((n + 1)); np=$((np + 1))
      grep -qF -- "$p" "$REC_ROOT/$rec" || fail "quotes: $id: not in $rec, byte for byte: $(short "$p")"
    done < <(phrases_of "$cited")
    [ "$n" -gt 0 ] || fail "quotes: $id: no cited words"
  done
  [ "$RULE_FAIL" = 0 ] && ok "quotes: $nr rows ($REQUIRED_ROWS) cite a record of 0.1.5, $np phrases found byte for byte"
}

rule_coverage() {
  need "$BETA" || return 2
  local b n rec p nq nl=0 np=0 per=""
  while IFS= read -r b; do
    nl=$((nl + 1))
    n=$(printf '%s\n' "$b" | grep -o 'Record: deploy/' | grep -c .)
    if [ "$n" != 1 ]; then fail "coverage: a limit marked (0.1.5) names $n records, want exactly one: $(short "$b")"; continue; fi
    rec=$(printf '%s\n' "$b" | sed -n 's/.*Record: \(deploy\/[A-Za-z0-9._-]*\.md\).*/\1/p')
    if ! is_record "$rec"; then fail "coverage: a limit marked (0.1.5) names '$rec', which is not a record of 0.1.5: $(short "$b")"; continue; fi
    if [ ! -f "$REC_ROOT/$rec" ]; then fail "coverage: the record $rec is missing under $REC_ROOT"; continue; fi
    if ! quotes_of "$b" > /dev/null; then fail "coverage: the double quotes of a limit marked (0.1.5) do not pair up: $(short "$b")"; continue; fi
    nq=0
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      nq=$((nq + 1)); np=$((np + 1))
      grep -qF -- "$p" "$REC_ROOT/$rec" || fail "coverage: a limit marked (0.1.5): not in $rec, byte for byte: $(short "$p")"
    done < <(quotes_of "$b")
    [ "$nq" -gt 0 ] || fail "coverage: a limit marked (0.1.5) quotes nothing of $rec: $(short "$b")"
    per="$per ${rec#deploy/}"
  done < <(grep '^- (0\.1\.5) ' "$BETA")
  [ "$nl" -gt 0 ] || fail "coverage: no limit marked (0.1.5) in $BETA"
  # shellcheck disable=SC2086
  [ "$RULE_FAIL" = 0 ] && ok "coverage: $nl limits marked (0.1.5), each naming one record of 0.1.5 ($(printf '%s\n' $per | sort | uniq -c | awk '{ printf "%s%s %s", (NR > 1 ? ", " : ""), $1, $2 }')), $np quoted sentences found byte for byte"
}

rule_ledger() {
  need "$BETA" || return 2
  local np bullets rows lim row c1 c3 p n=0 prod beta
  np=$(section2 '^## What [0-9.]+ does not promise$' < "$BETA")
  [ -n "$np" ] || { fail "ledger: no section 'What X does not promise' in $BETA"; return 1; }
  bullets=$(printf '%s\n' "$np" | grep '^- ')
  rows=$(ledger_rows < "$BETA")
  [ -n "$rows" ] || fail "ledger: no table headed '$LEDGER_HEAD' in $BETA"
  for lim in "$LIMIT_1" "$LIMIT_2" "$LIMIT_3" "$LIMIT_4"; do
    if printf '%s\n' "$bullets" | grep -qF -- "- $lim"; then fail "ledger: still a bullet of the does-not-promise section: $lim"; fi
    row=$(printf '%s\n' "$rows" | grep -F -- "| $lim" | head -n 1)
    if [ -z "$row" ]; then fail "ledger: not quoted in the first column of the 0.1.5 ledger: $lim"; continue; fi
    c1=$(cell "$row" 1)
    case "$c1" in "$lim"*) ;; *) fail "ledger: the first cell does not begin with the limit: $lim" ;; esac
    c3=$(cell "$row" 3)
    printf '%s' "$c3" | grep -qE 'P-[0-9]+' || fail "ledger: the row of '$lim' names no promise row"
    for p in $(printf '%s' "$c3" | grep -oE 'P-[0-9]+'); do
      grep -q "^| $p |" "$BETA" || fail "ledger: the row of '$lim' names $p, which is not a promise row"
    done
    n=$((n + 1))
  done
  if printf '%s\n' "$bullets" | grep -qF -- "$SCHEMA_OLD"; then fail "ledger: the bullet of 0.1.1 on an older binary and a migrated database is still there"; fi
  prod=$(printf '%s\n' "$np" | section3 '^### The product itself$')
  [ -n "$prod" ] || fail "ledger: no section 'The product itself' under the does-not-promise section"
  printf '%s\n' "$prod" | grep -qF 'deploy/ROLLBACK-RUN.md' || fail "ledger: 'The product itself' does not name deploy/ROLLBACK-RUN.md"
  beta=$(section2 '^## What beta means here$' < "$BETA")
  printf '%s\n' "$beta" | grep -qE '(^|[^0-9-])P-30([^0-9]|$)' || fail "ledger: 'What beta means here' does not name P-30"
  [ "$RULE_FAIL" = 0 ] && ok "ledger: $n limits quoted in the 0.1.5 ledger and gone from the bullets, the schema bullet of 0.1.1 rewritten and naming deploy/ROLLBACK-RUN.md"
}

rule_nojs() {
  local got want
  got=$(git -C "$ROOT" ls-files -- '*.js' '*.mjs' '*.cjs') || { fail "nojs: git ls-files failed in $ROOT"; return 2; }
  # shellcheck disable=SC2086
  want=$(printf '%s\n' $JS_ALLOWED | grep . | sort)
  got=$(printf '%s\n' "$got" | grep . | sort)
  if [ "$got" != "$want" ]; then
    # shellcheck disable=SC2086
    fail "nojs: the tracked JavaScript is not exactly '$JS_ALLOWED': $(printf '%s ' $got)"
    return 1
  fi
  ok "nojs: the tracked JavaScript is exactly $JS_ALLOWED"
}

rule_stable() {
  need "$STABLE" || return 2
  need "$README" || return 2
  local want got h area sec prom miss items item n nitems=0 narea=0 before paths p np=0 out
  [ "$(head -n 1 "$STABLE")" = "$STABLE_TITLE" ] || fail "stable: the first line of $STABLE is not '$STABLE_TITLE'"
  want=$(printf '%s\n%s\n' "$STABLE_AREAS" "$STABLE_REST" | grep .)
  got=$(awk '/^```/ { f = !f; next } !f && /^## / { sub(/^## /, ""); print }' "$STABLE")
  before=$RULE_FAIL
  while IFS= read -r h; do
    [ -n "$h" ] || continue
    printf '%s\n' "$got" | grep -qxF -- "$h" || fail "stable: the heading '## $h' is missing from $STABLE"
  done <<EOF_W
$want
EOF_W
  if [ "$RULE_FAIL" = "$before" ] && [ "$got" != "$want" ]; then
    fail "stable: the level-two headings are not exactly these, in this order: $(printf '%s' "$want" | tr '\n' '|')"
  fi
  while IFS= read -r area; do
    [ -n "$area" ] || continue
    sec=$(section2 "^## $area\$" < "$STABLE")
    if [ -z "$sec" ]; then continue; fi
    narea=$((narea + 1))
    n=$(printf '%s\n' "$sec" | grep -c '^### What 1\.0 promises$')
    [ "$n" = 1 ] || { fail "stable: '## $area' has $n headings '### What 1.0 promises', want exactly one"; continue; }
    n=$(printf '%s\n' "$sec" | grep -c '^### What is still missing$')
    [ "$n" = 1 ] || { fail "stable: '## $area' has $n headings '### What is still missing', want exactly one"; continue; }
    if [ "$(printf '%s\n' "$sec" | grep -n '^### What 1\.0 promises$' | cut -d: -f1)" -gt "$(printf '%s\n' "$sec" | grep -n '^### What is still missing$' | cut -d: -f1)" ]; then
      fail "stable: '## $area' puts '### What is still missing' before '### What 1.0 promises'"
      continue
    fi
    prom=$(printf '%s\n' "$sec" | section3 '^### What 1\.0 promises$' | grep -c .)
    [ "$prom" -gt 0 ] || fail "stable: '## $area': nothing is written under '### What 1.0 promises'"
    miss=$(printf '%s\n' "$sec" | section3 '^### What is still missing$' | grep .)
    if printf '%s\n' "$miss" | grep -qE '^- \[[xX]\]'; then fail "stable: '## $area' has a checked item; what is missing is not done until a record shows it"; fi
    items=$(printf '%s\n' "$miss" | grep '^- \[ \] ')
    if [ -z "$items" ]; then fail "stable: '## $area' has no '- [ ] ' item under '### What is still missing'"; continue; fi
    out=$(printf '%s\n' "$miss" | grep -v '^- \[ \] ')
    [ -z "$out" ] || fail "stable: '## $area': a line under '### What is still missing' is not a '- [ ] ' item: $(short "$out")"
    while IFS= read -r item; do
      [ -n "$item" ] || continue
      nitems=$((nitems + 1))
      printf '%s' "$item" | grep -qE '(^|[^A-Za-z0-9-])[A-Z]{2,4}-[0-9]{2}([^0-9]|$)|(^|[^A-Za-z0-9-])OS-[0-9]+([^0-9]|$)|`[A-Za-z0-9._-]+/[^` ]*`' \
        || fail "stable: '## $area': an item names no requirement, owner step or path: $(short "$item")"
    done <<EOF_I
$items
EOF_I
  done <<EOF_A
$STABLE_AREAS
EOF_A
  case "$STABLE_REST" in
    *'Outside the 1.0 promise'*)
      sec=$(section2 '^## Outside the 1\.0 promise$' < "$STABLE")
      for h in 'plugin ABI' 'SDK' 'MCP'; do
        printf '%s\n' "$sec" | grep -qF -- "$h" || fail "stable: '## Outside the 1.0 promise' does not name '$h'"
      done
      ;;
  esac
  paths=$(grep -oE '(deploy|docs)/[A-Za-z0-9._/-]*\.md' "$STABLE" | sort -u)
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    np=$((np + 1))
    [ -f "$ROOT/$p" ] || fail "stable: the page names $p, which is not a file of $ROOT"
  done <<EOF_P
$paths
EOF_P
  grep -qF '](docs/stable.md)' "$README" || fail "stable: $README does not link docs/stable.md"
  [ "$RULE_FAIL" = 0 ] && ok "stable: $narea areas with a promise and a checklist, $nitems unchecked items that each name a source, $np documents named that exist, and the README links docs/stable.md"
}

# run_ok DESC PREFIX CMD... : run CMD in ROOT (in RUN_DIR when the call sets it); its last line must begin with PREFIX. With
# RUN_NOSKIP=1 in the environment of the call, a last line that says a rule was skipped
# fails too, because a driver that skipped its own record rule has proven less than asked.
run_ok() {
  local desc="$1" want="$2" out last
  shift 2
  out=$(cd "${RUN_DIR:-$ROOT}" && "$@" 2>&1)
  last=$(printf '%s\n' "$out" | tail -n 1)
  case "$last" in
    "$want"*)
      if [ "${RUN_NOSKIP:-0}" = 1 ]; then
        case "$last" in
          *" skipped)"*)
            printf '%s\n' "$out" | tail -n 5
            fail "record: $desc skipped a rule: $last"
            return
            ;;
        esac
      fi
      echo "ok: record: $desc: $last"
      ;;
    *)
      printf '%s\n' "$out" | tail -n 5
      fail "record: $desc did not end '$want…'"
      ;;
  esac
}

# record_prove : what must hold before the drivers' record-checks run at RECORD_AT, because
# the drivers there are the code of HEAD and RECORD_AT is HEAD but for new changelog sections.
record_prove() {
  local reason drv bound moved
  at_ok record || return 2
  git -C "$ROOT" rev-parse -q --verify "$RECORD_AT^{commit}" > /dev/null || { fail "record: RECORD_AT '$RECORD_AT' is not a commit of $ROOT"; return 2; }
  git -C "$ROOT" merge-base --is-ancestor "$RECORD_AT" "$(at_ref)" || { fail "record: RECORD_AT $RECORD_AT is not an ancestor of $(at_ref)"; return 1; }
  if ! reason=$(src_since "$RECORD_AT"); then
    fail "record: the source of the binary moved since RECORD_AT $RECORD_AT (judged: $(judged)): $reason"
    return 1
  fi
  for drv in deploy/test/upgrade-verify.sh deploy/test/browser-verify.sh; do
    bound=$(bound_of "$drv") || { fail "record: $drv has no single BOUND_FILES= line"; return 1; }
    # shellcheck disable=SC2086
    moved=$(moved_since "$RECORD_AT" $bound | tr '\n' ' ')
    if [ -n "$moved" ]; then
      fail "record: a bound file of $drv moved since RECORD_AT $RECORD_AT (judged: $(judged)): $moved"
      return 1
    fi
  done
  return 0
}

rule_record() {
  local missing="" rec has_sum=no has_uat=no rc
  [ -f "$E/amd64-t34-summary.txt" ] && has_sum=yes
  [ -d "$E/mac-t34-uat" ] && has_uat=yes
  if [ "$has_sum" = yes ] && [ "$has_uat" = yes ]; then
    record_prove
    rc=$?
    [ "$rc" = 2 ] && return 2
    if [ "$rc" = 0 ]; then
      WT="$TMPROOT/at"
      if ! git -C "$ROOT" -c core.hooksPath=/dev/null worktree add --detach -q "$WT" "$RECORD_AT" > /dev/null 2>&1; then
        WT=""
        fail "record: git worktree add of RECORD_AT $RECORD_AT failed"
        return 2
      fi
      for rec in deploy/MIGRATION-RESTORE-RUN.md deploy/UPDATE-CHAIN-RUN.md deploy/ROLLBACK-RUN.md; do
        RUN_DIR="$WT" run_ok "upgrade-verify.sh record-check of $rec" 'RECORD-CHECK OK (' env R="$REC_ROOT/$rec" E="$E" bash deploy/test/upgrade-verify.sh record-check
      done
      RUN_DIR="$WT" run_ok "browser-verify.sh record-check of deploy/BROWSER-RUN.md" 'RECORD-CHECK OK (' env R="$REC_ROOT/deploy/BROWSER-RUN.md" E="$E" bash deploy/test/browser-verify.sh record-check
      git -C "$ROOT" worktree remove --force "$WT" > /dev/null 2>&1
      WT=""
    fi
  elif [ "$has_sum" = yes ] || [ "$has_uat" = yes ]; then
    # Evidence that is partly there is not evidence that is missing: the half that is
    # there would go unchecked, and the gate would say nothing.
    fail "record: $E holds only part of the evidence set (amd64-t34-summary.txt $([ "$has_sum" = yes ] && echo present || echo missing), mac-t34-uat $([ "$has_uat" = yes ] && echo present || echo missing))"
  else
    missing="$missing E ($E holds no amd64-t34-summary.txt and mac-t34-uat)"
  fi
  if [ -d "$E32/amd64-t32-src" ]; then
    RUN_NOSKIP=1 run_ok "32-check.sh all after the merge of phase 32" '32-CHECK OK (' env AT="$AT" BASE=e1116b3 TIP=93bfb80 E="$E32" bash deploy/test/32-check.sh all
  elif ls "$E32"/amd64-t32-* > /dev/null 2>&1; then
    fail "record: $E32 holds only part of the evidence set of phase 32 (amd64-t32-src missing)"
  else
    missing="$missing E32 ($E32 holds no amd64-t32-src)"
  fi
  [ "$RULE_FAIL" = 0 ] || return 1
  if [ -n "$missing" ]; then
    echo "SKIP record: no scratch files under$missing"
    return "$SKIP"
  fi
  ok "record: the record-checks of the four records of phase 34 ran at RECORD_AT $RECORD_AT, which the judged tree ($(judged)) differs from at most by new version sections of CHANGELOG.md, and the gate of phase 32 ended OK"
}

# ------------------------------------------------------------- selftest ----

ST_N=0
ST_K=0
st_pass() { ST_N=$((ST_N + 1)); echo "ok $ST_N $1"; }
st_bad() { ST_N=$((ST_N + 1)); ST_K=$((ST_K + 1)); echo "FAIL $ST_N $1"; }
# st_run DESC WANT-EXIT MESSAGE NAME=VALUE... -- RULE : run this file on one rule with the
# environment given, each NAME=VALUE one argument (a value may hold spaces and newlines);
# the exit status must be WANT-EXIT and the output must hold MESSAGE.
st_run() {
  local desc="$1" want="$2" msg="$3" out rc
  local -a envs=()
  shift 3
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs[${#envs[@]}]="$1"; shift; done
  shift
  # AT= first: a value the caller of the selftest set must not reach a case that did not ask
  # for it; a case that names AT comes later in the list and wins.
  out=$(env AT= ${envs[@]+"${envs[@]}"} bash "$ME" "$@" 2>&1)
  rc=$?
  if [ "$rc" != "$want" ]; then st_bad "$desc: exit $rc, want $want; last line: $(printf '%s\n' "$out" | tail -n 1)"; return; fi
  if [ -n "$msg" ] && ! printf '%s\n' "$out" | grep -qF -- "$msg"; then st_bad "$desc: the output does not say '$msg'"; return; fi
  st_pass "$desc"
}
# t_row_append ID TOKEN < FILE : append a backticked token to the Cited words of row ID.
t_row_append() {
  ID="$1" TOK="$2" awk 'BEGIN { id = "| " ENVIRON["ID"] " |"; t = ENVIRON["TOK"] }
    index($0, id) == 1 { sub(/ \|$/, ", `" t "` |") } { print }'
}
# t_row_drop ID < FILE : the file without the row ID.
t_row_drop() { ID="$1" awk 'BEGIN { id = "| " ENVIRON["ID"] " |" } index($0, id) != 1 { print }'; }
# t_after LINE NEW < FILE : NEW inserted after the line that begins with LINE.
t_after() { O="$1" N="$2" awk 'BEGIN { o = ENVIRON["O"]; n = ENVIRON["N"] } { print } index($0, o) == 1 { print n }'; }
# t_row_sub ID OLD NEW < FILE : in row ID, the first OLD replaced by NEW.
t_row_sub() {
  ID="$1" O="$2" N="$3" awk 'BEGIN { id = "| " ENVIRON["ID"] " |"; o = ENVIRON["O"]; n = ENVIRON["N"]; ol = length(o) }
    index($0, id) == 1 { i = index($0, o); if (i > 0) $0 = substr($0, 1, i - 1) n substr($0, i + ol) } { print }'
}

# t_commit DIR : commit every tracked change in DIR, as upgrade-verify.sh does.
t_commit() {
  git -C "$1" -c user.name=t34 -c user.email=t34@invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -am selftest
}
# t_cl_top VERSION DATE < FILE : a new section, directly above the first level-two heading.
t_cl_top() {
  V="$1" D="$2" awk 'BEGIN { v = ENVIRON["V"]; d = ENVIRON["D"] }
    !done && /^## / { printf "## %s — %s\n\n### Neu\n\n- Ein Fall des Selbsttests.\n\n", v, d; done = 1 } { print }'
}
# t_cl_case DIR VERSION DATE [EDIT] : in the clone DIR, reset to HEADSHA, add the section
# (when VERSION is not empty), then apply EDIT, a function that filters CHANGELOG.md from
# stdin to stdout. The caller commits.
t_cl_case() {
  local dir="$1" ver="$2" dat="$3" edit="${4:-}"
  git -C "$dir" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA" || return 1
  if [ -n "$ver" ]; then
    t_cl_top "$ver" "$dat" < "$dir/CHANGELOG.md" > "$dir/CHANGELOG.md.new" && mv "$dir/CHANGELOG.md.new" "$dir/CHANGELOG.md"
  fi
  if [ -n "$edit" ]; then
    "$edit" < "$dir/CHANGELOG.md" > "$dir/CHANGELOG.md.new" && mv "$dir/CHANGELOG.md.new" "$dir/CHANGELOG.md"
  fi
}
t_e_date() { sed 's/^## 0\.1\.4 — 2026-11-12$/## 0.1.4 — 2026-11-13/'; }
t_e_last() { sed '$d'; }
t_e_end() { cat; printf '\n## 9.9.8 — 2099-01-01\n\n### Neu\n\n- Ein Fall des Selbsttests.\n'; }
t_e_nohead() { awk '!done && /^## / { print "- Ein Satz ohne Version."; print ""; done = 1 } { print }'; }
t_e_pre() { sed '1s/^# Changelog$/# Changes/'; }

selftest() {
  local T base tip rec less wt0 c1 c2 tipsha side at atarg
  T=$(mktemp -d "$TMPROOT/st.XXXXXX")
  wt0=$(git -C "$ROOT" worktree list | grep -c .)
  cp "$BETA" "$T/good.md"
  # Merged into main, BASE...HEAD is every later change too, so the frozen control
  # failed on every checkout after the merge of phase 34 (as WR-02 found for
  # 32-check.sh). Without BASE the selftest pins the pair that bounds phase 34,
  # cfce101 and its merge af52d7b, when both are here; otherwise it falls back to
  # the merge base as before.
  if [ -z "${BASE:-}" ] \
    && git -C "$ROOT" rev-parse -q --verify "cfce101^{commit}" > /dev/null 2>&1 \
    && git -C "$ROOT" rev-parse -q --verify "af52d7b^{commit}" > /dev/null 2>&1; then
    # TIP is never empty here (the header defaults it to HEAD), so without BASE
    # both bounds are pinned together.
    base=cfce101
    tip=af52d7b
  else
    base=${BASE:-$(git -C "$ROOT" merge-base origin/main HEAD 2> /dev/null)}
    tip=${TIP:-HEAD}
  fi
  base=$(git -C "$ROOT" rev-parse -q --verify "$base^{commit}" 2> /dev/null)
  # The cases on ROOT judge the release whose records this gate proves, not the live HEAD,
  # which moved on after it (the frozen control failed on every checkout after pull request
  # 101). AT given by the caller is kept; otherwise the tag v0.1.5 when this clone has it,
  # and the working tree when it has not.
  at=$AT
  if [ -z "$at" ] && git -C "$ROOT" rev-parse -q --verify 'v0.1.5^{commit}' > /dev/null 2>&1; then at=v0.1.5; fi
  atarg="AT=$at"
  # frozen
  st_run 'frozen: control, the records as they are' 0 'ok: frozen' "BASE=$base" "TIP=$tip" "$atarg" -- frozen
  mkdir -p "$T/rec/deploy"
  for rec in $RECORDS; do cp "$REC_ROOT/$rec" "$T/rec/$rec"; done
  sed "s/^Evidence commit: .*/Evidence commit: $base/" "$REC_ROOT/deploy/MIGRATION-RESTORE-RUN.md" > "$T/rec/deploy/MIGRATION-RESTORE-RUN.md"
  st_run 'frozen: the Evidence commit of a record replaced by BASE' 1 'frozen: deploy/MIGRATION-RESTORE-RUN.md: a bound file moved since the evidence commit' "BASE=$base" "TIP=$tip" "REC_ROOT=$T/rec" "$atarg" -- frozen
  less=$(printf '%s\n' "$PHASE_FILES" | grep -vxF 'docs/beta.md')
  st_run 'frozen: the list of the phase without docs/beta.md' 1 'extra: docs/beta.md' "BASE=$base" "TIP=$tip" "PHASE_FILES=$less" "$atarg" -- frozen
  st_run 'frozen: no BASE' 2 'frozen: BASE is required' "BASE=" "TIP=$tip" "$atarg" -- frozen
  st_run 'frozen: RECORD_AT is not a commit' 2 "frozen: RECORD_AT '0000000' is not a commit" "BASE=$base" "TIP=$tip" "RECORD_AT=0000000" "$atarg" -- frozen
  st_run 'frozen: RECORD_AT is BASE, whose binary is not that of the evidence' 1 'does not hold the binary of the evidence commit' "BASE=$base" "TIP=$tip" "RECORD_AT=$base" "$atarg" -- frozen
  # A scratch clone of ROOT, with one commit on top of HEAD per case. TIP is resolved here:
  # inside the clone, HEAD is the case's commit and would make BASE...TIP list the case.
  HEADSHA=$(git -C "$ROOT" rev-parse "${at:-HEAD}^{commit}")
  tipsha=$(git -C "$ROOT" rev-parse -q --verify "$tip^{commit}")
  git -c core.hooksPath=/dev/null clone -q --shared --no-checkout "$ROOT" "$T/clone" 2> /dev/null
  t_cl_case "$T/clone" 9.9.9 2099-01-01
  t_commit "$T/clone"
  st_run 'frozen: a commit that adds a new top section to CHANGELOG.md passes' 0 'ok: frozen' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" 9.9.9 2099-01-01
  printf '// selftest\n' >> "$T/clone/changelog.go"
  t_commit "$T/clone"
  st_run 'frozen: a commit that also changes changelog.go fails' 1 'beyond CHANGELOG.md: changelog.go' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" 9.9.9 2099-01-01 t_e_date
  t_commit "$T/clone"
  st_run 'frozen: a commit that adds a section and edits an older line of CHANGELOG.md fails' 1 'an older line changed, moved or was removed' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" 9.9.9 2099-01-01 t_e_last
  t_commit "$T/clone"
  st_run 'frozen: a commit that adds a section and removes an older line of CHANGELOG.md fails' 1 'an older line changed, moved or was removed' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" "" "" t_e_end
  t_commit "$T/clone"
  st_run 'frozen: a commit that adds the section at the end of CHANGELOG.md fails' 1 'an older line changed, moved or was removed' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" "" "" t_e_nohead
  t_commit "$T/clone"
  st_run 'frozen: a commit whose added lines do not begin with a version heading fails' 1 'the added lines do not begin with a version heading' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" 9.9.9 2099-01-01 t_e_pre
  t_commit "$T/clone"
  st_run 'frozen: a commit that changes the preamble of CHANGELOG.md fails' 1 'a line above its first version heading changed' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" 0.1.4 2026-11-24
  t_commit "$T/clone"
  st_run 'frozen: a commit that adds a second section for an existing version fails' 1 'names a version an older heading already has' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  t_cl_case "$T/clone" "" ""
  printf 'selftest\n' >> "$T/clone/README.md"
  t_commit "$T/clone"
  side=$(git -C "$T/clone" rev-parse HEAD)
  git -C "$T/clone" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA"
  st_run 'frozen: RECORD_AT on a side commit that is not an ancestor of HEAD fails' 1 'is not an ancestor of HEAD' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" "RECORD_AT=$side" -- frozen
  # Without AT the gate judges the working tree. The clone is checked out clean at the pin,
  # so the control passes; an untracked file under the source of the binary and an edit of a
  # bound file that is not committed each make it fail, and with AT=HEAD the same edit is
  # invisible, because AT names a commit and the edit is in none.
  git -C "$T/clone" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA"
  st_run 'frozen: AT unset, the clone checked out clean at the pin: the working tree passes' 0 'ok: frozen' "ROOT=$T/clone" "BASE=$base" "TIP=$tipsha" -- frozen
  printf 'package selftest\n' > "$T/clone/internal/selftest_untracked.go"
  st_run 'frozen: AT unset, an untracked file under the source of the binary fails' 1 '(judged: the working tree): beyond CHANGELOG.md: internal/selftest_untracked.go' "ROOT=$T/clone" "BASE=$base" "TIP=$tipsha" -- frozen
  rm -f "$T/clone/internal/selftest_untracked.go"
  printf '# selftest\n' >> "$T/clone/deploy/test/run.sh"
  st_run 'frozen: AT unset, an uncommitted edit of a bound file fails and names it' 1 '(judged: the working tree): deploy/test/run.sh' "ROOT=$T/clone" "BASE=$base" "TIP=$tipsha" -- frozen
  st_run 'frozen: AT=HEAD does not see the uncommitted edit of a bound file' 0 'ok: frozen' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
  git -C "$T/clone" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA"
  st_run 'frozen: AT is not a commit' 2 "frozen: AT 'no-such-ref' is not a commit" "BASE=$base" "TIP=$tip" "AT=no-such-ref" -- frozen
  # quotes
  st_run 'quotes: control, the page as it is' 0 'ok: quotes' "BETA=$T/good.md" -- quotes
  t_row_append P-27 'NO SUCH PHRASE 34' < "$T/good.md" > "$T/q-absent.md"
  st_run 'quotes: P-27 cites a phrase its record does not hold' 1 'P-27: not in deploy/MIGRATION-RESTORE-RUN.md, byte for byte: NO SUCH PHRASE 34' "BETA=$T/q-absent.md" -- quotes
  t_row_sub P-27 '`deploy/MIGRATION-RESTORE-RUN.md`' '`deploy/BACKUP-RUN.md`' < "$T/good.md" > "$T/q-other.md"
  st_run 'quotes: P-27 names a record that is not a record of 0.1.5' 1 "P-27: its Record column names 'deploy/BACKUP-RUN.md', which is not a record of 0.1.5" "BETA=$T/q-other.md" -- quotes
  t_row_drop P-27 < "$T/good.md" > "$T/q-gone.md"
  st_run 'quotes: P-27 is gone' 1 'P-27: 0 rows' "BETA=$T/q-gone.md" -- quotes
  t_row_append E-11 'Evidence commit: 0000000' < "$T/good.md" > "$T/q-e11.md"
  st_run 'quotes: E-11 cites an evidence commit its record does not name' 1 'E-11: not in deploy/ROLLBACK-RUN.md, byte for byte: Evidence commit: 0000000' "BETA=$T/q-e11.md" -- quotes
  # coverage
  st_run 'coverage: control, the page as it is' 0 'ok: coverage' "BETA=$T/good.md" -- coverage
  t_after '### The unit and the platform' '- (0.1.5) A limit that rests on nothing. Why: a case of the selftest.' < "$T/good.md" > "$T/c-none.md"
  st_run 'coverage: a limit marked (0.1.5) names no record' 1 'coverage: a limit marked (0.1.5) names 0 records, want exactly one' "BETA=$T/c-none.md" -- coverage
  t_after '### The unit and the platform' '- (0.1.5) A limit of the rollback. Why: a case of the selftest. Record: deploy/ROLLBACK-RUN.md, *Not tested*: "NO SUCH PHRASE 34 in deploy/ROLLBACK-RUN.md."' < "$T/good.md" > "$T/c-absent.md"
  st_run 'coverage: a limit marked (0.1.5) quotes what its record does not say' 1 'coverage: a limit marked (0.1.5): not in deploy/ROLLBACK-RUN.md, byte for byte: NO SUCH PHRASE 34' "BETA=$T/c-absent.md" -- coverage
  t_after '### The unit and the platform' '- (0.1.5) A limit of an older record. Why: a case of the selftest. Record: deploy/BACKUP-RUN.md, *Not tested*: "BAK-01 PASS"' < "$T/good.md" > "$T/c-old.md"
  st_run 'coverage: a limit marked (0.1.5) names a record that is not of 0.1.5' 1 "coverage: a limit marked (0.1.5) names 'deploy/BACKUP-RUN.md', which is not a record of 0.1.5" "BETA=$T/c-old.md" -- coverage
  # ledger
  st_run 'ledger: control, the page as it is' 0 'ok: ledger' "BETA=$T/good.md" -- ledger
  t_after '### Install, update and rollback' "- $LIMIT_3: the update migrated cleanly. Why: kept." < "$T/good.md" > "$T/l-bullet.md"
  st_run 'ledger: a limit left as a bullet' 1 "ledger: still a bullet of the does-not-promise section: $LIMIT_3" "BETA=$T/l-bullet.md" -- ledger
  t_after '### The product itself' "$SCHEMA_OLD Why: kept." < "$T/good.md" > "$T/l-schema.md"
  st_run 'ledger: the schema bullet of 0.1.1 still there' 1 'ledger: the bullet of 0.1.1 on an older binary and a migrated database is still there' "BETA=$T/l-schema.md" -- ledger
  # nojs
  st_run 'nojs: control, the repository as it is' 0 'ok: nojs' -- nojs
  st_run 'nojs: no JavaScript allowed, so the htmx file is one too many' 1 'nojs: the tracked JavaScript is not exactly' "JS_ALLOWED=" -- nojs
  # stable
  cp "$STABLE" "$T/stable-good.md" 2> /dev/null || : > "$T/stable-good.md"
  st_run 'stable: control, the page and the README as they are' 0 'ok: stable' "STABLE=$T/stable-good.md" -- stable
  grep -vxF '## Template contract' "$T/stable-good.md" > "$T/s-noheading.md"
  st_run 'stable: a copy without the heading ## Template contract' 1 "stable: the heading '## Template contract' is missing" "STABLE=$T/s-noheading.md" -- stable
  grep -v '^- \[ \] ' "$T/stable-good.md" > "$T/s-noitem.md"
  st_run 'stable: a copy in which no area has a - [ ] item' 1 "stable: '## Template contract' has no '- [ ] ' item" "STABLE=$T/s-noitem.md" -- stable
  awk '!done && /^- \[ \] / { sub(/^- \[ \] /, "- [x] "); done = 1 } { print }' "$T/stable-good.md" > "$T/s-checked.md"
  st_run 'stable: a copy with a - [x] item' 1 "stable: '## Template contract' has a checked item" "STABLE=$T/s-checked.md" -- stable
  t_after '### What is still missing' '- [ ] Something is missing and says nothing about what it waits on.' < "$T/stable-good.md" > "$T/s-nosource.md"
  st_run 'stable: an item that names no requirement, owner step or path' 1 "stable: '## Template contract': an item names no requirement, owner step or path: - [ ] Something is missing and says nothing" "STABLE=$T/s-nosource.md" -- stable
  sed 's#docs/beta\.md#docs/nosuch.md#g' "$T/stable-good.md" > "$T/s-nopath.md"
  st_run 'stable: a copy that names a document that does not exist' 1 'stable: the page names docs/nosuch.md, which is not a file of' "STABLE=$T/s-nopath.md" -- stable
  sed 's#^\# What 1\.0 will promise, and what it still lacks$#\# Stable#' "$T/stable-good.md" > "$T/s-title.md"
  st_run 'stable: a copy with another title' 1 "stable: the first line of $T/s-title.md is not" "STABLE=$T/s-title.md" -- stable
  grep -vxF '## Migrations' "$T/stable-good.md" > "$T/s-nomig.md"
  st_run 'stable: a copy without the heading ## Migrations' 1 "stable: the heading '## Migrations' is missing" "STABLE=$T/s-nomig.md" -- stable
  sed 's/MCP/mcp tools/g' "$T/stable-good.md" > "$T/s-nomcp.md"
  st_run 'stable: a copy whose outside section does not name MCP' 1 "stable: '## Outside the 1.0 promise' does not name 'MCP'" "STABLE=$T/s-nomcp.md" -- stable
  awk '/^## Not yet decided$/ { print "## Decided"; next } { print }' "$T/stable-good.md" > "$T/s-order.md"
  st_run 'stable: a copy whose last heading is another' 1 "stable: the heading '## Not yet decided' is missing" "STABLE=$T/s-order.md" -- stable
  awk '/^## Data contract$/ { hold = 1 } /^## Migrations$/ { hold = 0 } !hold { print }' "$T/stable-good.md" > "$T/s-noarea.md"
  st_run 'stable: a copy without the data contract area' 1 "stable: the heading '## Data contract' is missing" "STABLE=$T/s-noarea.md" -- stable
  grep -vF 'docs/stable.md' "$README" > "$T/readme-nolink.md"
  st_run 'stable: a README without the link' 1 'does not link docs/stable.md' "README=$T/readme-nolink.md" -- stable
  # record
  mkdir -p "$T/empty-e" "$T/bad-e/mac-t34-uat"
  : > "$T/bad-e/amd64-t34-summary.txt"
  st_run 'record: E and E32 are empty directories, so the rule is skipped and not passed' "$SKIP" 'SKIP record: no scratch files under' "E=$T/empty-e" "E32=$T/empty-e" -- record
  st_run 'record: a run in which every rule was skipped proves nothing and exits 77' "$SKIP" '34-CHECK NOT PROVEN (0 rules, 1 skipped)' "E=$T/empty-e" "E32=$T/empty-e" -- record
  st_run 'record: a skip beside a rule that passed names the skipped rule and still exits 0' 0 'NOT PROVEN: record' "E=$T/empty-e" "E32=$T/empty-e" -- nojs record
  st_run 'record: a skip beside a rule that passed keeps the count in the last line' 0 '34-CHECK OK (1 rules, 1 skipped)' "E=$T/empty-e" "E32=$T/empty-e" -- nojs record
  st_run 'record: E holds an empty set, so the record-check fails' 1 'record: upgrade-verify.sh record-check of deploy/MIGRATION-RESTORE-RUN.md did not end' "E=$T/bad-e" "E32=$T/empty-e" "$atarg" -- record
  # A half-present set is evidence that is partly there, and the rule refuses it.
  mkdir -p "$T/half-summary" "$T/half-uat/mac-t34-uat" "$T/half-t32"
  : > "$T/half-summary/amd64-t34-summary.txt"
  : > "$T/half-t32/amd64-t32-pass1.log"
  st_run 'record: E holds amd64-t34-summary.txt and no mac-t34-uat, so the rule fails and is not skipped' 1 'record: ' "E=$T/half-summary" "E32=$T/empty-e" -- record
  st_run 'record: the message names what is missing from E (the browser set)' 1 'holds only part of the evidence set (amd64-t34-summary.txt present, mac-t34-uat missing)' "E=$T/half-summary" "E32=$T/empty-e" -- record
  st_run 'record: E holds mac-t34-uat and no amd64-t34-summary.txt, so the rule fails and is not skipped' 1 'holds only part of the evidence set (amd64-t34-summary.txt missing, mac-t34-uat present)' "E=$T/half-uat" "E32=$T/empty-e" -- record
  st_run 'record: E32 holds files of phase 32 and no amd64-t32-src, so the rule fails and is not skipped' 1 'holds only part of the evidence set of phase 32 (amd64-t32-src missing)' "E=$T/empty-e" "E32=$T/half-t32" -- record
  # The coupling to the gate of phase 32 reads its last line, not its rule count. The fake
  # root is a git repository of two commits: c1 is RECORD_AT, c2 is HEAD and adds a section
  # to CHANGELOG.md. Its drivers end OK only where HEAD is FAKE_AT, which is how the case
  # proves that the record-checks run at RECORD_AT and not at HEAD. Its 32-check.sh ends as
  # FAKE_LAST says.
  mkdir -p "$T/fake/deploy/test" "$T/fake/internal" "$T/full-e/mac-t34-uat" "$T/full-e32/amd64-t32-src"
  : > "$T/full-e/amd64-t34-summary.txt"
  git init -q "$T/fake" 2> /dev/null
  for rec in upgrade-verify browser-verify; do
    printf '%s\n' '#!/bin/sh' "BOUND_FILES=\"deploy/test/$rec.sh\"" 'if [ "$(git rev-parse HEAD)" = "$FAKE_AT" ]; then echo "RECORD-CHECK OK (1 checks)"; else echo "RECORD-CHECK FAILED (HEAD is not FAKE_AT)"; fi' > "$T/fake/deploy/test/$rec.sh"
  done
  printf '%s\n' '#!/bin/sh' 'echo "$FAKE_LAST"' > "$T/fake/deploy/test/32-check.sh"
  printf '%s\n' '# Changelog' '' '## 0.0.1 — 2026-01-01' '' '- Ein Punkt.' > "$T/fake/CHANGELOG.md"
  printf '%s\n' 'package x' > "$T/fake/internal/x.go"
  git -C "$T/fake" add deploy CHANGELOG.md internal
  t_commit "$T/fake"
  c1=$(git -C "$T/fake" rev-parse HEAD)
  t_cl_top 0.0.2 2026-01-02 < "$T/fake/CHANGELOG.md" > "$T/fake/CHANGELOG.md.new" && mv "$T/fake/CHANGELOG.md.new" "$T/fake/CHANGELOG.md"
  t_commit "$T/fake"
  c2=$(git -C "$T/fake" rev-parse HEAD)
  st_run 'record: the gate of phase 32 ends with another number of rules, and the rule passes' 0 '34-CHECK OK (1 rules)' "ROOT=$T/fake" "E=$T/full-e" "E32=$T/full-e32" "FAKE_LAST=32-CHECK OK (4 rules)" "RECORD_AT=$c1" "FAKE_AT=$c1" -- record
  st_run 'record: the gate of phase 32 skipped its own record rule, and the rule fails' 1 "record: 32-check.sh all after the merge of phase 32 skipped a rule" "ROOT=$T/fake" "E=$T/full-e" "E32=$T/full-e32" "FAKE_LAST=32-CHECK OK (4 rules, 1 skipped)" "RECORD_AT=$c1" "FAKE_AT=$c1" -- record
  st_run 'record: the gate of phase 32 fails, and the rule fails' 1 "record: 32-check.sh all after the merge of phase 32 did not end '32-CHECK OK (…'" "ROOT=$T/fake" "E=$T/full-e" "E32=$T/full-e32" "FAKE_LAST=32-CHECK FAILED (1 of 5)" "RECORD_AT=$c1" "FAKE_AT=$c1" -- record
  st_run 'record: with RECORD_AT at HEAD the fake drivers see HEAD and fail' 1 'record: upgrade-verify.sh record-check of deploy/MIGRATION-RESTORE-RUN.md did not end' "ROOT=$T/fake" "E=$T/full-e" "E32=$T/full-e32" "FAKE_LAST=32-CHECK OK (5 rules)" "RECORD_AT=$c2" "FAKE_AT=$c1" -- record
  printf '%s\n' 'package x // moved' > "$T/fake/internal/x.go"
  t_commit "$T/fake"
  st_run 'record: the source of the binary moved since RECORD_AT beyond CHANGELOG.md, and the rule fails' 1 'beyond CHANGELOG.md: internal/x.go' "ROOT=$T/fake" "E=$T/full-e" "E32=$T/full-e32" "FAKE_LAST=32-CHECK OK (5 rules)" "RECORD_AT=$c1" "FAKE_AT=$c1" -- record
  if [ "$(git -C "$T/fake" worktree list | grep -c .)" = 1 ]; then st_pass 'record: no worktree is left behind in the fake root'; else st_bad 'record: no worktree is left behind in the fake root'; fi
  if [ "$(git -C "$ROOT" worktree list | grep -c .)" = "$wt0" ]; then st_pass 'record: no worktree of ROOT is left behind'; else st_bad 'record: no worktree of ROOT is left behind'; fi
  if [ "$ST_K" = 0 ]; then
    echo "SELFTEST OK ($ST_N cases)"
    return 0
  fi
  echo "SELFTEST FAILED ($ST_K of $ST_N)"
  return 1
}

# ----------------------------------------------------------------- main ----

usage() { echo "usage: 34-check.sh frozen|quotes|coverage|ledger|nojs|record|stable|all ... | selftest" >&2; exit 2; }
[ "$#" -gt 0 ] || usage
if [ "$1" = selftest ]; then
  [ "$#" = 1 ] || usage
  selftest
  exit $?
fi

RULES=""
for a in "$@"; do
  case "$a" in
    frozen | quotes | coverage | ledger | nojs | record | stable) RULES="$RULES $a" ;;
    all) RULES="$RULES frozen quotes coverage ledger nojs record stable" ;;
    *) usage ;;
  esac
done

N=0
K=0
S=0
SKIPPED=""
HARD=0
for rule in $RULES; do
  RULE_FAIL=0
  "rule_$rule"
  rc=$?
  if [ "$rc" = "$SKIP" ] && [ "$RULE_FAIL" = 0 ]; then
    S=$((S + 1))
    SKIPPED="$SKIPPED ${rule}"
    continue
  fi
  N=$((N + 1))
  [ "$rc" = 2 ] && HARD=1
  if [ "$RULE_FAIL" != 0 ] || { [ "$rc" != 0 ] && [ "$rc" != 1 ]; }; then K=$((K + 1)); fi
done
if [ "$K" = 0 ]; then
  if [ "$S" = 0 ]; then
    echo "34-CHECK OK ($N rules)"
    exit 0
  fi
  echo "NOT PROVEN:${SKIPPED}"
  if [ "$N" = 0 ]; then
    echo "34-CHECK NOT PROVEN ($N rules, $S skipped)"
    exit "$SKIP"
  fi
  echo "34-CHECK OK ($N rules, $S skipped)"
  exit 0
fi
echo "34-CHECK FAILED ($K of $N)"
[ "$HARD" = 1 ] && exit 2
exit 1
