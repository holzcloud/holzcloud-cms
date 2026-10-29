#!/usr/bin/env bash
# Phase 28 (the promise in writing): the acceptance checks of the phase, in one place.
#
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh beta [structure|baseline|ledger|quotes|coverage|nonpromise|srv|evidence]
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh owner-steps [steps|complete]
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh changelog [extract|topics|go]
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh extract RECORD REF
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh frozen
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh regression
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh selftest
#   bash .planning/phases/28-the-promise-in-writing/28-check.sh all
#
# `extract` prints the text of one step of a record exactly as the owner-steps check
# compares it (REF is O-1, O-2 ... for the timer and native records, "item 1" for the
# consistency and the robust record, "step 2" for the archived 0.1.0 owner steps), so that a verbatim
# region is copied by a program and not retyped.
#
# `beta`, `owner-steps` and `changelog` with no second word run every sub-check of theirs.
# The sub-checks exist so that a plan can run the ones its own work has made true; `all` runs
# them all. GO=0 leaves out `changelog go` (the Go tests of the changelog screens) and
# NATIVE_SELFTEST=0 leaves out the 70 second selftest of native-amd64.sh in `regression`.
#
# What a promise is. A row of the promises table (P-n) names one record and cites words
# from it in the last cell. A cited word is a backticked token or a sentence between
# straight double quotes. Every token must be found by `grep -F` in the record, byte for
# byte. The rows after P-10 (the ones this phase moves) must cite at least one whole
# sentence: it ends with a full stop, begins where a sentence of the record begins and
# stops where one stops. That is the whole meaning of "a sentence of a record proves it,
# and a check proves the sentence is quoted byte for byte".
#
# What the page may not do. While a Status line of deploy/NATIVE-AMD64-RUN.md reads
# `pending owner`, no promise cites that record or names that requirement, and the
# does-not-promise section names it with its reason and an owner step. When the record
# says `recorded` this check fails on purpose: the page is stale, and the line moves by
# the hand-over rule of that record.
#
# Every check reads. The only commands that run are the git commands listed in
# EVIDENCE_CMDS below (the page must show them; this file runs its own copy), the lines
# that release.yml itself holds for the version and the notes, and, in `regression` and
# `changelog go`, the earlier checkers and `go test`.
#
# Exit status: 0 all checks passed, 1 at least one check failed, 2 a check could not be run
# (not a git checkout, a file is missing). bash 3.2 and BWK awk (macOS): no mapfile, no
# associative arrays, no gawk extensions, no sed \n, no GNU address forms.
set -u
export LC_ALL=C

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(git -C "$HERE" rev-parse --show-toplevel 2>/dev/null) || { echo "not inside a git checkout" >&2; exit 2; }
cd "$ROOT" || exit 2

BETA=${BETA:-docs/beta.md}
OWNER=${OWNER_STEPS:-$HERE/28-OWNER-STEPS.md}
CHANGELOG=${CHANGELOG_FILE:-CHANGELOG.md}
RELEASE_YML=${RELEASE_YML:-.github/workflows/release.yml}
NATIVE_REC=${NATIVE_REC:-deploy/NATIVE-AMD64-RUN.md}
BASELINE_REV=${BASELINE_REV:-f1ba5a2}
BASELINE_FILE=${BASELINE_FILE:-}
ARCHIVE23=${ARCHIVE23:-.planning/milestones/v0.1.0-phases/23-repository-tidy-and-beta-criteria/23-OWNER-STEPS.md}
OWNER27=${OWNER27:-.planning/phases/27-repository-tidy/27-OWNER-STEPS.md}
TIMER_REC=deploy/BACKUP-TIMER-RUN.md
CON_REC=deploy/BACKUP-CONSISTENCY-RUN.md
ROB_REC=deploy/BACKUP-ROBUST-RUN.md
NATIVE_PATH=deploy/NATIVE-AMD64-RUN.md
RECORDS="deploy/SECURITY-SCORE.md deploy/DEPLOY-RUN.md deploy/BACKUP-RUN.md $TIMER_REC $CON_REC"
FAIL=0
NFAIL=0
TMPROOT=$(mktemp -d) || exit 2

cleanup() { rm -rf "$TMPROOT"; }
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; FAIL=1; NFAIL=$((NFAIL + 1)); }
ok() { echo "ok: $*"; }
mktmp() { mktemp -d "$TMPROOT/d.XXXXXX"; }
need() { [ -f "$1" ] || { echo "missing file: $1" >&2; exit 2; }; }

# The git commands the evidence section must show, and that this file runs itself. The two
# program-identity commands are pinned to 85fa486, the head of main when phase 28 began, so
# that they stay true whatever is merged later; the other three name HEAD on purpose, because
# their claim is that nobody has touched the bound files.
EVIDENCE_CMDS=(
  "git diff --quiet bf74496 HEAD -- deploy/backup.sh deploy/restore.sh deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh"
  "git diff --quiet b501b1b HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh"
  "git diff --quiet e86d490 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh"
  "git diff --quiet b501b1b 85fa486 -- . ':!.planning' ':!docs' ':!deploy' ':!.github'"
  "git diff --quiet e86d490 85fa486 -- . ':!.planning' ':!docs' ':!deploy' ':!.github'"
)

# ------------------------------------------------------------- helpers ----

baseline_text() {
  if [ -n "$BASELINE_FILE" ]; then cat "$BASELINE_FILE"; else git show "$BASELINE_REV:docs/beta.md"; fi
}

