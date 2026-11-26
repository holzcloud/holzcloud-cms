#!/usr/bin/env bash
# The gate of phase 32: the promises of 0.1.5 that rest on deploy/BACKUP-VERIFY-RUN.md.
#
#   bash deploy/test/32-check.sh RULE...
#
# RULE is one of frozen, quotes, ledger, deploy, record, all, selftest:
#
# frozen    the evidence still holds: the SHA of the single `Evidence commit:` line of the
#           record is a commit, the bound files of the set (BOUND below) are unchanged
#           between it and HEAD, the source of the binary (cmd, internal, go.mod, go.sum,
#           changelog.go, CHANGELOG.md) changed in nothing but CHANGELOG.md and there only
#           by new version sections (one block of lines directly above the older first `## `
#           line, all of its level-two headings `## X.Y.Z — YYYY-MM-DD` of a version no
#           older heading has, every older line and the preamble byte for byte as they
#           were), and the files the phase changed (BASE...TIP) are
#           exactly the list of PHASE_FILES. BASE is required; the rule refuses to run
#           without it. TIP is HEAD unless set, so on the branch of phase 32 the rule is
#           what it was. After the merge, BASE...HEAD lists every later phase too, so the
#           rule runs with TIP set to the merge of phase 32:
#             BASE=e1116b3 TIP=93bfb80 bash deploy/test/32-check.sh all
#           (93bfb80 merged pull request 88, the branch of phase 32, onto e1116b3). TIP only
#           moves the list of the phase's files; the bound files and the binary's source
#           are always compared with the judged tree: the commit AT, or else the working tree
#           of ROOT with its untracked files, so an edit that is not committed shows.
# quotes    every required row of docs/beta.md (REQUIRED_ROWS) exists once, names
#           deploy/BACKUP-VERIFY-RUN.md in its Record column, and every phrase of its
#           Cited words column is found in the record by grep -F, byte for byte. A phrase
#           is a sentence between straight double quotes or a backticked token outside
#           them. A phrase that spans a line break of the record, or that holds a
#           backtick and is cited as a backticked token, is not found, and the rule fails.
#           The same holds for every sentence quoted by a limit marked (0.1.5) that names
#           `Record: deploy/BACKUP-VERIFY-RUN.md`, and at least six such limits must be
#           there. A limit marked (0.1.5) that names another record of 0.1.5 is checked by
#           deploy/test/34-check.sh, the gate of phase 34, and not here.
# ledger    the three limits that 0.1.5 answers are quoted in the first column of the
#           ledger table of 0.1.5 (header `| 0.1.1 said | 0.1.5 says | Promise rows |`),
#           each of those rows names promise rows that exist, and no bullet of the section
#           *What X does not promise* still begins with one of them; the bullet on the
#           three gaps of deploy/DEPLOY.md is gone, because the document now states them.
# deploy    deploy/DEPLOY.md names deploy/BACKUP-VERIFY-RUN.md and holds the sentence
#           `Binaries released before 0.1.5 do not carry this refusal`.
# record    when E holds amd64-t32-src, `bash deploy/test/backup-verify.sh record-check`
#           with that E proves the record against its scratch files and must end OK. With AT
#           set it runs in a temporary git worktree at AT, removed again, because the
#           record-check compares the binary's source with the tree it runs in; when
#           it does not, the rule prints `SKIP record: no scratch files under E` and is
#           counted as skipped in the last line, never as passed.
# all       frozen, quotes, ledger, deploy, record, in that order.
# selftest  one negative case per rule, each beside a control that passes, on copies in a
#           temporary directory; the real files are never written. Run bare, it pins the
#           bounds of phase 32 itself (BASE=e1116b3, TIP=93bfb80) for the frozen cases, so
#           it passes on any checkout after the merge; with BASE set it uses BASE and TIP
#           as given. Where e1116b3 and 93bfb80 are not in the clone (a shallow clone) the
#           frozen cases that need them print `SKIP …`, count as neither passed nor failed,
#           and the last line says `SELFTEST OK (N cases, S skipped)`.
#
# Why this file and not the archived gate. The gate of v0.1.1 is
# .planning/milestones/v0.1.1-phases/28-the-promise-in-writing/28-check.sh. It stays as it
# is: it is the gate of record of that release. It does not run from its archived place,
# because .planning is a git repository of its own and the gate resolves the repository
# from its own location; and on main it has been red since 0.1.2 for reasons outside this
# phase (deploy/DEPLOY.md, .github/workflows/release.yml and deploy/NATIVE-AMD64-RUN.md
# changed after its evidence, and the heading of the beta structure changed in af6c4ac).
# This gate lives beside the drivers it checks, so that it runs where the evidence lives.
#
# Environment:
#   ROOT  the repository; default: the repository that holds this file. Every git command
#         of this file runs as `git -C "$ROOT"`, so the gate works from a worktree and from
#         a copy of the file elsewhere.
#   BASE  the commit the phase branched from (frozen only), for example
#         BASE=$(git merge-base origin/main HEAD); after the merge, e1116b3
#   TIP   the end of the phase's change (frozen only); default HEAD. After the merge of
#         phase 32, 93bfb80, its merge commit
#   AT    the commit this gate judges as the state of the repository (frozen and record);
#         default empty, which means the working tree of ROOT, untracked files included.
#         After a release the live HEAD moves on, so a gate of that release is proved by
#         naming the release commit (AT=v0.1.5); without AT a local run sees an edit that
#         is not committed yet (IN-04 of the review of phase 34). BASE...TIP are commits.
#   R     the record; default $ROOT/deploy/BACKUP-VERIFY-RUN.md
#   BETA  the page of promises; default $ROOT/docs/beta.md
#   DEPLOY_MD  the deployment document; default $ROOT/deploy/DEPLOY.md
#   E     the scratch root of the evidence set (record only); default $ROOT/deploy/test/out
#
# The last line is `32-CHECK OK (N rules)`, with `, M skipped` added when the record rule
# was skipped (a skipped rule is not among the N), or `32-CHECK FAILED (K of N)`; the
# selftest ends `SELFTEST OK (N cases)`, `SELFTEST OK (N cases, S skipped)` or
# `SELFTEST FAILED (K of N)`. Exit status: 0 every
# rule passed, 1 a rule failed, 2 usage or a rule could not run.
#
# bash 3.2 and BWK awk (macOS): no mapfile, no associative arrays, no gawk extensions.
set -u
export LC_ALL=C

