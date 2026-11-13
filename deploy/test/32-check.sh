#!/usr/bin/env bash
# The gate of phase 32: the promises of 0.1.5 that rest on deploy/BACKUP-VERIFY-RUN.md.
#
#   bash deploy/test/32-check.sh RULE...     RULE is one of: quotes, all, selftest
#
# quotes    every required row of docs/beta.md (P-23) exists, names
#           deploy/BACKUP-VERIFY-RUN.md in its Record column, and every phrase of its
#           Cited words column is found in the record by grep -F, byte for byte. A phrase
#           is a sentence between straight double quotes or a backticked token outside
#           them. A phrase that spans a line break of the record, or that holds a
#           backtick and is cited as a backticked token, is not found, and the rule fails.
# all       every rule above, in order.
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
#   R     the record; default $ROOT/deploy/BACKUP-VERIFY-RUN.md
#   BETA  the page of promises; default $ROOT/docs/beta.md
#
# The last line is `32-CHECK OK (N rules)` or `32-CHECK FAILED (K of N)`; the selftest
# ends `SELFTEST OK (N cases)` or `SELFTEST FAILED (K of N)`. Exit status: 0 every rule
# passed, 1 a rule failed, 2 usage or a rule could not run.
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
REC_NAME=deploy/BACKUP-VERIFY-RUN.md

# The rows of docs/beta.md that must exist and cite the record.
REQUIRED_ROWS="P-23"

TMPROOT=$(mktemp -d) || exit 2
trap 'rm -rf "$TMPROOT"' EXIT

RULE_FAIL=0
fail() { echo "FAIL: $*"; RULE_FAIL=$((RULE_FAIL + 1)); }
ok() { echo "ok: $*"; }
need() { [ -f "$1" ] || { echo "FAIL: missing file: $1"; return 2; }; }

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

# --------------------------------------------------------------- rules ----

rule_quotes() {
  need "$BETA" || return 2
  need "$R" || return 2
  local id row rec cited p n np=0 nr=0
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
  [ "$RULE_FAIL" = 0 ] && ok "quotes: $nr rows ($REQUIRED_ROWS) cite $REC_NAME, $np phrases found byte for byte"
}

# ------------------------------------------------------------- selftest ----

ST_N=0
ST_K=0
st_pass() { ST_N=$((ST_N + 1)); echo "ok $ST_N $1"; }
st_bad() { ST_N=$((ST_N + 1)); ST_K=$((ST_K + 1)); echo "FAIL $ST_N $1"; }
# st_run DESC WANT-EXIT MESSAGE ENV... -- RULE : run this file on one rule with the
# environment given; the exit status must be WANT-EXIT and the output must hold MESSAGE.
st_run() {
  local desc="$1" want="$2" msg="$3" out rc
  shift 3
  local envs=""
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do envs="$envs $1"; shift; done
  shift
  # shellcheck disable=SC2086
  out=$(env $envs bash "$ME" "$@" 2>&1)
  rc=$?
  if [ "$rc" != "$want" ]; then st_bad "$desc: exit $rc, want $want; last line: $(printf '%s\n' "$out" | tail -n 1)"; return; fi
  if [ -n "$msg" ] && ! printf '%s\n' "$out" | grep -qF -- "$msg"; then st_bad "$desc: the output does not say '$msg'"; return; fi
  st_pass "$desc"
}
# t_row ID AWK-PROGRAM-ON-THAT-LINE < FILE : rewrite the table row of one ID.
t_row_append() { # ID TOKEN : append a backticked token to the Cited words of the row ID
  ID="$1" TOK="$2" awk 'BEGIN { id = "| " ENVIRON["ID"] " |"; t = ENVIRON["TOK"] }
    index($0, id) == 1 { sub(/ \|$/, ", `" t "` |") } { print }'
}
t_row_drop() { ID="$1" awk 'BEGIN { id = "| " ENVIRON["ID"] " |" } index($0, id) != 1 { print }'; }

selftest() {
  local T
  T=$(mktemp -d "$TMPROOT/st.XXXXXX")
  cp "$BETA" "$T/good.md"
  # quotes
  st_run 'quotes: control, the page as it is' 0 'ok: quotes' "BETA=$T/good.md" "R=$R" -- quotes
  t_row_append P-23 'NO SUCH PHRASE 32' < "$T/good.md" > "$T/q-absent.md"
  st_run 'quotes: P-23 cites a phrase the record does not hold' 1 'P-23: not in deploy/BACKUP-VERIFY-RUN.md, byte for byte: NO SUCH PHRASE 32' "BETA=$T/q-absent.md" "R=$R" -- quotes
  t_row_drop P-23 < "$T/good.md" > "$T/q-gone.md"
  st_run 'quotes: P-23 is gone' 1 'P-23: 0 rows' "BETA=$T/q-gone.md" "R=$R" -- quotes
  if [ "$ST_K" = 0 ]; then echo "SELFTEST OK ($ST_N cases)"; return 0; fi
  echo "SELFTEST FAILED ($ST_K of $ST_N)"
  return 1
}

# ----------------------------------------------------------------- main ----

usage() { echo "usage: 32-check.sh quotes|all|selftest ..." >&2; exit 2; }
[ "$#" -gt 0 ] || usage
if [ "$1" = selftest ]; then
  [ "$#" = 1 ] || usage
  selftest
  exit $?
fi

RULES=""
for a in "$@"; do
  case "$a" in
    quotes) RULES="$RULES $a" ;;
    all) RULES="$RULES quotes" ;;
    *) usage ;;
  esac
done

N=0
K=0
HARD=0
for rule in $RULES; do
  N=$((N + 1))
  RULE_FAIL=0
  "rule_$rule"
  rc=$?
  [ "$rc" = 2 ] && HARD=1
  if [ "$rc" != 0 ] || [ "$RULE_FAIL" != 0 ]; then K=$((K + 1)); fi
done
if [ "$K" = 0 ]; then
  echo "32-CHECK OK ($N rules)"
  exit 0
fi
echo "32-CHECK FAILED ($K of $N)"
[ "$HARD" = 1 ] && exit 2
exit 1