# section LEVEL HEADING-ERE : stdin -> the lines under the heading, up to the next heading of
# the same or a higher level. Fences are respected.
section() {
  awk -v lvl="$1" -v re="$2" '
    function hl(s) { return match(s, /^#+/) ? RLENGTH : 0 }
    /^```/ { if (on) print; fence = !fence; next }
    fence { if (on) print; next }
    /^#+ / {
      n = hl($0)
      if (on && n <= lvl) exit
      if (!on && n == lvl && $0 ~ re) { on = 1; next }
    }
    on { print }
  '
}

# headings LEVEL : stdin -> the headings of exactly that level, fences respected.
headings() {
  awk -v lvl="$1" '
    function hl(s) { return match(s, /^#+/) ? RLENGTH : 0 }
    /^```/ { fence = !fence; next }
    fence { next }
    /^#+ / { if (hl($0) == lvl) print }
  '
}

# table_rows : stdin -> the data rows of the tables in it (header and separator dropped).
table_rows() { awk '!/^\|/ { n = 0; next } { n++; if (n > 2) print }'; }

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

# sentence_ok RECORD QUOTE : the quote is one whole sentence of a line of the record.
sentence_ok() {
  Q="$2" awk '
    BEGIN { q = ENVIRON["Q"]; ql = length(q) }
    { i = index($0, q)
      if (i > 0) {
        before = substr($0, 1, i - 1); after = substr($0, i + ql, 1)
        if ((before == "" || before == "- " || before ~ /(\. |\*\* )$/) && (after == "" || after == " ")) found = 1
      } }
    END { exit found ? 0 : 1 }' "$1"
}

is_record() { local r; for r in $RECORDS; do [ "$r" = "$1" ] && return 0; done; return 1; }

# native_statuses : "SRV-01 pending owner" lines, from the Status line under each SRV heading.
native_statuses() {
  awk '/^## SRV-0[123]:/ { cur = substr($2, 1, 6) } /^Status: / { if (cur != "") { s = $0; sub(/^Status: /, "", s); print cur, s; cur = "" } }' "$NATIVE_REC"
}
native_pending() { native_statuses | grep -q "^$1 pending owner$"; }
native_any_pending() { native_statuses | grep -q ' pending owner$'; }

page_version() { sed -n 1p "$BETA" 2> /dev/null | sed -nE 's/^# What ([0-9]+\.[0-9]+\.[0-9]+) promises.*/\1/p'; }
page_version_or_default() { local v; v=$(page_version); printf '%s' "${v:-0.1.1}"; }
baseline_version() { baseline_text | sed -n 1p | sed -nE 's/^# What ([0-9]+\.[0-9]+\.[0-9]+) promises.*/\1/p'; }

# ---------------------------------------------------------------- beta ----

beta_parts() {
  PS=$(mktmp); NP=$(mktmp); ES=$(mktmp); LS=$(mktmp); BN=$(mktmp)
  section 2 '^## What [0-9.]+ promises$' < "$BETA" > "$PS/s"
  section 2 '^## What [0-9.]+ does not promise$' < "$BETA" > "$NP/s"
  section 2 '^## What the evidence is bound to$' < "$BETA" > "$ES/s"
  section 2 '^## What changed since [0-9.]+$' < "$BETA" > "$LS/s"
  baseline_text | section 2 '^## What [0-9.]+ does not promise$' > "$BN/s"
  grep -E '^- ' "$BN/s" | sed 's/^- //' > "$BN/bullets"
  grep -E '^- ' "$NP/s" | sed 's/^- //' > "$NP/bullets"
  table_rows < "$LS/s" > "$LS/rows"
}

# kept_by BULLET : does a bullet of the page begin with this baseline bullet?
kept_by() {
  local B="$1" C
  while IFS= read -r C; do
    if [ "$C" = "$B" ] || [[ "$C" == "$B "* ]]; then return 0; fi
  done < "$NP/bullets"
  return 1
}

check_beta_structure() {
  need "$BETA"
  local n0=$NFAIL v bv want got al w
  v=$(page_version); bv=$(baseline_version)
  [ -n "$v" ] || { fail "$BETA: line 1 is not '# What X.Y.Z promises, and what it does not'"; return; }
  sed -n 1p "$BETA" | grep -qx "# What $v promises, and what it does not" || fail "$BETA: the title line is wrong"
  want=$(printf '%s\n' '## What beta means here' "## What $v promises" "## What $v does not promise" "## What changed since $bv" '## What the evidence is bound to' '## Where the word alpha still appears')
  got=$(headings 2 < "$BETA")
  [ "$got" = "$want" ] || fail "$BETA: the level-two headings are not, in order: $(printf '%s' "$want" | tr '\n' '|')"
  want=$(printf '%s\n' '### The nightly backup and a restore' '### Install, update and rollback' '### The unit and the platform' '### The product itself')
  got=$(section 2 '^## What [0-9.]+ does not promise$' < "$BETA" | headings 3)
  [ "$got" = "$want" ] || fail "$BETA: the groups under the does-not-promise section are not, in order: $(printf '%s' "$want" | tr '\n' '|')"
  section 2 '^## What beta means here$' < "$BETA" | grep -qF 'not feature-frozen' || fail "$BETA: 'not feature-frozen' is gone from the beta paragraph"
  section 2 '^## What beta means here$' < "$BETA" | grep -qF 'not promised stable' || fail "$BETA: 'not promised stable' is gone from the beta paragraph"
  al=$(section 2 '^## Where the word alpha still appears$' < "$BETA")
  for w in IsAlpha 'deploy/DEPLOY.md' '1.0'; do printf '%s\n' "$al" | grep -qF -- "$w" || fail "$BETA: the alpha section lost '$w'"; done
  ! grep -qF 'TODO' "$BETA" || fail "$BETA: an unfinished-work marker"
  [ "$NFAIL" = "$n0" ] && ok "beta structure: title $v, six headings, four groups, the beta and alpha paragraphs"
}

check_beta_baseline() {
  need "$BETA"; beta_parts
  local n0=$NFAIL r B nr=0 nb=0 kept moved L
  while IFS= read -r r; do
    nr=$((nr + 1))
    grep -Fxq -- "$r" "$BETA" || fail "baseline row changed or gone: $(printf '%s' "$r" | cut -c1-90)"
  done < <(baseline_text | grep -E '^\| (P|E)-[0-9]+ \|')
  cut -d'|' -f2 "$LS/rows" | sed 's/^ *//; s/ *$//' > "$LS/col1"
  while IFS= read -r B; do
    nb=$((nb + 1)); kept=0; moved=0
    kept_by "$B" && kept=1
    while IFS= read -r L; do [ "$L" = "$B" ] && { moved=1; break; }; done < "$LS/col1"
    if [ $kept = 0 ] && [ $moved = 0 ]; then fail "a baseline limit is neither kept nor in the ledger: $(printf '%s' "$B" | cut -c1-90)"; fi
    if [ $kept = 1 ] && [ $moved = 1 ]; then fail "a baseline limit is both kept and in the ledger: $(printf '%s' "$B" | cut -c1-90)"; fi
  done < "$BN/bullets"
  [ "$NFAIL" = "$n0" ] && ok "baseline $BASELINE_REV: $nr rows unchanged, $nb limits each kept (as the start of a bullet) or moved (quoted in the ledger)"
}

check_beta_ledger() {
  need "$BETA"; beta_parts
  local n0=$NFAIL row c1 c2 c3 n=0 p
  [ -s "$LS/s" ] || { fail "no section 'What changed since ...'"; return; }
  grep -E '^\| P-[0-9]+ \|' "$PS/s" | awk -F'|' '{ x = $2; gsub(/ /, "", x); print x }' > "$LS/pids"
  while IFS= read -r row; do
    n=$((n + 1))
    c1=$(cell "$row" 1); c2=$(cell "$row" 2); c3=$(cell "$row" 3)
    grep -Fxq -- "$c1" "$BN/bullets" || fail "ledger row quotes something the baseline does not say: $(printf '%s' "$c1" | cut -c1-80)"
    [ "${#c2}" -ge 20 ] || fail "ledger row without an answer: $(printf '%s' "$c1" | cut -c1-60)"
    if printf '%s' "$c3" | grep -qE 'P-[0-9]+'; then
      for p in $(printf '%s' "$c3" | grep -oE 'P-[0-9]+'); do
        grep -qx "$p" "$LS/pids" || fail "the ledger names $p, which is not a promise row"
      done
    else
      [ "$c3" = 'none (reworded)' ] || fail "the third cell of a ledger row is neither P-n nor 'none (reworded)': $c3"
    fi
  done < "$LS/rows"
  [ "$n" -ge 1 ] || fail "the ledger has no row"
  [ "$NFAIL" = "$n0" ] && ok "ledger: $n rows, each quotes a baseline limit and names real promise rows"
}

# One P or E row: its record, its tokens, its quotes.
row_check() { # ROW KIND(P|E)
  local row="$1" kind="$2" id rec num q t nq=0 nt=0 quoted rest
  id=$(cell "$row" 1); rec=$(cell "$row" 3 | tr -d '` ')
  num=${id#*-}
  if [ "$rec" = "$NATIVE_PATH" ]; then
    if native_any_pending; then fail "$id cites $NATIVE_PATH while a requirement there is pending owner"; fi
  elif ! is_record "$rec"; then fail "$id: the record '$rec' is not one of the cited records"; return; fi
  [ -f "$rec" ] || { fail "$id: $rec does not exist"; return; }
  quoted=$(cell "$row" 4)
  if ! quotes_of "$quoted" > /dev/null; then fail "$id: the double quotes of the cited words do not pair up"; return; fi
  while IFS= read -r q; do
    [ -n "$q" ] || continue
    nq=$((nq + 1))
    grep -qF -- "$q" "$rec" || fail "$id: not in $rec, byte for byte: $(printf '%s' "$q" | cut -c1-90)"
    if [ "$kind" = P ] && [ "$num" -gt 10 ]; then
      case "$q" in *.) ;; *) fail "$id: a quoted sentence must end with a full stop: $(printf '%s' "$q" | cut -c1-60)" ;; esac
      sentence_ok "$rec" "$q" || fail "$id: the quote is not one whole sentence of $rec: $(printf '%s' "$q" | cut -c1-70)"
    fi
  done < <(quotes_of "$quoted")
  rest=$(unquoted_of "$quoted")
  while IFS= read -r t; do
    [ -n "$t" ] || continue
    nt=$((nt + 1))
    grep -qF -- "$t" "$rec" || fail "$id: not in $rec, byte for byte: $(printf '%s' "$t" | cut -c1-90)"
  done < <(ticks_of "$rest")
  [ $((nq + nt)) -gt 0 ] || fail "$id: no cited words"
  if [ "$kind" = P ] && [ "$num" -gt 10 ] && [ "$nq" = 0 ]; then fail "$id: a promise moved by this phase cites no whole sentence"; fi
}

check_beta_quotes() {
  need "$BETA"; beta_parts
  local n0=$NFAIL row n=0 prev=10 num id s
  while IFS= read -r row; do
    n=$((n + 1)); row_check "$row" P
    id=$(cell "$row" 1); num=${id#P-}
    if [ "$num" -gt 10 ]; then
      [ "$num" = $((prev + 1)) ] || fail "$id: the new promise rows are not numbered one after another from P-11"
      prev=$num
    fi
  done < <(grep -E '^\| P-[0-9]+ \|' "$PS/s")
  [ "$n" -ge "$(baseline_text | grep -cE '^\| P-[0-9]+ \|')" ] || fail "fewer promise rows than the baseline has"
  grep -qE '^\| P-1[1-9] \|' "$PS/s" || fail "no promise row after P-10: nothing moved"
  for s in SRV-01 SRV-02 SRV-03; do
    if native_pending "$s" && grep -qF "$s" "$PS/s"; then fail "$s is pending owner and appears in the promises section"; fi
  done
  [ "$NFAIL" = "$n0" ] && ok "quotes: $n promise rows, every cited word found byte for byte, every moved row cites a whole sentence"
}

check_beta_coverage() {
  need "$BETA"; beta_parts
  local n0=$NFAIL r w p
  for r in BKP-01 BKP-02 BKP-03 BKP-04 BKP-05 CON-01 CON-02 CON-03; do
    grep -qF "$r" "$PS/s" || fail "the promises section does not mention $r"
  done
  for w in 'window' 'overlaps-snapshot-step'; do grep -qF -- "$w" "$PS/s" || fail "the promises section lacks the word '$w' (CON-01 names the window and the run)"; done
  cut -d'|' -f4 "$LS/rows" > "$LS/c3"
  for p in $(grep -E '^\| P-[0-9]+ \|' "$PS/s" | awk -F'|' '{ x = $2; gsub(/ /, "", x); print x }'); do
    [ "${p#P-}" -gt 10 ] || continue
    grep -qw -- "$p" "$LS/c3" || fail "$p is not named by any ledger row: a moved line must say which 0.1.0 limit it answers"
  done
  [ "$NFAIL" = "$n0" ] && ok "coverage: BKP-01 to BKP-05 and CON-01 to CON-03 are in the promises, the window is named, every moved row is in the ledger"
}

check_beta_nonpromise() {
  need "$BETA"; need "$OWNER"; beta_parts
  local n0=$NFAIL b n=0 recs rec q w lines
  lines=$(awk '/^### /{ f = 1; next } f && $0 != "" && $0 !~ /^- / { print NR ": " substr($0, 1, 60) }' "$NP/s")
  [ -z "$lines" ] || fail "does-not-promise: lines that are not bullets under a group (a bullet is one line): $lines"
  while IFS= read -r b; do
    n=$((n + 1))
    printf '%s\n' "$b" | grep -qE ' Why: .{15,}' || fail "no reason (' Why: ' and a sentence): $(printf '%s' "$b" | cut -c1-80)"
    if printf '%s\n' "$b" | grep -q '"'; then
      recs=$(printf '%s\n' "$b" | grep -oE 'Record: deploy/[A-Za-z0-9-]+\.md' | sed 's/^Record: //' | sort -u)
      if [ "$(printf '%s\n' "$recs" | grep -c .)" != 1 ]; then fail "a bullet with quotes needs exactly one 'Record: deploy/X.md': $(printf '%s' "$b" | cut -c1-70)"; continue; fi
      rec=$recs
      [ -f "$rec" ] || { fail "Record $rec does not exist"; continue; }
      quotes_of "$b" > /dev/null || { fail "unpaired quotes: $(printf '%s' "$b" | cut -c1-70)"; continue; }
      while IFS= read -r q; do
        [ -n "$q" ] || continue
        grep -qF -- "$q" "$rec" || fail "not in $rec, byte for byte: $(printf '%s' "$q" | cut -c1-90)"
      done < <(quotes_of "$b")
    fi
    for q in $(printf '%s\n' "$b" | grep -oE 'OS-[0-9]+'); do
      grep -qE "^### $q: " "$OWNER" || fail "names $q, which $OWNER does not have: $(printf '%s' "$b" | cut -c1-60)"
    done
    # a limit no baseline limit begins is new: it names the record it comes from and quotes it
    if ! kept_by_prefix "$b"; then
      printf '%s\n' "$b" | grep -qE 'Record: deploy/[A-Za-z0-9-]+\.md' || fail "a new limit names no record: $(printf '%s' "$b" | cut -c1-80)"
      printf '%s\n' "$b" | grep -q '"' || fail "a new limit quotes no sentence of its record: $(printf '%s' "$b" | cut -c1-80)"
    fi
  done < "$NP/bullets"
  for w in 'atomic' 'status 24' 'MANIFEST' '404' 'DEPLOY.md'; do
    grep -qF -- "$w" "$NP/s" || fail "does-not-promise lacks the word '$w'"
  done
  [ "$NFAIL" = "$n0" ] && ok "does-not-promise: $n limits, each with a reason, every quote found, every owner step exists, every new limit traced to its record"
}
# kept_by_prefix BULLET : does this page bullet begin with a baseline bullet?
kept_by_prefix() {
  local C="$1" B
  while IFS= read -r B; do
    if [ "$C" = "$B" ] || [[ "$C" == "$B "* ]]; then return 0; fi
  done < "$BN/bullets"
  return 1
}

check_beta_srv() {
  need "$BETA"; need "$OWNER"; need "$NATIVE_REC"; beta_parts
  local n0=$NFAIL s m b src os n
  [ -n "$(native_statuses)" ] || { fail "cannot read the Status lines of $NATIVE_REC"; return; }
  for s in SRV-01 SRV-02 SRV-03; do
    m=${s#SRV-0}
    if native_pending "$s"; then
      grep -F "Pending owner: $s" "$NP/bullets" > "$NP/srv"
      n=$(grep -c . "$NP/srv")
      [ "$n" -ge 1 ] || { fail "$s is pending owner and no does-not-promise line says 'Pending owner: $s'"; continue; }
      while IFS= read -r b; do
        os=""
        for src in $(printf '%s\n' "$b" | grep -oE 'OS-[0-9]+'); do
          grep -A12 "^### $src: " "$OWNER" | grep -qxF "Source: $NATIVE_PATH O-$m" && os=$src
        done
        [ -n "$os" ] || fail "$s: a line names no owner step whose Source is $NATIVE_PATH O-$m: $(printf '%s' "$b" | cut -c1-70)"
        printf '%s\n' "$b" | grep -qF "Record: $NATIVE_PATH" || fail "$s: a line names no 'Record: $NATIVE_PATH': $(printf '%s' "$b" | cut -c1-70)"
      done < "$NP/srv"
    else
      fail "$s: the record says '$(native_statuses | grep "^$s " | cut -d' ' -f2-)': the page is stale; move the line by the hand-over of $NATIVE_PATH and change this check with it"
    fi
  done
  [ "$NFAIL" = "$n0" ] && ok "srv: SRV-01 to SRV-03 are pending owner, stay under does-not-promise with their reason and an owner step, and no promise cites the record"
}

check_beta_evidence() {
  need "$BETA"; beta_parts
  local n0=$NFAIL row c n=0 sha ns
  grep -qF 'made again' "$ES/s" || fail "the evidence section lost the rule words 'made again'"
  for sha in f1428d2 2a11eeb bf74496 b501b1b e86d490 85fa486; do grep -qF "$sha" "$ES/s" || fail "the evidence section does not name $sha"; done
  grep -qF 'bausteine.css' "$ES/s" || fail "the evidence section does not name cmd/holzcloud/assets/bausteine.css, a program file that PR 47 changes after the evidence builds"
  grep -qF 'PR 47' "$ES/s" || fail "the evidence section does not say 'PR 47'"
  while IFS= read -r row; do n=$((n + 1)); row_check "$row" E; done < <(grep -E '^\| E-[0-9]+ \|' "$ES/s")
  [ "$n" -ge "$(baseline_text | grep -cE '^\| E-[0-9]+ \|')" ] || fail "fewer evidence rows than the baseline has"
  grep -E '^\| E-4 \|' "$ES/s" | grep -qF 'Evidence commit: b501b1b' || fail "E-4 does not cite 'Evidence commit: b501b1b'"
  grep -E '^\| E-4 \|' "$ES/s" | grep -qF "$TIMER_REC" || fail "E-4 does not name $TIMER_REC"
  grep -E '^\| E-5 \|' "$ES/s" | grep -qF 'Evidence commit: e86d490' || fail "E-5 does not cite 'Evidence commit: e86d490'"
  grep -E '^\| E-5 \|' "$ES/s" | grep -qF "$CON_REC" || fail "E-5 does not name $CON_REC"
  for c in "${EVIDENCE_CMDS[@]}"; do
    grep -Fxq -- "$c" "$ES/s" || { fail "the evidence section does not show the command: $c"; continue; }
    ( eval "$c" ) || fail "the command exits non-zero: $c"
  done
  ns=$(git diff --numstat bf74496 HEAD -- deploy/DEPLOY.md | awk '{ print $1 " added, " $2 " removed" }')
  if [ "$ns" = "39 added, 0 removed" ]; then
    grep -qF '39 added lines' "$ES/s" || fail "deploy/DEPLOY.md has 39 added lines against bf74496 and the evidence section does not say '39 added lines'"
  else
    fail "deploy/DEPLOY.md against bf74496 is '$ns' now: the page's statement about it has to be re-made, and this check with it"
  fi
  [ "$NFAIL" = "$n0" ] && ok "evidence: E-1 to E-$n, ${#EVIDENCE_CMDS[@]} commands shown and exit 0, the DEPLOY.md difference stated"
}

check_beta() {
  case "${1:-all}" in
    structure) check_beta_structure ;;
    baseline) check_beta_baseline ;;
    ledger) check_beta_ledger ;;
    quotes) check_beta_quotes ;;
    coverage) check_beta_coverage ;;
    nonpromise) check_beta_nonpromise ;;
    srv) check_beta_srv ;;
    evidence) check_beta_evidence ;;
    all) check_beta_structure; check_beta_baseline; check_beta_ledger; check_beta_quotes; check_beta_coverage; check_beta_nonpromise; check_beta_srv; check_beta_evidence ;;
    *) echo "unknown beta check: $1" >&2; exit 2 ;;
  esac
}