HERE=$(cd "$(dirname "$0")" && pwd)
if [ -z "${ROOT:-}" ]; then
  ROOT=$(git -C "$HERE" rev-parse --show-toplevel 2> /dev/null) || ROOT=$(cd "$HERE/../.." && pwd)
fi
ME="$HERE/$(basename "$0")"
R=${R:-$ROOT/deploy/BACKUP-VERIFY-RUN.md}
BETA=${BETA:-$ROOT/docs/beta.md}
DEPLOY_MD=${DEPLOY_MD:-$ROOT/deploy/DEPLOY.md}
E=${E:-$ROOT/deploy/test/out}
BASE=${BASE:-}
TIP=${TIP:-HEAD}
AT=${AT:-}
REC_NAME=deploy/BACKUP-VERIFY-RUN.md

# The rows of docs/beta.md that must exist and cite the record.
REQUIRED_ROWS="P-23 P-24 P-25 P-26 E-7 E-8"
# The limits marked (0.1.5) that name the record: at least this many.
MIN_LIMITS=6

# The bound files of the evidence set, as the record names them, and the source of the
# binary under test; src_since lets a release change CHANGELOG.md, and only by new sections.
BOUND="deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/Dockerfile deploy/test/backup-verify.sh"
BINARY_SRC="cmd internal go.mod go.sum changelog.go CHANGELOG.md"

# Every file phase 32 changed, sorted as `sort` prints them under LC_ALL=C. The frozen rule
# compares the real change BASE...TIP with it, name for name.
PHASE_FILES="cmd/holzcloud/check_files_test.go
cmd/holzcloud/cli.go
cmd/holzcloud/deploy_docs_test.go
cmd/holzcloud/deploy_scripts_test.go
cmd/holzcloud/main.go
cmd/holzcloud/schema_guard_test.go
deploy/BACKUP-VERIFY-RUN.md
deploy/DEPLOY.md
deploy/backup.sh
deploy/restore.sh
deploy/test/32-check.sh
deploy/test/README.md
deploy/test/backup-robust.sh
deploy/test/backup-verify.sh
deploy/test/restore-robust.sh
deploy/test/run.sh
docs/beta.md
internal/db/db.go
internal/db/maintain.go
internal/db/maintain_test.go
internal/db/schema_guard.go
internal/db/schema_guard_test.go
internal/media/missing.go
internal/media/missing_test.go"

