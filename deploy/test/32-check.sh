#!/usr/bin/env bash
# The gate of phase 32: the promises of 0.1.5 that rest on deploy/BACKUP-VERIFY-RUN.md.
#
#   bash deploy/test/32-check.sh RULE...
#
# RULE is one of frozen, quotes, ledger, deploy, record, all, selftest:
#
# frozen    the evidence still holds: the SHA of the single `Evidence commit:` line of the
#           record is a commit, the bound files of the set (BOUND below) and the source of
#           the binary (cmd, internal, go.mod, go.sum) are unchanged between it and HEAD,
#           and the files the phase changed (BASE...HEAD) are exactly the list of
#           PHASE_FILES. BASE is required; the rule refuses to run without it.
# quotes    every required row of docs/beta.md (REQUIRED_ROWS) exists once, names
#           deploy/BACKUP-VERIFY-RUN.md in its Record column, and every phrase of its
#           Cited words column is found in the record by grep -F, byte for byte. A phrase
#           is a sentence between straight double quotes or a backticked token outside
#           them. A phrase that spans a line break of the record, or that holds a
#           backtick and is cited as a backticked token, is not found, and the rule fails.
#           The same holds for every sentence quoted by a limit marked (0.1.5).
# ledger    the three limits that 0.1.5 answers are quoted in the first column of the
#           ledger table of 0.1.5 (header `| 0.1.1 said | 0.1.5 says | Promise rows |`),
#           each of those rows names promise rows that exist, and no bullet of the section
#           *What X does not promise* still begins with one of them; the bullet on the
#           three gaps of deploy/DEPLOY.md is gone, because the document now states them.
# deploy    deploy/DEPLOY.md names deploy/BACKUP-VERIFY-RUN.md and holds the sentence
#           `Binaries released before 0.1.5 do not carry this refusal`.
# record    when E holds amd64-t32-src, `bash deploy/test/backup-verify.sh record-check`
#           with that E proves the record against its scratch files and must end OK; when
#           it does not, the rule prints `SKIP record: no scratch files under E` and is
#           counted as skipped in the last line, never as passed.
# all       frozen, quotes, ledger, deploy, record, in that order.
# selftest  one negative case per rule, each beside a control that passes, on copies in a
#           temporary directory; the real files are never written.
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
#         BASE=$(git merge-base origin/main HEAD)
#   R     the record; default $ROOT/deploy/BACKUP-VERIFY-RUN.md
#   BETA  the page of promises; default $ROOT/docs/beta.md
#   DEPLOY_MD  the deployment document; default $ROOT/deploy/DEPLOY.md
#   E     the scratch root of the evidence set (record only); default $ROOT/deploy/test/out
#
# The last line is `32-CHECK OK (N rules)`, with `, M skipped` added when the record rule
# was skipped (a skipped rule is not among the N), or `32-CHECK FAILED (K of N)`; the
# selftest ends `SELFTEST OK (N cases)` or `SELFTEST FAILED (K of N)`. Exit status: 0 every
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
REC_NAME=deploy/BACKUP-VERIFY-RUN.md

# The rows of docs/beta.md that must exist and cite the record.
REQUIRED_ROWS="P-23 P-24 P-25 P-26 E-7 E-8"

# The bound files of the evidence set, as the record names them, and the source of the
# binary under test.
BOUND="deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/Dockerfile deploy/test/backup-verify.sh"
BINARY_SRC="cmd internal go.mod go.sum"

# Every file phase 32 changed, sorted as `sort` prints them under LC_ALL=C. The frozen rule
# compares the real change BASE...HEAD with it, name for name.
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
docs/beta.md
internal/db/db.go
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
trap 'rm -rf "$TMPROOT"' EXIT

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
not_in() { printf '%s\n' "$1" | while IFS= read -r l; do [ -n "$l" ] || continue; printf '%s\n' "$2" | grep -qxF -- "$l" || printf '%s ' "$l"; done; }

# --------------------------------------------------------------- rules ----