# --------------------------------------------------------- owner steps ----

# trim_blank : stdin -> stdout without leading and trailing blank lines.
trim_blank() { awk '{ l[NR] = $0 } END { a = 1; n = NR; while (a <= n && l[a] == "") a++; while (n >= a && l[n] == "") n--; for (i = a; i <= n; i++) print l[i] }'; }

# rec_extract PATH REF : the text of one step of a record or of an earlier owner-steps file, as it stands there.
rec_extract() {
  local p="$1" ref="$2" n v
  case "$p" in
    */BACKUP-TIMER-RUN.md|*/NATIVE-AMD64-RUN.md)
      n=${ref#O-}
      awk -v n="$n" '
        on && (/^\*\*O-[0-9]+\. / || /^\*\*Recording a result\.\*\*/ || /^## /) { exit }
        !on && $0 ~ ("^\\*\\*O-" n "\\. ") { on = 1 }
        on { print }' "$p" | trim_blank ;;
    */BACKUP-CONSISTENCY-RUN.md|*/BACKUP-ROBUST-RUN.md)
      n=${ref#item }
      awk -v n="$n" '/^## Owner steps$/ { on = 1; next } on && /^## / { exit } on && index($0, n ". ") == 1 { print; exit }' "$p" ;;
    */23-OWNER-STEPS.md)
      n=${ref#step }; n=${n%% *}; v=$(page_version_or_default)
      awk -v n="$n" '
        on && /^## / { exit }
        !on && $0 ~ ("^## " n "\\. ") { on = 1; next }
        on { print }' "$p" | trim_blank | sed -e "s/0\\.1\\.0/$v/g" -e 's/after step 1\./after OS-3./' ;;
    */27-OWNER-STEPS.md)
      awk -v id="$ref" '
        on && (/^### / || /^## /) { exit }
        !on && index($0, "### " id ": ") == 1 { on = 1; next }
        on { print }' "$p" | trim_blank ;;
    *) return 1 ;;
  esac
}