# The three limits of 0.1.1 that the record of 0.1.5 answers, by their first words.
LIMIT_1='Bit rot in a complete backup is not caught'
LIMIT_2='A restore does not report a row whose file the backup lacks'
LIMIT_3='A failed removal in the retention step is silent'
GAPS_BULLET='- Three gaps of `deploy/DEPLOY.md`'
LEDGER_HEAD='| 0.1.1 said | 0.1.5 says | Promise rows |'
DEPLOY_SENTENCE='Binaries released before 0.1.5 do not carry this refusal'

TMPROOT=$(mktemp -d) || exit 2
# WT is the worktree of AT while the record rule has one. It is removed by name, never with
# git's prune subcommand, which would touch worktrees that are not this gate's.
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

# section2 HEADING-ERE < FILE : the lines under a level-two heading, up to the next one.
section2() {
  awk -v re="$1" '
    /^```/ { if (on) print; fence = !fence; next }
    fence { if (on) print; next }
    /^## / { if (on) exit; if ($0 ~ re) { on = 1; next } }
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

# --------------------------------------------------------------- rules ----

rule_frozen() {
  need "$R" || return 2
  if [ -z "$BASE" ]; then
    fail "frozen: BASE is required (the commit the phase branched from, e.g. the merge base of origin/main and HEAD)"
    return 2
  fi
  git -C "$ROOT" rev-parse -q --verify "$BASE^{commit}" > /dev/null || { fail "frozen: BASE '$BASE' is not a commit of $ROOT"; return 2; }
  git -C "$ROOT" rev-parse -q --verify "$TIP^{commit}" > /dev/null || { fail "frozen: TIP '$TIP' is not a commit of $ROOT"; return 2; }
  local n sha got want moved
  at_ok frozen || return 2
  n=$(grep -c '^Evidence commit: ' "$R")
  [ "$n" = 1 ] || { fail "frozen: $REC_NAME has $n 'Evidence commit:' lines, want exactly one"; return 1; }
  sha=$(sed -n 's/^Evidence commit: \([0-9a-f]*\)$/\1/p' "$R")
  if [ -z "$sha" ] || ! git -C "$ROOT" rev-parse -q --verify "$sha^{commit}" > /dev/null; then
    fail "frozen: the Evidence commit '$sha' of $REC_NAME is not a commit of $ROOT"
    return 1
  fi
  # shellcheck disable=SC2086
  moved=$(moved_since "$sha" $BOUND | tr '\n' ' ')
  if [ -n "$moved" ]; then
    fail "frozen: a bound file moved since the evidence commit $sha (judged: $(judged)): $moved"
  fi
  if ! moved=$(src_since "$sha"); then
    fail "frozen: the source of the binary moved since the evidence commit $sha (judged: $(judged)): $moved"
  fi
  got=$(git -C "$ROOT" diff --name-only "$BASE...$TIP" | sort)
  want=$(printf '%s\n' "$PHASE_FILES" | sort)
  if [ "$got" != "$want" ]; then
    fail "frozen: the files changed in $BASE...$TIP (TIP $TIP) are not the files of the phase; extra: $(not_in "$got" "$want")missing: $(not_in "$want" "$got")"
  fi
  [ "$RULE_FAIL" = 0 ] && ok "frozen: judged: $(judged); the 8 bound files unchanged and $BINARY_SRC unchanged but for new version sections of CHANGELOG.md since $sha, and $BASE...$TIP changed exactly the $(printf '%s\n' "$want" | grep -c .) files of the phase"
}

rule_quotes() {
  need "$BETA" || return 2
  need "$R" || return 2
  local id row rec cited p n np=0 nr=0 b nl=0
  for id in $REQUIRED_ROWS; do
    n=$(grep -c "^| $id |" "$BETA")
    if [ "$n" != 1 ]; then fail "quotes: $id: $n rows in $BETA, want exactly one"; continue; fi
    nr=$((nr + 1))
    row=$(grep "^| $id |" "$BETA")
    rec=$(cell "$row" 3 | tr -d '` ')
    [ "$rec" = "$REC_NAME" ] || fail "quotes: $id: its Record column names '$rec', not $REC_NAME"
    cited=$(cell "$row" 4)
    if ! quotes_of "$cited" > /dev/null; then fail "quotes: $id: the double quotes of its Cited words do not pair up"; continue; fi
    n=0
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      n=$((n + 1)); np=$((np + 1))
      grep -qF -- "$p" "$R" || fail "quotes: $id: not in $REC_NAME, byte for byte: $(short "$p")"
    done < <(phrases_of "$cited")
    [ "$n" -gt 0 ] || fail "quotes: $id: no cited words"
  done
  # Only the limits that name this record; those of the other records of 0.1.5 are the
  # business of deploy/test/34-check.sh.
  while IFS= read -r b; do
    printf '%s\n' "$b" | grep -qF "Record: $REC_NAME" || continue
    nl=$((nl + 1))
    if ! quotes_of "$b" > /dev/null; then fail "quotes: the double quotes of a limit marked (0.1.5) do not pair up: $(short "$b")"; continue; fi
    n=0
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      n=$((n + 1)); np=$((np + 1))
      grep -qF -- "$p" "$R" || fail "quotes: a limit marked (0.1.5): not in $REC_NAME, byte for byte: $(short "$p")"
    done < <(quotes_of "$b")
    [ "$n" -gt 0 ] || fail "quotes: a limit marked (0.1.5) quotes nothing: $(short "$b")"
  done < <(grep '^- (0\.1\.5) ' "$BETA")
  [ "$nl" -ge "$MIN_LIMITS" ] || fail "quotes: $nl limits marked (0.1.5) name Record: $REC_NAME, want at least $MIN_LIMITS"
  [ "$RULE_FAIL" = 0 ] && ok "quotes: $nr rows ($REQUIRED_ROWS) and $nl limits marked (0.1.5) cite $REC_NAME, $np phrases found byte for byte"
}