rule_frozen() {
  need "$R" || return 2
  if [ -z "$BASE" ]; then
    fail "frozen: BASE is required (the commit the phase branched from, e.g. the merge base of origin/main and HEAD)"
    return 2
  fi
  git -C "$ROOT" rev-parse -q --verify "$BASE^{commit}" > /dev/null || { fail "frozen: BASE '$BASE' is not a commit of $ROOT"; return 2; }
  local n sha got want moved
  n=$(grep -c '^Evidence commit: ' "$R")
  [ "$n" = 1 ] || { fail "frozen: $REC_NAME has $n 'Evidence commit:' lines, want exactly one"; return 1; }
  sha=$(sed -n 's/^Evidence commit: \([0-9a-f]*\)$/\1/p' "$R")
  if [ -z "$sha" ] || ! git -C "$ROOT" rev-parse -q --verify "$sha^{commit}" > /dev/null; then
    fail "frozen: the Evidence commit '$sha' of $REC_NAME is not a commit of $ROOT"
    return 1
  fi
  # shellcheck disable=SC2086
  if ! git -C "$ROOT" diff --quiet "$sha" HEAD -- $BOUND; then
    moved=$(git -C "$ROOT" diff --name-only "$sha" HEAD -- $BOUND | tr '\n' ' ')
    fail "frozen: a bound file moved since the evidence commit $sha: $moved"
  fi
  # shellcheck disable=SC2086
  if ! git -C "$ROOT" diff --quiet "$sha" HEAD -- $BINARY_SRC; then
    moved=$(git -C "$ROOT" diff --name-only "$sha" HEAD -- $BINARY_SRC | tr '\n' ' ')
    fail "frozen: the source of the binary moved since the evidence commit $sha: $moved"
  fi
  got=$(git -C "$ROOT" diff --name-only "$BASE...HEAD" | sort)
  want=$(printf '%s\n' "$PHASE_FILES" | sort)
  if [ "$got" != "$want" ]; then
    fail "frozen: the files changed in $BASE...HEAD are not the files of the phase; extra: $(not_in "$got" "$want")missing: $(not_in "$want" "$got")"
  fi
  [ "$RULE_FAIL" = 0 ] && ok "frozen: the 8 bound files and $BINARY_SRC unchanged since $sha, and $BASE...HEAD changed exactly the $(printf '%s\n' "$want" | grep -c .) files of the phase"
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
  while IFS= read -r b; do
    nl=$((nl + 1))
    printf '%s\n' "$b" | grep -qF "Record: $REC_NAME" || fail "quotes: a limit marked (0.1.5) names no 'Record: $REC_NAME': $(short "$b")"
    if ! quotes_of "$b" > /dev/null; then fail "quotes: the double quotes of a limit marked (0.1.5) do not pair up: $(short "$b")"; continue; fi
    n=0
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      n=$((n + 1)); np=$((np + 1))
      grep -qF -- "$p" "$R" || fail "quotes: a limit marked (0.1.5): not in $REC_NAME, byte for byte: $(short "$p")"
    done < <(quotes_of "$b")
    [ "$n" -gt 0 ] || fail "quotes: a limit marked (0.1.5) quotes nothing: $(short "$b")"
  done < <(grep '^- (0\.1\.5) ' "$BETA")
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
  local out last
  if [ ! -d "$E/amd64-t32-src" ]; then
    echo "SKIP record: no scratch files under E ($E)"
    return "$SKIP"
  fi
  out=$(E="$E" R="$R" bash "$ROOT/deploy/test/backup-verify.sh" record-check 2>&1)
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
  out=$(env $envs bash "$ME" "$@" 2>&1)
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

selftest() {
  local T base
  T=$(mktemp -d "$TMPROOT/st.XXXXXX")
  cp "$BETA" "$T/good.md"
  cp "$DEPLOY_MD" "$T/deploy-good.md"
  base=${BASE:-$(git -C "$ROOT" merge-base origin/main HEAD 2> /dev/null)}
  base=$(git -C "$ROOT" rev-parse -q --verify "$base^{commit}" 2> /dev/null)
  # frozen
  st_run 'frozen: control, the record as it is' 0 'ok: frozen' "BASE=$base" "R=$R" -- frozen
  sed "s/^Evidence commit: .*/Evidence commit: $base/" "$R" > "$T/r-base.md"
  st_run 'frozen: the Evidence commit replaced by BASE' 1 'frozen: a bound file moved since the evidence commit' "BASE=$base" "R=$T/r-base.md" -- frozen
  st_run 'frozen: no BASE' 2 'frozen: BASE is required' "BASE=" "R=$R" -- frozen
  # quotes
  st_run 'quotes: control, the page as it is' 0 'ok: quotes' "BETA=$T/good.md" "R=$R" -- quotes
  t_row_append P-23 'NO SUCH PHRASE 32' < "$T/good.md" > "$T/q-absent.md"
  st_run 'quotes: P-23 cites a phrase the record does not hold' 1 'P-23: not in deploy/BACKUP-VERIFY-RUN.md, byte for byte: NO SUCH PHRASE 32' "BETA=$T/q-absent.md" "R=$R" -- quotes
  t_row_drop P-23 < "$T/good.md" > "$T/q-gone.md"
  st_run 'quotes: P-23 is gone' 1 'P-23: 0 rows' "BETA=$T/q-gone.md" "R=$R" -- quotes
  t_sub '"Real bit rot on a disk was not played' '"Real bit rot on a disk was played' < "$T/good.md" > "$T/q-limit.md"
  st_run 'quotes: a limit marked (0.1.5) quotes what the record does not say' 1 'a limit marked (0.1.5): not in deploy/BACKUP-VERIFY-RUN.md' "BETA=$T/q-limit.md" "R=$R" -- quotes
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
  if [ "$ST_K" = 0 ]; then
    echo "SELFTEST OK ($ST_N cases)"
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