# os_split FILE OUTDIR : the body of each "### OS-n: title" step into OUTDIR/OS-n.txt, the ids in order into OUTDIR/ids.
os_split() {
  : > "$2/ids"
  awk -v out="$2" '
    /^```/ { fence = !fence }
    !fence && /^## / { if (cur != "") close(out "/" cur ".txt"); cur = ""; next }
    !fence && /^### OS-[0-9]+: / { if (cur != "") close(out "/" cur ".txt"); cur = $2; sub(/:$/, "", cur); print cur >> (out "/ids") }
    cur != "" { print >> (out "/" cur ".txt") }
  ' "$1"
}

os_source() { grep -m1 '^Source: ' "$1" | sed 's/^Source: //; s/ (.*$//'; }

check_owner_steps_steps() {
  need "$OWNER"
  local n0=$NFAIL T id f src where needs phrase body firstfence regions reg m n=0 w q
  T=$(mktmp); os_split "$OWNER" "$T"
  [ -s "$T/ids" ] || { fail "$OWNER has no '### OS-n: title' step"; return; }
  if grep -Eq 'ghp_|github_pat_|gho_|x-access-token|BEGIN [A-Z ]*PRIVATE KEY' "$OWNER"; then fail "$OWNER looks like it holds a secret"; fi
  if grep -Eq 'git push[^`]*( --force| -f( |$)|--force-with-lease| --delete| :[A-Za-z]| --tags)|git tag ' "$OWNER"; then fail "$OWNER holds a force push, a deletion or a tag command: a published tag is never moved and no step deletes a ref"; fi
  while IFS= read -r id; do
    f="$T/$id.txt"; n=$((n + 1))
    src=$(os_source "$f"); [ -n "$src" ] || { fail "$id: no 'Source:' line"; continue; }
    where=$(grep -m1 '^Where: ' "$f" | sed 's/^Where: //')
    case "$where" in *'development machine'*|*'GitHub UI'*|*'real server'*) ;; *) fail "$id: 'Where:' names none of: development machine, GitHub UI, real server" ;; esac
    needs=$(grep -m1 '^Needs: ' "$f" | sed 's/^Needs: //')
    [ -n "$needs" ] || fail "$id: no 'Needs:' line"
    if [ "$needs" != nothing ]; then
      for m in $(printf '%s\n' "$needs" | grep -oE 'OS-[0-9]+'); do
        grep -qx "$m" "$T/ids" || fail "$id: Needs $m, which does not exist"
        [ "${m#OS-}" -lt "${id#OS-}" ] || fail "$id: Needs $m, which does not come before it"
      done
    fi
    firstfence=$(grep -n '^```' "$f" | head -n 1 | cut -d: -f1)
    [ -n "$firstfence" ] || { fail "$id: no command block (a fenced block)"; continue; }
    body=$(tail -n +"$firstfence" "$f" | grep -v -e '^Expect begins at: ' -e '^A different output is explained at: ')
    for w in 'Expect begins at: ' 'A different output is explained at: '; do
      phrase=$(grep -m1 "^$w" "$f" | sed "s/^$w//")
      if [ -z "$phrase" ]; then fail "$id: no '${w% }' line"; continue; fi
      q=$(quotes_of "$phrase" | head -n 1)
      if ! quotes_of "$phrase" > /dev/null || [ -z "$q" ]; then fail "$id: the phrase of '${w% }' must be one \"quoted\" span: $phrase"; continue; fi
      printf '%s\n' "$body" | grep -qF -- "$q" || fail "$id: the phrase of '${w% }' is not in the step after its first command block: $q"
    done
    if [ "$src" = new ]; then
      grep -qE '^Rehearsal: .{20,}' "$f" || fail "$id: Source is new and there is no 'Rehearsal:' line (date and how, 20 characters at least)"
    else
      regions=$(grep -c '^<!-- verbatim: ' "$f")
      [ "$regions" = 1 ] || { fail "$id: a step with a record as Source needs exactly one verbatim region, has $regions"; continue; }
      reg=$(grep -m1 '^<!-- verbatim: ' "$f" | sed 's/^<!-- verbatim: //; s/ -->$//')
      [ "$reg" = "$src" ] || { fail "$id: the verbatim region names '$reg' and Source says '$src'"; continue; }
      grep -qx '<!-- end verbatim -->' "$f" || { fail "$id: the verbatim region is not closed"; continue; }
      awk '/^<!-- verbatim: /{ on = 1; next } /^<!-- end verbatim -->$/ { on = 0 } on' "$f" | trim_blank > "$T/$id.have"
      rec_extract "$(printf '%s' "$src" | awk '{ print $1 }')" "$(printf '%s' "$src" | cut -d' ' -f2-)" > "$T/$id.want" 2> /dev/null
      if [ ! -s "$T/$id.want" ]; then fail "$id: nothing to compare with: $src"
      elif ! cmp -s "$T/$id.have" "$T/$id.want"; then
        fail "$id: the verbatim region is not the text of $src, byte for byte ($(diff "$T/$id.have" "$T/$id.want" | grep -c '^[<>]') lines differ)"
      fi
    fi
  done < "$T/ids"
  [ "$NFAIL" = "$n0" ] && ok "owner steps: $n steps, each with Source, Where, Needs, a command block, the place of the expected output and of the meaning of a different one; verbatim regions equal their records"
}

check_owner_steps_complete() {
  need "$OWNER"; need "$TIMER_REC"; need "$NATIVE_PATH"; need "$CON_REC"; need "$ROB_REC"
  local n0=$NFAIL T ref k lit idx
  T=$(mktmp); os_split "$OWNER" "$T"
  for k in $(grep -oE '^\*\*O-[0-9]+\.' "$TIMER_REC" | tr -dc '0-9\n'); do echo "$TIMER_REC O-$k"; done > "$T/refs"
  for k in $(grep -oE '^\*\*O-[0-9]+\.' "$NATIVE_PATH" | tr -dc '0-9\n'); do echo "$NATIVE_PATH O-$k"; done >> "$T/refs"
  for k in $(awk '/^## Owner steps$/ { on = 1; next } on && /^## / { exit } on && /^[0-9]+\. / { sub(/\..*/, ""); print }' "$CON_REC"); do echo "$CON_REC item $k"; done >> "$T/refs"
  for k in $(awk '/^## Owner steps$/ { on = 1; next } on && /^## / { exit } on && /^[0-9]+\. / { sub(/\..*/, ""); print }' "$ROB_REC"); do echo "$ROB_REC item $k"; done >> "$T/refs"
  while IFS= read -r ref; do
    grep -lx "Source: $ref" "$T"/OS-*.txt > /dev/null 2>&1 || fail "no owner step has 'Source: $ref'"
  done < "$T/refs"
  for lit in 'git fetch origin' 'git push origin main' 'git push origin main:release' 'gh workflow run native-amd64.yml' 'git ls-remote --tags origin v' 'gh release view v' 'git bundle verify' 'shasum -a 256' 'Actions'; do
    grep -qF -- "$lit" "$OWNER" || fail "$OWNER does not contain '$lit'"
  done
  idx=$(grep -E '^\| OS-[0-9]+ \|' "$OWNER" | awk -F'|' '{ x = $2; gsub(/ /, "", x); print x }')
  [ "$idx" = "$(cat "$T/ids")" ] || fail "the index table at the top lists a different set of steps than the headings"
  [ "$(grep -c . "$T/ids")" -ge 21 ] || fail "fewer than 21 owner steps"
  [ "$NFAIL" = "$n0" ] && ok "owner steps complete: every step of the four records, the push of main, the button, the release and the bundle are here, and the index equals the headings"
}