rule_ledger() {
  need "$BETA" || return 2
  local np rows lim row c1 c3 p n=0 bullets
  np=$(section2 '^## What [0-9.]+ does not promise$' < "$BETA")
  [ -n "$np" ] || { fail "ledger: no section 'What X does not promise' in $BETA"; return 1; }
  bullets=$(printf '%s\n' "$np" | grep '^- ')
  rows=$(ledger_rows < "$BETA")
  [ -n "$rows" ] || fail "ledger: no table headed '$LEDGER_HEAD' in $BETA"
  for lim in "$LIMIT_1" "$LIMIT_2" "$LIMIT_3"; do
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
  if printf '%s\n' "$np" | grep -qF -- "$GAPS_BULLET"; then fail "ledger: the bullet on the three gaps of deploy/DEPLOY.md is still there"; fi
  [ "$RULE_FAIL" = 0 ] && ok "ledger: $n limits quoted in the 0.1.5 ledger and gone from the bullets, the three-gaps bullet gone"
}

rule_deploy() {
  need "$DEPLOY_MD" || return 2
  grep -qF -- "$REC_NAME" "$DEPLOY_MD" || fail "deploy: deploy/DEPLOY.md does not name $REC_NAME"
  grep -qF -- "$DEPLOY_SENTENCE" "$DEPLOY_MD" || fail "deploy: deploy/DEPLOY.md does not hold '$DEPLOY_SENTENCE'"
  [ "$RULE_FAIL" = 0 ] && ok "deploy: deploy/DEPLOY.md names $REC_NAME and says '$DEPLOY_SENTENCE'"
}