check_owner_steps() {
  case "${1:-all}" in
    steps) check_owner_steps_steps ;;
    complete) check_owner_steps_complete ;;
    all) check_owner_steps_steps; check_owner_steps_complete ;;
    *) echo "unknown owner-steps check: $1" >&2; exit 2 ;;
  esac
}

# ----------------------------------------------------------- changelog ----

# release_snippets DIR : writes DIR/version.sh (the version line) and DIR/notes.sh (the notes step), both taken from release.yml itself.
release_snippets() {
  grep -F 'version=$(grep -m1' "$RELEASE_YML" | head -n 1 | sed 's/^ *//' > "$1/version.sh"
  awk '
    /- name: Write the release notes from the CHANGELOG/ { f = 1; next }
    f && !on && /^ *run: \|$/ { match($0, /^ */); base = RLENGTH; on = 1; next }
    on {
      if ($0 == "") { print ""; next }
      match($0, /^ */)
      if (RLENGTH <= base) exit
      if (ind == "") ind = RLENGTH
      print substr($0, ind + 1)
    }' "$RELEASE_YML" > "$1/notes.sh"
}

# run_release_notes DIR : the two snippets of release.yml, run on a copy of the changelog in DIR; sets REL_VERSION, writes DIR/release-notes.md.
run_release_notes() {
  cp "$CHANGELOG" "$1/CHANGELOG.md"
  REL_VERSION=$( cd "$1" && eval "$(cat version.sh)" && printf '%s' "$version" )
  [ -n "$REL_VERSION" ] || return 1
  ( cd "$1" && TAG="v$REL_VERSION" bash -c "$(cat notes.sh)" > run.out 2> run.err )
}

check_changelog_extract() {
  need "$CHANGELOG"; need "$RELEASE_YML"
  local n0=$NFAIL T want head notes
  T=$(mktmp); release_snippets "$T"
  [ -s "$T/version.sh" ] || { fail "cannot find the version line in $RELEASE_YML"; return; }
  [ -s "$T/notes.sh" ] || { fail "cannot find the notes step in $RELEASE_YML"; return; }
  run_release_notes "$T" || { fail "release.yml's own lines fail on $CHANGELOG (version '${REL_VERSION:-}'): $(head -c 200 "$T/run.err" 2> /dev/null)"; return; }
  want=$(page_version_or_default)
  [ "$REL_VERSION" = "$want" ] || fail "release.yml's own line reads $REL_VERSION from $CHANGELOG; the page says $want"
  head=$(grep -m1 '^## ' "$CHANGELOG")
  printf '%s\n' "$head" | grep -qE '^## [0-9]+\.[0-9]+\.[0-9]+ — [0-9]{4}-[0-9]{2}-[0-9]{2}$' || fail "the first heading is not '## X.Y.Z — YYYY-MM-DD': $head"
  [ "$(grep '^## ' "$CHANGELOG" | sed -n 2p)" = "## 0.1.0 — 2026-10-14" ] || fail "the heading after the new entry is not '## 0.1.0 — 2026-10-14'"
  notes="$T/release-notes.md"
  [ -s "$notes" ] || { fail "release.yml's notes step wrote no notes for $REL_VERSION"; return; }
  grep -qF 'https://github.com/holzcloud/holzcloud-cms/blob/main/docs/beta.md' "$notes" || fail "the entry does not link docs/beta.md with an absolute URL"
  [ "$(wc -c < "$notes")" -ge 400 ] || fail "the notes are under 400 bytes"
  [ "$NFAIL" = "$n0" ] && ok "changelog extract: release.yml's own lines read $REL_VERSION and write $(wc -c < "$notes" | tr -d ' ') bytes of notes ($(grep -c '^\*\*' "$notes") bold paragraphs)"
}

check_changelog_topics() {
  need "$CHANGELOG"; need "$RELEASE_YML"
  local n0=$NFAIL T notes w n
  T=$(mktmp); release_snippets "$T"
  run_release_notes "$T" || { fail "release.yml's own lines fail on $CHANGELOG"; return; }
  notes="$T/release-notes.md"
  for w in 'docs/beta.md' 'deploy/DEPLOY.md' 'rsync' 'MANIFEST' 'amd64' 'Bildlage' '2560' 'Kleine Bilder' '/opt/holzcloud/offsite-ssh' 'REMOTE_TARGET' 'Persistent' 'MIN_FREE_MB'; do
    grep -qF -- "$w" "$notes" || fail "the entry does not mention '$w'"
  done
  n=$(grep -c '^\*\*' "$notes"); [ "$n" -ge 6 ] || fail "the entry has $n bold lead paragraphs, expected 6 or more"
  [ "$NFAIL" = "$n0" ] && ok "changelog topics: the findings, the gaps, the two features and the owner's part are in the entry"
}

check_changelog_go() {
  if [ "${GO:-1}" = 0 ]; then echo "skipped: GO=0"; return; fi
  command -v go > /dev/null || { echo "go not found" >&2; exit 2; }
  local n0=$NFAIL
  go test ./internal/changelog/ -count=1 > /dev/null 2>&1 || fail "go test ./internal/changelog/"
  go test . -run 'Changelog' -count=1 > /dev/null 2>&1 || fail "go test . -run Changelog"
  go test ./internal/admin -run 'Changelog' -count=1 > /dev/null 2>&1 || fail "go test ./internal/admin -run Changelog"
  [ "$NFAIL" = "$n0" ] && ok "changelog go: the parser and the screens read the new entry"
}

check_changelog() {
  case "${1:-all}" in
    extract) check_changelog_extract ;;
    topics) check_changelog_topics ;;
    go) check_changelog_go ;;
    all) check_changelog_extract; check_changelog_topics; check_changelog_go ;;
    *) echo "unknown changelog check: $1" >&2; exit 2 ;;
  esac
}

# --------------------------------------------------------------- frozen ----

# Phase 28 edits docs/beta.md, CHANGELOG.md, its own files and nothing else. Everything the earlier evidence binds stays.
check_frozen() {
  local n0=$NFAIL
  git diff --quiet 05fc076 HEAD -- deploy/backup.sh deploy/restore.sh deploy/DEPLOY.md deploy/holzcloud.service deploy/holzcloud-backup.service deploy/holzcloud-backup.timer deploy/test/run.sh deploy/test/nightly.sh deploy/SECURITY-SCORE.md deploy/DEPLOY-RUN.md deploy/BACKUP-RUN.md deploy/BACKUP-TIMER-RUN.md deploy/BACKUP-CONSISTENCY-RUN.md || fail "a bound file or an earlier record changed after 05fc076"
  git diff --quiet 7d20710 HEAD -- deploy/test/native-amd64.sh deploy/NATIVE-AMD64-RUN.md .github/workflows/native-amd64.yml .github/workflows/release.yml || fail "a phase 26 file or release.yml changed after 7d20710"
  git diff --quiet 85fa486 HEAD -- . ':!.planning' ':!docs' ':!CHANGELOG.md' || fail "something outside .planning, docs and CHANGELOG.md changed after 85fa486"
  git diff --quiet 85fa486 HEAD -- docs ':!docs/beta.md' || fail "a page of docs other than beta.md changed after 85fa486"
  [ "$NFAIL" = "$n0" ] && ok "frozen: the bound files, the earlier records, release.yml, the phase 26 instrument and the whole program are as they were"
}

# ----------------------------------------------------------- regression ----

check_regression() {
  local n0=$NFAIL out
  out=$(bash deploy/test/nightly.sh selftest 2>&1 | tail -n 3); printf '%s\n' "$out" | grep -qF 'SELFTEST OK (542 cases)' || fail "nightly.sh selftest: $out"
  out=$(bash deploy/test/nightly.sh record-check 2>&1 | tail -n 1); [ "$out" = 'RECORD-CHECK OK 29 blocks' ] || fail "nightly.sh record-check: $out"
  out=$(E="$ROOT/deploy/test/out" bash deploy/test/nightly.sh record-check-con 2>&1 | tail -n 1); [ "$out" = 'RECORD-CHECK OK 27 blocks' ] || fail "nightly.sh record-check-con: $out"
  out=$(bash deploy/test/native-amd64.sh status-check 2>&1 | tail -n 1); case "$out" in 'STATUS-CHECK OK'*) ;; *) fail "native-amd64.sh status-check: $out" ;; esac
  out=$(bash deploy/test/native-amd64.sh record-check 2>&1 | tail -n 1); case "$out" in 'RECORD-CHECK OK 0 blocks; recorded: none; pending owner: SRV-01 SRV-02 SRV-03') ;; *) fail "native-amd64.sh record-check: $out" ;; esac
  if [ "${NATIVE_SELFTEST:-1}" = 1 ]; then
    out=$(bash deploy/test/native-amd64.sh selftest 2>&1 | tail -n 1); [ "$out" = 'SELFTEST OK (862 cases)' ] || fail "native-amd64.sh selftest: $out"
  fi
  [ "$NFAIL" = "$n0" ] && ok "regression: nightly.sh selftest 542, record-check 29, record-check-con 27, native-amd64.sh status-check and record-check$( [ "${NATIVE_SELFTEST:-1}" = 1 ] && echo ', selftest 862')"
}

# ------------------------------------------------------------- selftest ----
# The checks, played on small pages made here and on copies of them that are wrong in one way.
# The records are the real ones; the page, the baseline, the owner steps and the changelog are fixtures.

fx_baseline() {
  cat > "$1" <<'EOF'
# What 0.1.0 promises, and what it does not

## What beta means here

Beta is not feature-frozen and the schema is not promised stable.

## What 0.1.0 promises

| ID | Promise | Record | Cited words |
|----|---------|--------|-------------|
| P-1 | The unit starts. | `deploy/SECURITY-SCORE.md` | `UNIT-01: the unit starts and the site answers` |

## What 0.1.0 does not promise

### The nightly backup and a restore

- The real 03:15 window was not exercised: the timer was moved a minute ahead, so the shipped calendar entry never fired.
- A restore on a real server was not made: every restore went into a throwaway container.
- Bit rot in a complete backup is not caught.

### Install, update and rollback

- A real VPS image was not played.

### The unit and the platform

- The unit was not tuned.

### The product itself

- Features may change.

## What the evidence is bound to

| ID | Evidence | Record | Cited words |
|----|----------|--------|-------------|
| E-1 | Unit measurements. | `deploy/SECURITY-SCORE.md` | `2026-10-12, f1428d2` |

## Where the word alpha still appears

IsAlpha in deploy/DEPLOY.md, until 1.0.
EOF
}

fx_page() {
  local q1='The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole.' c
  {
    cat <<'EOF'
# What 0.1.1 promises, and what it does not

## What beta means here

Beta is not feature-frozen and the schema is not promised stable.

## What 0.1.1 promises

| ID | Promise | Record | Cited words |
|----|---------|--------|-------------|
| P-1 | The unit starts. | `deploy/SECURITY-SCORE.md` | `UNIT-01: the unit starts and the site answers` |
EOF
    echo "| P-11 | BKP-01 BKP-02 BKP-03 BKP-04 BKP-05 CON-01 CON-02 CON-03: the shipped entry fires the shipped service. The window and the run: overlaps-snapshot-step. | \`$TIMER_REC\` | \"$q1\", \`TRIGGERED-BY: holzcloud-backup.timer\` |"
    cat <<'EOF'

## What 0.1.1 does not promise

These are the limits.

### The nightly backup and a restore

- The server's own zone was not used. Why: the zone was moved so that 03:15 arrived within two minutes. Record: deploy/BACKUP-TIMER-RUN.md "The real wall-clock 03:15 on a server's own zone, and daylight saving." Owner step OS-6.
- A restore on a real server was not made: every restore went into a throwaway container. Why: the result is not recorded. Pending owner: SRV-02. Record: deploy/NATIVE-AMD64-RUN.md "Status: pending owner" Owner step OS-13.
- Native amd64 and the reboot were not run. Why: the owner has not pressed the button. Pending owner: SRV-01, Owner step OS-5. Pending owner: SRV-03, Owner step OS-14. Record: deploy/NATIVE-AMD64-RUN.md "Status: pending owner"
- Bit rot in a complete backup is not caught. Why: nothing in this milestone touched it.
- The pair of database and media is not atomic, and a backup refused by rsync status 24 leaves no MANIFEST; a restore shows a 404 for a missing file and DEPLOY.md does not say so. Why: that is the finding. Record: deploy/BACKUP-CONSISTENCY-RUN.md "The documents do not say that a vanished file is such an interruption."

### Install, update and rollback

- A real VPS image was not played. Why: no record of this milestone played the install text.

### The unit and the platform

- The unit was not tuned. Why: measured, not tuned, and that stays.

### The product itself

- Features may change. Why: this is a beta and is not feature-frozen.

## What changed since 0.1.0

| 0.1.0 said | 0.1.1 says | Promise rows |
|------------|------------|--------------|
| The real 03:15 window was not exercised: the timer was moved a minute ahead, so the shipped calendar entry never fired. | The shipped entry fired the shipped service. | P-11 |

## What the evidence is bound to

| ID | Evidence | Record | Cited words |
|----|----------|--------|-------------|
| E-1 | Unit measurements. | `deploy/SECURITY-SCORE.md` | `2026-10-12, f1428d2` |
EOF
    echo "| E-4 | Timer. | \`$TIMER_REC\` | \`Evidence commit: b501b1b\` |"
    echo "| E-5 | Consistency. | \`$CON_REC\` | \`Evidence commit: e86d490\` |"
    echo
    echo "A set is made again when a bound file changes (2a11eeb, bf74496). DEPLOY.md has 39 added lines against bf74496. The program is pinned at 85fa486; PR 47 changes cmd/holzcloud/assets/bausteine.css after it."
    echo
    echo '```text'
    for c in "${EVIDENCE_CMDS[@]}"; do echo "$c"; done
    echo '```'
    cat <<'EOF'

## Where the word alpha still appears

IsAlpha in deploy/DEPLOY.md, until 1.0.
EOF
  } > "$1"
}