rule_record() {
  local out last run="$ROOT" rr="$R" ee="$E"
  if [ ! -d "$E/amd64-t32-src" ]; then
    echo "SKIP record: no scratch files under E ($E)"
    return "$SKIP"
  fi
  if [ -n "$AT" ]; then
    # The record-check compares the binary's source with the tree it runs in, so to prove
    # the release it runs in a worktree of the release. R and E are made absolute first,
    # because the check no longer runs where the caller stood.
    at_ok record || return 2
    case "$rr" in /*) ;; *) rr="$PWD/$rr" ;; esac
    case "$ee" in /*) ;; *) ee="$PWD/$ee" ;; esac
    WT="$TMPROOT/at"
    if ! git -C "$ROOT" -c core.hooksPath=/dev/null worktree add --detach -q "$WT" "$AT" > /dev/null 2>&1; then
      WT=""
      fail "record: git worktree add of AT $AT failed"
      return 2
    fi
    run="$WT"
  fi
  out=$(cd "$run" && E="$ee" R="$rr" bash "$run/deploy/test/backup-verify.sh" record-check 2>&1)
  if [ -n "$WT" ]; then
    git -C "$ROOT" worktree remove --force "$WT" > /dev/null 2>&1
    WT=""
  fi
  last=$(printf '%s\n' "$out" | tail -n 1)
  case "$last" in
    'RECORD-CHECK OK ('*) ok "record: $last with E=$E" ;;
    *)
      printf '%s\n' "$out" | tail -n 5
      fail "record: backup-verify.sh record-check did not end OK"
      ;;
  esac
}

# ------------------------------------------------------------- selftest ----

ST_N=0
ST_K=0
ST_S=0
st_skip() { ST_S=$((ST_S + 1)); echo "SKIP $1"; }
st_pass() { ST_N=$((ST_N + 1)); echo "ok $ST_N $1"; }
st_bad() { ST_N=$((ST_N + 1)); ST_K=$((ST_K + 1)); echo "FAIL $ST_N $1"; }
# st_run DESC WANT-EXIT MESSAGE ENV... -- RULE : run this file on one rule with the
# environment given; the exit status must be WANT-EXIT and the output must hold MESSAGE.
st_run() {
  local desc="$1" want="$2" msg="$3" out rc envs=""
  shift 3
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs="$envs $1"; shift; done
  shift
  # shellcheck disable=SC2086
  # AT= first: a value the caller of the selftest set must not reach a case that did not ask
  # for it; a case that names AT comes later in the list and wins.
  out=$(env AT= $envs bash "$ME" "$@" 2>&1)
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
# t_sub OLD NEW < FILE : the first OLD of each line replaced by NEW.
t_sub() { O="$1" N="$2" awk 'BEGIN { o = ENVIRON["O"]; n = ENVIRON["N"]; ol = length(o) } { i = index($0, o); if (i > 0) $0 = substr($0, 1, i - 1) n substr($0, i + ol); print }'; }
# t_after LINE NEW < FILE : NEW inserted after the line that begins with LINE.
t_after() { O="$1" N="$2" awk 'BEGIN { o = ENVIRON["O"]; n = ENVIRON["N"] } { print } index($0, o) == 1 { print n }'; }
# t_drop_line LINE < FILE : the file without the lines that begin with LINE.
t_drop_line() { O="$1" awk 'BEGIN { o = ENVIRON["O"] } index($0, o) != 1 { print }'; }

# t_commit DIR : commit every tracked change in DIR, as upgrade-verify.sh does.
t_commit() {
  git -C "$1" -c user.name=t32 -c user.email=t32@invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -am selftest
}
# t_cl_top VERSION DATE < FILE : a new section, directly above the first level-two heading.
t_cl_top() {
  V="$1" D="$2" awk 'BEGIN { v = ENVIRON["V"]; d = ENVIRON["D"] }
    !done && /^## / { printf "## %s — %s\n\n### Neu\n\n- Ein Fall des Selbsttests.\n\n", v, d; done = 1 } { print }'
}
# t_cl_case DIR : in the clone DIR, reset to HEADSHA and add a new top section to
# CHANGELOG.md. The caller edits further and commits.
t_cl_case() {
  git -C "$1" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA" || return 1
  t_cl_top 9.9.9 2099-01-01 < "$1/CHANGELOG.md" > "$1/CHANGELOG.md.new" && mv "$1/CHANGELOG.md.new" "$1/CHANGELOG.md"
}

selftest() {
  local T base tip tipsha at atarg wt0
  T=$(mktemp -d "$TMPROOT/st.XXXXXX")
  cp "$BETA" "$T/good.md"
  cp "$DEPLOY_MD" "$T/deploy-good.md"
  # The files of phase 32 are BASE...TIP only for the pair that bounds phase 32. Merged
  # into main, a later branch makes BASE...HEAD every later phase too, so the control
  # would fail on every checkout after the merge. The selftest therefore pins the pair
  # of phase 32 itself (the same one the header of this file names) when BASE is not
  # given and both commits are here. Where they are not (a shallow clone), the cases
  # that need them say SKIP: they are neither passed nor failed, and the last line says
  # how many were skipped.
  pin_base=e1116b3
  pin_tip=93bfb80
  pin_ok=0
  if git -C "$ROOT" rev-parse -q --verify "$pin_base^{commit}" > /dev/null 2>&1 \
    && git -C "$ROOT" rev-parse -q --verify "$pin_tip^{commit}" > /dev/null 2>&1; then
    pin_ok=1
  fi
  control_ok=1
  if [ -n "${BASE:-}" ]; then
    base=$BASE
    tip=${TIP:-HEAD}
  elif [ "$pin_ok" = 1 ]; then
    base=$pin_base
    tip=$pin_tip
  else
    base=$(git -C "$ROOT" merge-base origin/main HEAD 2> /dev/null)
    tip=${TIP:-HEAD}
    control_ok=0
  fi
  base=$(git -C "$ROOT" rev-parse -q --verify "$base^{commit}" 2> /dev/null)
  wt0=$(git -C "$ROOT" worktree list | grep -c .)
  # The cases on ROOT judge the release whose record this gate proves, not the live HEAD,
  # which moved on after it (pull request 101 changed internal/plugin). AT given by the
  # caller is kept; otherwise the tag v0.1.5 when this clone has it, and the working tree
  # when it has not.
  at=$AT
  if [ -z "$at" ] && git -C "$ROOT" rev-parse -q --verify 'v0.1.5^{commit}' > /dev/null 2>&1; then at=v0.1.5; fi
  atarg="AT=$at"
  # frozen
  if [ "$control_ok" = 1 ] && [ -n "$base" ]; then
    st_run 'frozen: control, the record as it is' 0 'ok: frozen' "BASE=$base" "TIP=$tip" "R=$R" "$atarg" -- frozen
  else
    st_skip "frozen: control, the record as it is: no BASE given and $pin_base, $pin_tip (the bounds of phase 32) are not in this clone"
  fi
  if [ -n "$base" ]; then
    sed "s/^Evidence commit: .*/Evidence commit: $base/" "$R" > "$T/r-base.md"
    st_run 'frozen: the Evidence commit replaced by BASE' 1 'frozen: a bound file moved since the evidence commit' "BASE=$base" "TIP=$tip" "R=$T/r-base.md" "$atarg" -- frozen
  else
    st_skip 'frozen: the Evidence commit replaced by BASE: no base commit can be found in this clone'
  fi
  st_run 'frozen: no BASE' 2 'frozen: BASE is required' "BASE=" "TIP=$tip" "R=$R" "$atarg" -- frozen
  if [ "$pin_ok" = 1 ]; then
    st_run 'frozen: after the merge, phase 32 compared on its own merge (BASE=e1116b3 TIP=93bfb80)' 0 'e1116b3...93bfb80 changed exactly the 24 files of the phase' "BASE=e1116b3" "TIP=93bfb80" "R=$R" "$atarg" -- frozen
    st_run 'frozen: TIP unset compares against HEAD, and says so' 1 'the files changed in e1116b3...HEAD (TIP HEAD) are not the files of the phase' "BASE=e1116b3" "TIP=" "R=$R" "$atarg" -- frozen
  else
    st_skip 'frozen: after the merge, phase 32 compared on its own merge (BASE=e1116b3 TIP=93bfb80): the commits are not in this clone'
    st_skip 'frozen: TIP unset compares against HEAD, and says so: e1116b3 is not in this clone'
  fi
  # A scratch clone of ROOT with one commit on top of HEAD per case. The cases need the
  # pinned bounds, like the control. TIP is resolved here: inside the clone, HEAD is the
  # case's commit and would make BASE...TIP list the case.
  if [ "$control_ok" = 1 ] && [ -n "$base" ]; then
    HEADSHA=$(git -C "$ROOT" rev-parse "${at:-HEAD}^{commit}")
    tipsha=$(git -C "$ROOT" rev-parse -q --verify "$tip^{commit}")
    git -c core.hooksPath=/dev/null clone -q --shared --no-checkout "$ROOT" "$T/clone" 2> /dev/null
    t_cl_case "$T/clone"
    t_commit "$T/clone"
    st_run 'frozen: a commit that adds a new top section to CHANGELOG.md passes' 0 'ok: frozen' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
    t_cl_case "$T/clone"
    printf '// selftest\n' >> "$T/clone/changelog.go"
    t_commit "$T/clone"
    st_run 'frozen: a commit that also changes changelog.go fails' 1 'beyond CHANGELOG.md: changelog.go' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
    t_cl_case "$T/clone"
    sed 's/^## 0\.1\.4 — 2026-11-12$/## 0.1.4 — 2026-11-13/' "$T/clone/CHANGELOG.md" > "$T/clone/CHANGELOG.md.new" && mv "$T/clone/CHANGELOG.md.new" "$T/clone/CHANGELOG.md"
    t_commit "$T/clone"
    st_run 'frozen: a commit that adds a section and edits an older line of CHANGELOG.md fails' 1 'an older line changed, moved or was removed' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
    # Without AT the gate judges the working tree. The clone is checked out clean at the
    # pin, so the control passes; an untracked file under the source of the binary and an
    # edit of a bound file that is not committed each make it fail, and with AT=HEAD the
    # same edit is invisible, because AT names a commit and the edit is in none.
    git -C "$T/clone" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA"
    st_run 'frozen: AT unset, the clone checked out clean at the pin: the working tree passes' 0 'ok: frozen' "ROOT=$T/clone" "BASE=$base" "TIP=$tipsha" -- frozen
    printf 'package selftest\n' > "$T/clone/internal/selftest_untracked.go"
    st_run 'frozen: AT unset, an untracked file under the source of the binary fails' 1 '(judged: the working tree): beyond CHANGELOG.md: internal/selftest_untracked.go' "ROOT=$T/clone" "BASE=$base" "TIP=$tipsha" -- frozen
    rm -f "$T/clone/internal/selftest_untracked.go"
    printf '# selftest\n' >> "$T/clone/deploy/backup.sh"
    st_run 'frozen: AT unset, an uncommitted edit of a bound file fails and names it' 1 '(judged: the working tree): deploy/backup.sh' "ROOT=$T/clone" "BASE=$base" "TIP=$tipsha" -- frozen
    st_run 'frozen: AT=HEAD does not see the uncommitted edit of a bound file' 0 'ok: frozen' "ROOT=$T/clone" "AT=HEAD" "BASE=$base" "TIP=$tipsha" -- frozen
    git -C "$T/clone" -c core.hooksPath=/dev/null checkout -q -f --detach "$HEADSHA"
    st_run 'frozen: AT is not a commit' 2 "frozen: AT 'no-such-ref' is not a commit" "BASE=$base" "TIP=$tip" "AT=no-such-ref" -- frozen
  else
    st_skip 'frozen: the three cases of a commit that changes CHANGELOG.md: no BASE given and the bounds of phase 32 are not in this clone'
  fi
  # quotes
  st_run 'quotes: control, the page as it is' 0 'ok: quotes' "BETA=$T/good.md" "R=$R" -- quotes
  t_row_append P-23 'NO SUCH PHRASE 32' < "$T/good.md" > "$T/q-absent.md"
  st_run 'quotes: P-23 cites a phrase the record does not hold' 1 'P-23: not in deploy/BACKUP-VERIFY-RUN.md, byte for byte: NO SUCH PHRASE 32' "BETA=$T/q-absent.md" "R=$R" -- quotes
  t_row_drop P-23 < "$T/good.md" > "$T/q-gone.md"
  st_run 'quotes: P-23 is gone' 1 'P-23: 0 rows' "BETA=$T/q-gone.md" "R=$R" -- quotes
  t_sub '"Real bit rot on a disk was not played' '"Real bit rot on a disk was played' < "$T/good.md" > "$T/q-limit.md"
  st_run 'quotes: a limit marked (0.1.5) quotes what the record does not say' 1 'a limit marked (0.1.5): not in deploy/BACKUP-VERIFY-RUN.md' "BETA=$T/q-limit.md" "R=$R" -- quotes
  t_after '- (0.1.5) Native amd64 timing of the checksum step' '- (0.1.5) A limit of another record. Why: a case of the selftest. Record: deploy/MIGRATION-RESTORE-RUN.md, *Not tested*: "NO SUCH PHRASE 34 in deploy/BACKUP-VERIFY-RUN.md."' < "$T/good.md" > "$T/q-other.md"
  st_run 'quotes: a limit marked (0.1.5) of another record is left to the gate of phase 34' 0 'ok: quotes' "BETA=$T/q-other.md" "R=$R" -- quotes
  t_drop_line '- (0.1.5) Native amd64 timing of the checksum step' < "$T/good.md" > "$T/q-five.md"
  st_run 'quotes: fewer than six limits marked (0.1.5) name the record' 1 'quotes: 5 limits marked (0.1.5) name Record: deploy/BACKUP-VERIFY-RUN.md, want at least 6' "BETA=$T/q-five.md" "R=$R" -- quotes
  # ledger
  st_run 'ledger: control, the page as it is' 0 'ok: ledger' "BETA=$T/good.md" -- ledger
  t_after '### The nightly backup and a restore' "- $LIMIT_1: a flipped byte inside a data page passes both integrity readers. Why: kept." < "$T/good.md" > "$T/l-bullet.md"
  st_run 'ledger: a limit left as a bullet' 1 "ledger: still a bullet of the does-not-promise section: $LIMIT_1" "BETA=$T/l-bullet.md" -- ledger
  # deploy
  st_run 'deploy: control, the document as it is' 0 'ok: deploy' "DEPLOY_MD=$T/deploy-good.md" -- deploy
  t_sub "$DEPLOY_SENTENCE" 'Older binaries lack it' < "$T/deploy-good.md" > "$T/deploy-bad.md"
  st_run 'deploy: the sentence on binaries before 0.1.5 gone' 1 "does not hold '$DEPLOY_SENTENCE'" "DEPLOY_MD=$T/deploy-bad.md" -- deploy
  # record
  mkdir -p "$T/empty-e"
  st_run 'record: E is an empty directory, so the rule is skipped and not passed' 0 'SKIP record: no scratch files under E' "E=$T/empty-e" "R=$R" -- record
  st_run 'record: a skipped rule is counted as skipped in the last line' 0 '32-CHECK OK (0 rules, 1 skipped)' "E=$T/empty-e" "R=$R" -- record
  # With AT the record-check runs in a worktree of AT, which is gone again afterwards.
  mkdir -p "$T/empty-src/amd64-t32-src"
  st_run 'record: AT set and an empty amd64-t32-src, so the record-check fails' 1 'record: backup-verify.sh record-check did not end OK' "E=$T/empty-src" "R=$R" "AT=${at:-HEAD}" -- record
  if [ "$(git -C "$ROOT" worktree list | grep -c .)" = "$wt0" ]; then st_pass 'record: no worktree of ROOT is left behind'; else st_bad 'record: no worktree of ROOT is left behind'; fi
  if [ "$ST_K" = 0 ]; then
    if [ "$ST_S" = 0 ]; then echo "SELFTEST OK ($ST_N cases)"; else echo "SELFTEST OK ($ST_N cases, $ST_S skipped)"; fi
    return 0
  fi
  echo "SELFTEST FAILED ($ST_K of $ST_N)"
  return 1
}