fx_owner() { # OUTFILE : an owner-steps file for the real records, steps cut from the records
  local out="$1" n
  {
    echo "# fixture"; echo
    echo "| Step | Where | Needs | Source |"; echo "|---|---|---|---|"
    for n in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21; do echo "| OS-$n | real server | nothing | x |"; done
    echo
  } > "$out"
  step_new() { # id title command
    cat >> "$out" <<EOF
### OS-$1: $2
Source: new
Where: development machine
Needs: nothing
Rehearsal: seen on 2026-10-26 in a throwaway repository, nothing real was touched.

\`\`\`sh
$3
\`\`\`

Expect begins at: "Expected: fine"
A different output is explained at: "A different output means: stop"

Expected: fine
A different output means: stop

EOF
  }
  step_rec() { # id title path ref
    local body; body=$(rec_extract "$3" "$4")
    cat >> "$out" <<EOF
### OS-$1: $2
Source: $3 $4
Where: real server
Needs: nothing
Expect begins at: "Expect"
A different output is explained at: "Expect"

\`\`\`sh
echo command
\`\`\`

<!-- verbatim: $3 $4 -->
$body
<!-- end verbatim -->

Expect and different: see above.

EOF
  }
  step_new 1 'look' 'git fetch origin'
  step_new 2 'integrate' 'git merge origin/main'
  step_new 3 'push' 'git push origin main'
  step_new 4 'ci' 'gh run list'
  step_rec 5 'button' "$NATIVE_PATH" O-1
  step_rec 6 't1' "$TIMER_REC" O-1
  step_rec 7 't2' "$TIMER_REC" O-2
  step_rec 8 't3' "$TIMER_REC" O-3
  step_rec 9 't4' "$TIMER_REC" O-4
  step_rec 10 't5' "$TIMER_REC" O-5
  step_rec 11 'c1' "$CON_REC" 'item 1'
  step_rec 12 'c2' "$CON_REC" 'item 2'
  step_rec 13 'n2' "$NATIVE_PATH" O-2
  step_rec 14 'n3' "$NATIVE_PATH" O-3
  step_new 15 'date' 'grep -m1 "^## " CHANGELOG.md'
  step_new 16 'release' 'git push origin main:release; gh workflow run native-amd64.yml'
  step_new 17 'check' 'git ls-remote --tags origin v0.1.1; gh release view v0.1.1'
  step_new 18 'afterwards' 'git push origin main:release'
  step_new 19 'bundle' 'git bundle verify X; shasum -a 256 X; Actions'
  step_rec 20 'r1' "$ROB_REC" 'item 1'
  step_rec 21 'r2' "$ROB_REC" 'item 2'
}

fx_changelog() { # OUTFILE : the real changelog with a small 0.1.1 entry on top
  # The preamble is everything above the first '## ' heading; the old entries start at the first
  # '## 0.1.0' heading. An entry for 0.1.1 that the real file already carries lies between the two
  # and is left out, so the fixture has exactly one 0.1.1 heading whether the file has one or not.
  local out="$1" pre post; pre=$(grep -n -m1 '^## ' "$CHANGELOG" | cut -d: -f1)
  post=$(grep -n -m1 '^## 0\.1\.0 ' "$CHANGELOG" | cut -d: -f1)
  [ -n "$post" ] || post="$pre"
  {
    head -n $((pre - 1)) "$CHANGELOG"
    echo '## 0.1.1 — 2026-10-26'; echo
    echo 'Dieses Release belegt, was 0.1.0 offen liess. Die Seite [docs/beta.md](https://github.com/holzcloud/holzcloud-cms/blob/main/docs/beta.md) hält es fest, und es steht in ganzen Sätzen hier, damit jemand, der das Programm betreibt, nachlesen kann, was sich verändert hat, wie es sich verändert hat und was nicht gezeigt wurde.'
    echo
    echo '**Zweiter Absatz.** Auch dieser Absatz ist lang genug, damit der Auszug der Release-Notizen nicht kürzer als vierhundert Byte wird und der Test, der darauf achtet, den Eintrag als Eintrag erkennt.'
    echo
    tail -n +"$post" "$CHANGELOG"
  } > "$out"
}

# t_sub OLD NEW : stdin -> stdout, the first occurrence of the literal OLD in each line replaced by NEW.
t_sub() { O="$1" N="$2" awk 'BEGIN { o = ENVIRON["O"]; n = ENVIRON["N"]; ol = length(o) } { i = index($0, o); if (i > 0) $0 = substr($0, 1, i - 1) n substr($0, i + ol); print }'; }
# t_del LITERAL : every line containing the literal is dropped.
t_del() { O="$1" awk 'BEGIN { o = ENVIRON["O"] } index($0, o) == 0 { print }'; }
# t_del_first LITERAL : the first line containing the literal is dropped.
t_del_first() { O="$1" awk 'BEGIN { o = ENVIRON["O"] } !d && index($0, o) > 0 { d = 1; next } { print }'; }
# t_after LITERAL TEXT : a line TEXT after each line that contains the literal.
t_after() { O="$1" N="$2" awk 'BEGIN { o = ENVIRON["O"]; n = ENVIRON["N"] } { print } index($0, o) > 0 { print n }'; }
# t_drop_first_fence : the first fenced block, fences included, is dropped.
t_drop_first_fence() { awk '!d && /^```/ { if (!on) { on = 1; next } on = 0; d = 1; next } on { next } { print }'; }
# t_append TEXT : a last line.
t_append() { N="$1" awk '{ print } END { print ENVIRON["N"] }'; }

starts_with() { case "$1" in "$2"*) return 0 ;; esac; return 1; }
st_expect() { # DESC WANTED-EXIT COMMAND... : run it in a subshell, compare the exit status
  local desc="$1" want="$2" rc; shift 2
  ( "$@" ) > /dev/null 2>&1; rc=$?
  if [ "$rc" = "$want" ]; then SELF_OK=$((SELF_OK + 1)); else echo "SELFTEST FAIL: $desc (exit $rc, wanted $want)" >&2; SELF_FAIL=$((SELF_FAIL + 1)); fi
}
st_fails() { # DESC MESSAGE COMMAND... : exit status 1, and the message among what it wrote to stderr
  local desc="$1" msg="$2" rc err; shift 2
  err=$( ( "$@" ) 2>&1 > /dev/null ); rc=$?
  if [ "$rc" != 1 ]; then echo "SELFTEST FAIL: $desc (exit $rc, wanted 1)" >&2; SELF_FAIL=$((SELF_FAIL + 1))
  elif ! printf '%s\n' "$err" | grep -qF -- "$msg"; then echo "SELFTEST FAIL: $desc (it failed, but not with '$msg': $(printf '%s' "$err" | head -n 2 | tr '\n' '|' | cut -c1-200))" >&2; SELF_FAIL=$((SELF_FAIL + 1))
  else SELF_OK=$((SELF_OK + 1)); fi
}
st_true() { # DESC COMMAND... : the command succeeds
  local desc="$1"; shift
  if "$@" > /dev/null 2>&1; then SELF_OK=$((SELF_OK + 1)); else echo "SELFTEST FAIL: $desc" >&2; SELF_FAIL=$((SELF_FAIL + 1)); fi
}
st_false() { # DESC COMMAND... : the command fails
  local desc="$1"; shift
  if "$@" > /dev/null 2>&1; then echo "SELFTEST FAIL: $desc" >&2; SELF_FAIL=$((SELF_FAIL + 1)); else SELF_OK=$((SELF_OK + 1)); fi
}

selftest() {
  SELF_OK=0; SELF_FAIL=0
  local T ME="$HERE/28-check.sh" P
  T=$(mktmp)
  fx_baseline "$T/base.md"; fx_page "$T/good.md"; fx_owner "$T/os-good.md"; fx_changelog "$T/cl-good.md"
  # the extractors, on the real records
  st_true 'the extractor finds O-1 of the timer record' starts_with "$(rec_extract "$TIMER_REC" O-1 | head -n 1)" '**O-1. The shipped timer at its real 03:15.'
  st_true 'the extractor finds item 2 of the consistency record' test "$(rec_extract "$CON_REC" 'item 2' | cut -c1-30)" = '2. Look for a directory withou'
  st_true 'the extractor finds item 1 of the robust record' test "$(rec_extract "$ROB_REC" 'item 1' | cut -c1-30)" = '1. The first night after the u'
  st_true 'the extractor stops O-3 of the native record before Recording a result' test "$(rec_extract "$NATIVE_PATH" O-3 | tail -n 1 | cut -c1-30)" = 'Paste the output of the last `'
  st_true 'the extractor finds step 2 of the 0.1.0 owner steps' test "$(rec_extract "$ARCHIVE23" 'step 2' | sed -n 2p)" = 'git push origin main:release'
  # sentence_ok: a whole sentence yes, the end of one no, the start of one no
  st_true 'a whole sentence is accepted' sentence_ok "$TIMER_REC" 'The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole.'
  st_false 'the end of a sentence is refused' sentence_ok "$TIMER_REC" 'hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole.'
  st_false 'the start of a sentence is refused' sentence_ok "$TIMER_REC" 'The shipped entry fires the shipped, hardened service without a person'
  # the rows of the real baseline all pass the token rule (the same words 0.1.0 proved with a grep loop)
  P=0; while IFS= read -r r; do
    case "$r" in '| P-'*) ( row_check "$r" P ) > /dev/null 2>&1 || P=$((P + 1)) ;; esac
  done < <(baseline_text | grep -E '^\| (P|E)-[0-9]+ \|')
  st_true 'every promise row of the real baseline passes the token rule' test "$P" = 0

  local B="BASELINE_FILE=$T/base.md"
  for P in structure baseline ledger quotes coverage nonpromise srv evidence; do
    st_expect "beta $P on the good fixture" 0 env "$B" BETA="$T/good.md" OWNER_STEPS="$T/os-good.md" bash "$ME" beta "$P"
  done
  neg() { # DESC SUBCHECK MESSAGE -- TRANSFORM ARGS... : the good page with one edit must fail the sub-check, saying why
    local desc="$1" sub="$2" msg="$3"; shift 4
    "$@" < "$T/good.md" > "$T/bad.md"
    st_fails "$desc" "$msg" env "$B" BETA="$T/bad.md" OWNER_STEPS="$T/os-good.md" bash "$ME" beta "$sub"
  }
  neg 'a quote changed by one letter' quotes 'not in deploy/BACKUP-TIMER-RUN.md, byte for byte' -- t_sub 'hardened service without' 'hardened services without'
  neg 'a quote that starts in the middle of a sentence' quotes 'not one whole sentence' -- t_sub '"The shipped entry fires the shipped, hardened' '"entry fires the shipped, hardened'
  neg 'a quote that does not end in a full stop' quotes 'must end with a full stop' -- t_sub 'is whole."' 'is whole"'
  neg 'a token that is not in the record' quotes 'not in deploy/BACKUP-TIMER-RUN.md, byte for byte' -- t_sub 'TRIGGERED-BY: holzcloud-backup.timer' 'TRIGGERED-BY: nobody'
  neg 'a promise cites the native record while it is pending' quotes 'while a requirement there is pending owner' -- t_sub '| `deploy/BACKUP-TIMER-RUN.md` | "' '| `deploy/NATIVE-AMD64-RUN.md` | "'
  neg 'a pending requirement named among the promises' quotes 'appears in the promises section' -- t_sub 'BKP-01 BKP-02' 'SRV-02 BKP-01 BKP-02'
  neg 'a promise row without a whole sentence' quotes 'cites no whole sentence' -- t_sub '"The shipped entry fires the shipped, hardened service without a person, a drop-in or a manual start, and the backup it leaves is whole.", ' ''
  neg 'a requirement missing from the promises' coverage 'does not mention CON-03' -- t_sub 'CON-03: ' ''
  neg 'the window not named' coverage 'lacks the word '\''overlaps-snapshot-step'\''' -- t_sub 'The window and the run: overlaps-snapshot-step.' ''
  neg 'a baseline limit dropped' baseline 'neither kept nor in the ledger' -- t_del 'Bit rot in a complete backup is not caught.'
  neg 'a baseline limit rewritten' baseline 'neither kept nor in the ledger' -- t_sub 'Bit rot in a complete backup is not caught.' 'Bit rot is rare.'
  neg 'a moved limit also kept' baseline 'both kept and in the ledger' -- t_after '### The nightly backup and a restore' '- The real 03:15 window was not exercised: the timer was moved a minute ahead, so the shipped calendar entry never fired. Why: still true.'
  neg 'a limit without a reason' nonpromise 'no reason' -- t_sub ' Why: nothing in this milestone touched it.' ''
  neg 'a quote that is not in its record' nonpromise 'not in deploy/BACKUP-TIMER-RUN.md, byte for byte' -- t_sub 'The real wall-clock 03:15 on a server' 'The real wall-clock 04:15 on a server'
  neg 'an owner step that does not exist' nonpromise 'names OS-77' -- t_sub 'Owner step OS-6.' 'Owner step OS-77.'
  neg 'a new limit that names no record' nonpromise 'a new limit names no record' -- t_after '### The product itself' '- A made-up limit. Why: nobody knows where it came from.'
  neg 'SRV-02 loses its owner step' srv 'SRV-02: a line names no owner step' -- t_sub 'Owner step OS-13.' ''
  neg 'SRV-03 line gone' srv 'no does-not-promise line says '\''Pending owner: SRV-03'\''' -- t_sub 'Pending owner: SRV-03' 'Pending owner: gone'
  neg 'a ledger row that quotes what 0.1.0 did not say' ledger 'quotes something the baseline does not say' -- t_sub '| The real 03:15 window was not exercised' '| The real 03:15 window was not tried'
  neg 'a ledger row that names a promise that does not exist' ledger 'names P-12' -- t_sub 'shipped service. | P-11 |' 'shipped service. | P-12 |'
  neg 'an evidence command missing' evidence 'does not show the command' -- t_del 'git diff --quiet e86d490 85fa486'
  neg 'E-4 loses its commit' evidence 'E-4 does not cite '\''Evidence commit: b501b1b'\''' -- t_sub 'Evidence commit: b501b1b' 'Evidence commit: 0000000'
  neg 'the version heading differs' structure 'the level-two headings are not' -- t_sub '## What 0.1.1 promises' '## What 0.1.2 promises'
  sed 's/^Status: pending owner$/Status: recorded/' "$NATIVE_PATH" > "$T/native-recorded.md"
  st_fails 'a recorded SRV makes the srv check fail' 'the page is stale' env "$B" BETA="$T/good.md" OWNER_STEPS="$T/os-good.md" NATIVE_REC="$T/native-recorded.md" bash "$ME" beta srv

  st_expect 'owner steps: the good fixture, steps' 0 env OWNER_STEPS="$T/os-good.md" BETA="$T/good.md" bash "$ME" owner-steps steps
  st_expect 'owner steps: the good fixture, complete' 0 env OWNER_STEPS="$T/os-good.md" BETA="$T/good.md" bash "$ME" owner-steps complete
  osneg() { # DESC SUBCHECK MESSAGE -- TRANSFORM ARGS...
    local desc="$1" sub="$2" msg="$3"; shift 4
    "$@" < "$T/os-good.md" > "$T/os-bad.md"
    st_fails "$desc" "$msg" env OWNER_STEPS="$T/os-bad.md" BETA="$T/good.md" bash "$ME" owner-steps "$sub"
  }
  osneg 'a verbatim region with one word changed' steps 'OS-6: the verbatim region is not the text of' -- t_sub 'The shipped timer at its real 03:15.' 'The shipped timer at its real 04:15.'
  osneg 'a step without a command block' steps 'OS-1: no command block' -- t_drop_first_fence
  osneg 'a step without its Where line' steps 'names none of: development machine' -- t_del_first 'Where: real server'
  osneg 'a step that Needs a later step' steps 'does not come before it' -- t_sub 'Needs: nothing' 'Needs: OS-9'
  osneg 'a step without its Source line' steps 'no '\''Source:'\'' line' -- t_del_first 'Source: deploy/BACKUP-TIMER-RUN.md O-2'
  osneg 'a new step without its rehearsal' steps 'no '\''Rehearsal:'\'' line' -- t_del_first 'Rehearsal: '
  osneg 'a step without the place of the expected output' steps 'no '\''Expect begins at:'\'' line' -- t_del_first 'Expect begins at: '
  osneg 'a force push' steps 'holds a force push, a deletion or a tag command' -- t_append 'git push --force origin main'
  osneg 'a secret' steps 'looks like it holds a secret' -- t_append 'ghp_0123456789abcdef'
  osneg 'a record step left out' complete 'no owner step has '\''Source: deploy/BACKUP-TIMER-RUN.md O-5'\''' -- t_sub 'Source: deploy/BACKUP-TIMER-RUN.md O-5' 'Source: deploy/BACKUP-TIMER-RUN.md O-9'
  osneg 'the index lists another set' complete 'the index table at the top lists a different set' -- t_del '| OS-19 |'
  osneg 'a step of the robust record left out' complete 'no owner step has '\''Source: deploy/BACKUP-ROBUST-RUN.md item 1'\''' -- t_sub 'Source: deploy/BACKUP-ROBUST-RUN.md item 1' 'Source: deploy/BACKUP-ROBUST-RUN.md item 9'
  osneg 'the second step of the robust record left out' complete 'no owner step has '\''Source: deploy/BACKUP-ROBUST-RUN.md item 2'\''' -- t_sub 'Source: deploy/BACKUP-ROBUST-RUN.md item 2' 'Source: deploy/BACKUP-ROBUST-RUN.md item 9'
  osneg 'the index lacks OS-20' complete 'the index table at the top lists a different set' -- t_del '| OS-20 |'

  st_expect 'changelog extract: the good fixture' 0 env CHANGELOG_FILE="$T/cl-good.md" BETA="$T/good.md" bash "$ME" changelog extract
  clneg() { # DESC MESSAGE -- TRANSFORM ARGS...
    local desc="$1" msg="$2"; shift 3
    "$@" < "$T/cl-good.md" > "$T/cl-bad.md"
    st_fails "$desc" "$msg" env CHANGELOG_FILE="$T/cl-bad.md" BETA="$T/good.md" bash "$ME" changelog extract
  }
  clneg 'a heading without a date' 'the first heading is not' -- t_sub '## 0.1.1 — 2026-10-26' '## 0.1.1'
  clneg 'a heading that release.yml cannot read' 'release.yml' -- t_sub '## 0.1.1 — 2026-10-26' '## v0.1.1 — 2026-10-26'
  clneg 'a level-two heading inside the entry' 'the heading after the new entry is not' -- t_sub '**Zweiter Absatz.**' '## Zweiter Absatz'
  clneg 'the link to the page gone' 'does not link docs/beta.md' -- t_sub 'blob/main/docs/beta.md' 'blob/main/docs/other.md'
  clneg 'the version differs from the page' 'the page says' -- t_sub '## 0.1.1 — 2026-10-26' '## 0.1.2 — 2026-10-26'

  [ -z "${SELFTEST_KEEP:-}" ] || cp -R "$T/." "$SELFTEST_KEEP/"
  if [ "$SELF_FAIL" = 0 ]; then echo "SELFTEST OK ($SELF_OK cases)"; else echo "SELFTEST FAILED ($SELF_FAIL of $((SELF_OK + SELF_FAIL)) cases)" >&2; FAIL=1; fi
}

# ----------------------------------------------------------------- main ----

cmd=${1:-all}; shift 2> /dev/null || true
case "$cmd" in
  extract) [ $# -eq 2 ] || { echo "usage: $0 extract RECORD-OR-FILE REF" >&2; exit 2; }; rec_extract "$1" "$2" ;;
  beta) check_beta "${1:-all}" ;;
  owner-steps) check_owner_steps "${1:-all}" ;;
  changelog) check_changelog "${1:-all}" ;;
  frozen) check_frozen ;;
  regression) check_regression ;;
  selftest) selftest ;;
  all) check_beta all; check_owner_steps all; check_changelog all; check_frozen; check_regression ;;
  *) echo "usage: $0 beta|owner-steps|changelog|extract|frozen|regression|selftest|all [sub-check]" >&2; exit 2 ;;
esac
[ "$FAIL" = 0 ] && exit 0
exit 1