# ----------------------------------------------------------------- main ----

usage() { echo "usage: 32-check.sh frozen|quotes|ledger|deploy|record|all ... | selftest" >&2; exit 2; }
[ "$#" -gt 0 ] || usage
if [ "$1" = selftest ]; then
  [ "$#" = 1 ] || usage
  selftest
  exit $?
fi

RULES=""
for a in "$@"; do
  case "$a" in
    frozen | quotes | ledger | deploy | record) RULES="$RULES $a" ;;
    all) RULES="$RULES frozen quotes ledger deploy record" ;;
    *) usage ;;
  esac
done

N=0
K=0
S=0
HARD=0
for rule in $RULES; do
  RULE_FAIL=0
  "rule_$rule"
  rc=$?
  if [ "$rc" = "$SKIP" ] && [ "$RULE_FAIL" = 0 ]; then
    S=$((S + 1))
    continue
  fi
  N=$((N + 1))
  [ "$rc" = 2 ] && HARD=1
  if [ "$RULE_FAIL" != 0 ] || { [ "$rc" != 0 ] && [ "$rc" != 1 ]; }; then K=$((K + 1)); fi
done
if [ "$K" = 0 ]; then
  if [ "$S" = 0 ]; then echo "32-CHECK OK ($N rules)"; else echo "32-CHECK OK ($N rules, $S skipped)"; fi
  exit 0
fi
echo "32-CHECK FAILED ($K of $N)"
[ "$HARD" = 1 ] && exit 2
exit 1
