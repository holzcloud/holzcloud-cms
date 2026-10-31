#!/usr/bin/env bash
# Phase 27 (repository tidy): the acceptance checks of the phase, in one place.
#
#   bash .planning/phases/27-repository-tidy/27-check.sh branches [--only NAME]
#   bash .planning/phases/27-repository-tidy/27-check.sh owner-steps
#   bash .planning/phases/27-repository-tidy/27-check.sh requirements
#   bash .planning/phases/27-repository-tidy/27-check.sh state
#   bash .planning/phases/27-repository-tidy/27-check.sh archive [live|file|roadmap|audit]
#   bash .planning/phases/27-repository-tidy/27-check.sh all
#
# Every check reads. The only thing that moves is `git fetch origin --prune`
# in `branches` (it updates remote-tracking refs and forgets the ones whose
# branch is gone, it changes no local branch and no remote ref). The mutating
# gsd-tools verb (`state.advance-plan`) is played on a throw-away copy of
# .planning, never on the real tree. Nothing here deletes, pushes or tags.
#
# Exit status: 0 all checks passed, 1 at least one check failed, 2 a check could
# not be run (not a git checkout, fetch failed, tool missing).
#
# The verbatim proofs compare against the state before the phase touched a file.
# That state is HEAD while the work is uncommitted, and the parent of the commit
# that added the new file once it is committed (base_of below).
set -u

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "not inside a git checkout" >&2; exit 2; }
cd "$ROOT" || exit 2
GSD="$HOME/.claude/gsd-core/bin/gsd-tools.cjs"
PHASE_DIR=.planning/phases/27-repository-tidy
STATE=.planning/STATE.md
HISTORY=.planning/milestones/STATE-HISTORY.md
ROADMAP=.planning/ROADMAP.md
ARCH_ROADMAP=.planning/milestones/v0.1.0-ROADMAP.md
ARCH_AUDIT=.planning/milestones/v0.1.0-MILESTONE-AUDIT.md
DOC=${BRANCHES_DOC:-docs/branches.md}
FAIL=0
TMPS=()

cleanup() { local d; for d in "${TMPS[@]:-}"; do [ -n "$d" ] && rm -rf "$d"; done; }
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; FAIL=1; }
ok() { echo "ok: $*"; }
mktmp() { local d; d=$(mktemp -d) || exit 2; TMPS+=("$d"); printf '%s' "$d"; }

# gsd QUERY... : run a gsd-tools query against GSD_DIR (default: this checkout).
gsd() {
  [ -f "$GSD" ] || { echo "gsd-tools not found at $GSD" >&2; exit 2; }
  node "$GSD" query "$@" --project-dir "${GSD_DIR:-$ROOT}" 2>&1 | grep -v '^gsd-tools: warning'
}

# base_of PATH : the commit to compare a verbatim move against.
base_of() {
  local created
  created=$(git log --diff-filter=A --format=%H -1 -- "$1" 2>/dev/null)
  if [ -n "$created" ]; then git rev-parse "$created^"; else git rev-parse HEAD; fi
}

# progress_rows FILE : the data rows of the Progress table (phase number first).
progress_rows() { grep -E '^\| [0-9]+(\.[0-9]+)?\. ' "$1" || true; }

# ---------------------------------------------------------------- state ----
check_state() {
  [ -f "$STATE" ] || { fail "$STATE is missing"; return; }

  # 1. exactly one Current Position heading, exactly one Phase: line in its slice
  #    (the slice is what state.advance-plan reads: heading to the next heading of the same or a higher level)
  local counts heads phases
  counts=$(awk '
    function level(s,  n) { n = 0; while (substr(s, n + 1, 1) == "#") n++; return n }
    /^```/ { fence = !fence; next }
    fence { if (inpos && /^Phase:/) n++; next }
    /^#+ / {
      lv = level($0); txt = $0; sub(/^#+ +/, "", txt); sub(/ +$/, "", txt); t = tolower(txt)
      if (inpos && lv <= poslv) { inpos = 0; done = 1 }
      if ((lv == 2 || lv == 3) && t == "current position") {
        heads++
        if (!done && !inpos) { inpos = 1; poslv = lv }
      }
      next
    }
    inpos && /^Phase:/ { n++ }
    END { printf "%d %d\n", heads, n }
  ' "$STATE")
  heads=${counts% *}; phases=${counts#* }
  [ "$heads" = 1 ] && ok "one Current Position heading" || fail "Current Position headings: $heads (want 1)"
  [ "$phases" = 1 ] && ok "one Phase: line in the Current Position section" || fail "Phase: lines in the Current Position section: $phases (want 1)"

  # 2. the tools read the file as valid
  local v
  v=$(gsd state.validate)
  if printf '%s' "$v" | grep -q '"valid": true' && ! printf '%s' "$v" | grep -q 'S002'; then ok "state.validate: valid, no S002"; else fail "state.validate: $(printf '%s' "$v" | tr -d '\n' | cut -c1-300)"; fi

  # 3. a resync would change no progress counter
  local s
  s=$(gsd state.sync --verify)
  if printf '%s' "$s" | grep -Eqi '^[[:space:]]*"[^"]*progress[^"]*",?$'; then fail "state.sync --verify would change a progress counter: $(printf '%s' "$s" | tr -d '\n' | cut -c1-300)"; else ok "state.sync --verify: no progress counter would move"; fi

  # 4. the counters are the real totals of the open milestone, read from the live Progress table
  local rows total done_n fm_total fm_done
  rows=$(progress_rows "$ROADMAP")
  total=$(printf '%s\n' "$rows" | grep -c . || true)
  done_n=$(printf '%s\n' "$rows" | awk -F'|' '{ s = $5; gsub(/^ +| +$/, "", s); if (s ~ /^Complete/) n++ } END { print n + 0 }')
  fm_total=$(awk 'NR == 1 && $0 != "---" { exit } NR > 1 && $0 == "---" { exit } /^  total_phases:/ { print $2 }' "$STATE")
  fm_done=$(awk 'NR == 1 && $0 != "---" { exit } NR > 1 && $0 == "---" { exit } /^  completed_phases:/ { print $2 }' "$STATE")
  if [ "$total" -gt 0 ] && [ "${fm_total:-x}" = "$total" ]; then ok "total_phases $fm_total equals the rows of the Progress table"; else fail "total_phases ${fm_total:-absent} but the Progress table has $total rows"; fi
  if [ "${fm_done:-x}" = "$done_n" ]; then ok "completed_phases $fm_done equals the Complete rows of the Progress table"; else fail "completed_phases ${fm_done:-absent} but the Progress table has $done_n Complete rows"; fi
  local fm_ms foreign
  fm_ms=$(awk 'NR == 1 && $0 != "---" { exit } NR > 1 && $0 == "---" { exit } /^milestone:/ { print $2 }' "$STATE")
  foreign=$(printf '%s\n' "$rows" | awk -F'|' -v ms="$fm_ms" '{ m = $3; gsub(/^ +| +$/, "", m); if (m != ms) n++ } END { print n + 0 }')
  if [ -n "$fm_ms" ] && [ "$foreign" = 0 ]; then ok "every row of the live Progress table belongs to the open milestone $fm_ms"; else fail "$foreign row(s) of the live Progress table do not belong to the open milestone '$fm_ms'"; fi

  # 5. state.advance-plan, played on a copy, does not refuse
  local T out
  T=$(mktmp); mkdir -p "$T/.planning"; cp -R .planning/. "$T/.planning/"; (cd "$T" && git init -q . >/dev/null 2>&1)
  out=$(GSD_DIR=$T gsd state.advance-plan)
  if printf '%s' "$out" | grep -q 'ambiguous_position_phase' || printf '%s' "$out" | grep -q '"error"'; then
    fail "state.advance-plan refuses: $(printf '%s' "$out" | tr -d '\n' | cut -c1-300)"
  elif printf '%s' "$out" | grep -q '"advanced"'; then ok "state.advance-plan answers without refusing (played on a copy)"
  else fail "state.advance-plan gave no answer: $(printf '%s' "$out" | tr -d '\n' | cut -c1-300)"; fi

  # 6. nothing was lost: every non-blank line of the old body is in the new STATE.md or in the history
  [ -f "$HISTORY" ] || { fail "$HISTORY is missing"; return; }
  local base W
  base=$(base_of "$HISTORY"); W=$(mktmp)
  git show "$base:$STATE" | awk 'NR == 1 && $0 == "---" { fm = 1; next } fm && $0 == "---" { fm = 0; next } !fm' | grep -v '^[[:space:]]*$' | sort -u > "$W/old"
  cat "$STATE" "$HISTORY" | grep -v '^[[:space:]]*$' | sort -u > "$W/new"
  local lost
  lost=$(comm -23 "$W/old" "$W/new" | wc -l | tr -d ' ')
  if [ "$lost" = 0 ]; then ok "every body line of STATE.md at ${base:0:10} is in STATE.md or STATE-HISTORY.md"; else fail "$lost line(s) of STATE.md at ${base:0:10} are in neither file:"; comm -23 "$W/old" "$W/new" | head -5 >&2; fi
  local n
  n=$(wc -l < "$STATE" | tr -d ' ')
  [ "$n" -le 320 ] && ok "STATE.md is $n lines" || fail "STATE.md is $n lines (want 320 or fewer)"
  grep -q "${base:0:10}" "$HISTORY" && ok "STATE-HISTORY.md names the base commit ${base:0:10}" || fail "STATE-HISTORY.md does not name the base commit ${base:0:10}"
}

# -------------------------------------------------------------- archive ----
check_archive() {
  case "${1:-all}" in
    live) check_archive_live ;;
    file) check_archive_file ;;
    roadmap) check_archive_live; check_archive_file ;;
    audit) check_archive_audit ;;
    all) check_archive_live; check_archive_file; check_archive_audit; check_archive_tools ;;
    *) echo "archive: live | file | roadmap | audit" >&2; exit 2 ;;
  esac
}

# the live roadmap: one line for v0.1.0, no phase 20 to 23 detail, a Progress table of the open milestone
check_archive_live() {
  local lines detail live
  lines=$(grep -c '^- .*\*\*v0\.1\.0 — ' "$ROADMAP" || true)
  [ "$lines" = 1 ] && ok "ROADMAP.md has one Milestones line for v0.1.0" || fail "ROADMAP.md has $lines Milestones lines for v0.1.0 (want 1)"
  grep '^- .*\*\*v0\.1\.0 — ' "$ROADMAP" | grep -q 'milestones/v0\.1\.0-ROADMAP\.md' && ok "that line points at milestones/v0.1.0-ROADMAP.md" || fail "the v0.1.0 line does not point at milestones/v0.1.0-ROADMAP.md"
  grep '^- .*\*\*v0\.1\.0 — ' "$ROADMAP" | grep -qi 'not yet archived' && fail "the v0.1.0 line still says it is not archived"
  detail=$(grep -cE '^### Phase (20|21|22|23):' "$ROADMAP" || true)
  [ "$detail" = 0 ] && ok "ROADMAP.md carries no phase 20 to 23 detail" || fail "ROADMAP.md still carries $detail phase 20 to 23 heading(s)"
  live=$(progress_rows "$ROADMAP" | awk -F'|' '{ p = $2; sub(/^ +/, "", p); sub(/\..*/, "", p); printf "%s ", p }')
  [ "$live" = "24 25 26 27 28 " ] && ok "the live Progress table lists phases 24 to 28 only" || fail "the live Progress table lists: ${live:-nothing}"
}

# the archive: phases 20 to 23 verbatim, rows 1 to 23 verbatim
check_archive_file() {
  local f
  for f in .planning/milestones/v0.1.0-REQUIREMENTS.md "$ARCH_ROADMAP"; do
    [ -s "$f" ] && ok "$f exists" || fail "$f is missing or empty"
  done
  [ -s "$ARCH_ROADMAP" ] || return
  local base W
  base=$(base_of "$ARCH_ROADMAP"); W=$(mktmp)
  git show "$base:$ROADMAP" | awk '
    /^### Phase 20:/ { on = 1 }
    on && /^---$/ { exit }
    on { buf[++n] = $0 }
    END { while (n > 0 && buf[n] ~ /^[[:space:]]*$/) n--; for (i = 1; i <= n; i++) print buf[i] }
  ' > "$W/want"
  awk '
    /^### Phase 20:/ { on = 1 }
    on && /^## / { exit }
    on { buf[++n] = $0 }
    END { while (n > 0 && buf[n] ~ /^[[:space:]]*$/) n--; for (i = 1; i <= n; i++) print buf[i] }
  ' "$ARCH_ROADMAP" > "$W/got"
  if [ -s "$W/want" ] && cmp -s "$W/want" "$W/got"; then ok "the archive carries the phase 20 to 23 detail byte for byte ($(wc -l < "$W/want" | tr -d ' ') lines, from ${base:0:10})"; else fail "the archive's phase 20 to 23 block differs from ROADMAP.md at ${base:0:10}"; fi
  local missing=0 row
  while IFS= read -r row; do
    [ -z "$row" ] && continue
    grep -qxF -- "$row" "$ARCH_ROADMAP" || { missing=$((missing + 1)); echo "  not in the archive: $row" >&2; }
  done < <(git show "$base:$ROADMAP" | grep -E '^\| ([1-9]|1[0-9]|2[0-3])\. ')
  [ "$missing" = 0 ] && ok "the archive carries the Progress rows 1 to 23 byte for byte" || fail "$missing Progress row(s) of phases 1 to 23 are not in the archive"
}

check_archive_audit() {
  [ -s "$ARCH_AUDIT" ] && ok "$ARCH_AUDIT exists" || { fail "$ARCH_AUDIT is missing or empty"; return; }
  grep -qi 'written after the release' "$ARCH_AUDIT" && ok "the audit says it was written after the release" || fail "the audit does not say it was written after the release"
  grep -q '2026-10-18' "$ARCH_AUDIT" && ok "the audit names the release date" || fail "the audit does not name the release date 2026-10-18"
  local id miss_ids=""
  for id in UNIT-01 UNIT-02 UNIT-03 DEP-01 DEP-02 DEP-03 BAK-01 BAK-02 BAK-03 REPO-01 REPO-02 REPO-03 QUAL-01; do
    grep -q "$id" "$ARCH_AUDIT" || miss_ids="$miss_ids $id"
  done
  [ -z "$miss_ids" ] && ok "the audit names all 13 requirements" || fail "the audit does not name:$miss_ids"
  local recs=0 bad=0 line path quote
  while IFS= read -r line; do
    recs=$((recs + 1))
    path=$(printf '%s' "$line" | sed -n 's/^Record: `\([^`]*\)` — ".*"$/\1/p')
    quote=$(printf '%s' "$line" | sed -n 's/^Record: `[^`]*` — "\(.*\)"$/\1/p')
    if [ -z "$path" ] || [ -z "$quote" ] || [ ! -f "$path" ] || ! grep -qF -- "$quote" "$path"; then bad=$((bad + 1)); echo "  record not found byte for byte: ${line:0:160}" >&2; fi
  done < <(grep '^Record: ' "$ARCH_AUDIT" || true)
  if [ "$recs" -ge 13 ] && [ "$bad" = 0 ]; then ok "$recs Record: lines, each quoted sentence is in the file it names"; else fail "Record: lines $recs (want 13 or more), not verified $bad"; fi
}

check_archive_tools() {
  local a r
  r=$(gsd roadmap.analyze)
  printf '%s' "$r" | grep -q '"phase_count": 5' && ok "roadmap.analyze: phase_count 5" || fail "roadmap.analyze does not report phase_count 5"
  a=$(gsd audit-open)
  printf '%s' "$a" | grep -q 'All artifact types clear' && ok "audit-open: all artifact types clear" || fail "audit-open is not clear: $(printf '%s' "$a" | tr -d '\n' | cut -c1-300)"
}

# ------------------------------------------------------------- branches ----
# col LINE N : cell N (1-based) of a markdown table row, without backticks, asterisks and edge spaces
col() { printf '%s\n' "$1" | awk -F'|' -v n="$((${2} + 1))" '{ x = $n; gsub(/[`*]/, "", x); gsub(/^ +| +$/, "", x); print x }'; }

check_branches() {
  local only=""
  if [ "${1:-}" = "--only" ]; then only=${2:-}; fi
  [ -f "$DOC" ] || { fail "$DOC is missing"; return; }
  git fetch origin --prune >/dev/null 2>&1 || { echo "git fetch origin --prune failed (record it and stop)" >&2; exit 2; }

  local W; W=$(mktmp)
  # the two tables, by section
  awk -v w="$W" '/^## The table$/ { s = "t"; next } /^## Decisions$/ { s = "d"; next } /^## / { s = "" } s == "t" && /^\| `/ { print > (w "/evidence") } s == "d" && /^\| `/ { print > (w "/decisions") }' "$DOC"
  touch "$W/evidence" "$W/decisions"
  [ -s "$W/evidence" ] && ok "evidence table: $(wc -l < "$W/evidence" | tr -d ' ') rows" || fail "no evidence table under '## The table'"
  [ -s "$W/decisions" ] && ok "decisions table: $(wc -l < "$W/decisions" | tr -d ' ') rows" || fail "no decisions table under '## Decisions'"

  local main10
  main10=$(git rev-parse --short=10 origin/main)
  if grep -qE "^Generated 20[0-9]{2}-[0-9]{2}-[0-9]{2} against origin/main [0-9a-f]{10}" "$DOC"; then
    if grep -qE "^Generated .* against origin/main $main10" "$DOC"; then ok "the page was generated against the current origin/main $main10"; else echo "warn: origin/main is $main10, the page was generated against another commit; the rows below are recomputed against $main10 all the same" >&2; fi
  else fail "no 'Generated DATE against origin/main SHA10.' line"; fi

  # sets: evidence rows = decision rows; remote = decision rows not deleted
  local e d
  e=$(awk -F'|' '{ x = $2; gsub(/[` ]/, "", x); print x }' "$W/evidence" | sort)
  d=$(awk -F'|' '{ x = $2; gsub(/[` ]/, "", x); print x }' "$W/decisions" | sort)
  if [ -z "$only" ]; then
    [ "$e" = "$d" ] && ok "evidence and decisions list the same branches" || fail "evidence and decisions list different branches"
    git for-each-ref --format='%(refname:short)' refs/remotes/origin | grep -vx -e origin -e origin/HEAD -e origin/main | sed 's#^origin/##' | sort > "$W/remote"
    awk -F'|' '{ n = $2; gsub(/[` ]/, "", n); r = $7; gsub(/^ +| +$/, "", r); if (r !~ /^deleted /) print n }' "$W/decisions" | sort > "$W/expected"
    if cmp -s "$W/remote" "$W/expected"; then ok "the remote's branches (besides main) are exactly the decisions not marked deleted"; else fail "the remote's branches differ from the decisions:"; diff "$W/remote" "$W/expected" >&2; fi
  fi

  # each evidence row against git
  local row b tip ahead behind verdict want plus
  while IFS= read -r row; do
    b=$(col "$row" 1)
    [ -n "$only" ] && [ "$b" != "$only" ] && continue
    if ! git rev-parse -q --verify "refs/remotes/origin/$b" >/dev/null; then
      grep -E "^\| \`$b\` " "$W/decisions" | grep -q '| deleted ' && continue
      fail "$b: not on the remote and not marked deleted"; continue
    fi
    tip=$(col "$row" 3)
    [ "$tip" = "$(git rev-parse --short=10 "origin/$b")" ] || fail "$b: tip $tip differs from git"
    ahead=$(col "$row" 4); ahead=${ahead#*/}; ahead=${ahead// /}
    [ "$ahead" = "$(git rev-list --count "origin/main..origin/$b")" ] || fail "$b: the branch-only count $ahead differs from git"
    verdict=$(col "$row" 7)
    if [ "$verdict" = "In use by a session" ]; then continue; fi
    plus=$(git cherry origin/main "origin/$b" | grep -c '^+' || true)
    if [ "$b" = release ]; then want=Keep; elif [ "$plus" -gt 0 ]; then want="Review first"; else want="Safe to delete"; fi
    [ "$verdict" = "$want" ] || fail "$b: verdict '$verdict' but the evidence says '$want'"
  done < "$W/evidence"

  # each decision row
  local dec result full subj
  while IFS= read -r row; do
    b=$(col "$row" 1)
    [ -n "$only" ] && [ "$b" != "$only" ] && continue
    verdict=$(col "$row" 2); dec=$(col "$row" 3); full=$(col "$row" 4); subj=$(col "$row" 5); result=$(col "$row" 6)
    case "$dec" in Delete|Keep|"In use by a session") ;; *) fail "$b: decision '$dec' is none of Delete, Keep, In use by a session" ;; esac
    printf '%s' "$full" | grep -qE '^[0-9a-f]{40}$' || fail "$b: the Tip cell must be the full 40-character SHA, it reads '$full'"
    [ -n "$subj" ] || fail "$b: the subject of the tip is empty"
    case "$result" in pending|kept|"in use by a session"|deleted\ 20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;; *) fail "$b: result '$result' is none of pending, kept, in use by a session, deleted YYYY-MM-DD" ;; esac
    if [ "$b" = release ] && [ "$dec" != Keep ]; then fail "release must be Keep, it reads '$dec'"; fi
    [ "$b" = "fix/bausteine-befunde" ] && [ "$dec" != "In use by a session" ] && fail "fix/bausteine-befunde belongs to a running session and is not touched; it reads '$dec'"
    [ "$dec" = Delete ] && [ "$verdict" = "In use by a session" ] && fail "$b: a branch in use is not deleted"
    if git rev-parse -q --verify "refs/remotes/origin/$b" >/dev/null; then
      [ "$full" = "$(git rev-parse "origin/$b")" ] || fail "$b: the recorded tip $full is not the remote's tip $(git rev-parse "origin/$b")"
      case "$result" in deleted*) fail "$b: marked '$result' but the branch is still on the remote" ;; esac
    else
      case "$result" in deleted*) ;; *) fail "$b: gone from the remote but the result reads '$result'" ;; esac
    fi
  done < "$W/decisions"

  # the deletion steps for the owner
  grep -q '^## Deleting a branch in the GitHub UI$' "$DOC" && ok "the GitHub UI steps are in the page" || fail "no '## Deleting a branch in the GitHub UI' section"
  grep -q '^## Restoring a deleted branch$' "$DOC" && ok "the restore routes are in the page" || fail "no '## Restoring a deleted branch' section"
  [ "$FAIL" = 0 ] && ok "branches: the page agrees with git${only:+ for $only}"
}


# ---------------------------------------------------------- owner steps ----
OWNER=${OWNER_STEPS:-.planning/phases/27-repository-tidy/27-OWNER-STEPS.md}
check_owner_steps() {
  [ -s "$OWNER" ] || { fail "$OWNER is missing or empty"; return; }
  [ -f "$DOC" ] || { fail "$DOC is missing"; return; }
  local W; W=$(mktmp)
  awk -v w="$W" '/^## Decisions$/ { s = 1; next } /^## / { s = 0 } s && /^\| `/ { print > (w "/decisions") }' "$DOC"
  touch "$W/decisions"
  local row b dec full n=0
  while IFS= read -r row; do
    b=$(col "$row" 1); dec=$(col "$row" 3); full=$(col "$row" 4)
    [ "$dec" = Delete ] || continue
    n=$((n + 1))
    grep -qF -- "--force-with-lease=refs/heads/$b:$full" "$OWNER" || fail "$b: no lease-guarded delete command with the full tip $full in the owner steps"
    grep -qF -- "git ls-remote --heads origin $b" "$OWNER" || fail "$b: no ls-remote check in the owner steps"
  done < "$W/decisions"
  [ "$n" -gt 0 ] && ok "owner steps: a lease-guarded delete and an ls-remote check for each of the $n branches to delete" || fail "no Delete row in the Decisions table"
  grep -qE 'GitHub|trash' "$OWNER" && grep -qF 'View all branches' "$OWNER" && ok "owner steps: the GitHub UI path is in the file" || fail "owner steps: the GitHub UI path is missing"
  local c
  c=$(grep -c '^Expected' "$OWNER" || true)
  [ "$c" -ge "$((n + 6))" ] && ok "owner steps: $c 'Expected' blocks" || fail "owner steps: $c 'Expected' blocks (want at least $((n + 6)): one per delete, the checks, the local steps)"
  grep -qF 'git bundle create' "$OWNER" && grep -qF 'git bundle verify' "$OWNER" && ok "owner steps: the bundle of the local backup branch is made and verified before any local delete" || fail "owner steps: bundle create / verify missing"
  if git show-ref --verify --quiet refs/heads/backup/lokal-vor-origin-abgleich; then
    grep -qF -- "$(git rev-parse backup/lokal-vor-origin-abgleich)" "$OWNER" && ok "owner steps: the tip of the local backup branch is noted" || fail "owner steps: the tip $(git rev-parse backup/lokal-vor-origin-abgleich) of the local backup branch is not noted"
  fi
  grep -qF 'git branch -D backup/lokal-vor-origin-abgleich' "$OWNER" && ok "owner steps: the local backup branch delete is written out" || fail "owner steps: no 'git branch -D backup/lokal-vor-origin-abgleich'"
  grep -qF 'docs/refresh-2026-10-20' "$OWNER" && ok "owner steps: the stray local branch is named" || fail "owner steps: docs/refresh-2026-10-20 is not named"
}

# --------------------------------------------------------- requirements ----
check_requirements() {
  local REQ=.planning/REQUIREMENTS.md id cell box
  for id in TIDY-01 TIDY-02 TIDY-03 TIDY-04; do
    cell=$(grep -E "^\| $id \| Phase 27 \|" "$REQ" | awk -F'|' '{ x = $4; gsub(/^ +| +$/, "", x); print x }')
    box=$(grep -E "^- \[[ x]\] \*\*$id\*\*" "$REQ" | sed -n 's/^- \[\(.\)\].*/\1/p')
    case "$cell:$box" in
      "Complete:x"|"Pending:"" "|"Pending owner:"" ") ok "$id: '$cell' and the box '[$box]' agree" ;;
      *) fail "$id: the traceability cell reads '$cell' and the box '[$box]'" ;;
    esac
  done
  # TIDY-01 may read Complete only when no Decisions row is still pending; TIDY-04 only when the two local branches are gone
  local c1 c4
  c1=$(grep -E "^\| TIDY-01 \|" "$REQ" | awk -F'|' '{ x = $4; gsub(/^ +| +$/, "", x); print x }')
  c4=$(grep -E "^\| TIDY-04 \|" "$REQ" | awk -F'|' '{ x = $4; gsub(/^ +| +$/, "", x); print x }')
  if [ "$c1" = Complete ] && [ -f "$DOC" ]; then
    awk '/^## Decisions$/ { s = 1; next } /^## / { s = 0 } s && /^\| `/ { print }' "$DOC" | awk -F'|' '{ r = $7; gsub(/^ +| +$/, "", r); if (r == "pending") n++ } END { exit n ? 1 : 0 }' && ok "TIDY-01 reads Complete and no decision is pending" || fail "TIDY-01 reads Complete but a decision is still pending"
  fi
  if [ "$c4" = Complete ]; then
    if git show-ref --verify --quiet refs/heads/backup/lokal-vor-origin-abgleich || git show-ref --verify --quiet refs/heads/docs/refresh-2026-10-20; then fail "TIDY-04 reads Complete but a local branch it names still exists"; else ok "TIDY-04 reads Complete and both local branches are gone"; fi
  fi
}

# ----------------------------------------------------------------- main ----
cmd=${1:-all}
shift || true
case "$cmd" in
  state) check_state ;;
  archive) check_archive "${1:-all}" ;;
  branches) check_branches "$@" ;;
  owner-steps) check_owner_steps ;;
  requirements) check_requirements ;;
  all) check_branches; check_owner_steps; check_requirements; check_state; check_archive ;;
  *) echo "usage: $0 branches [--only NAME] | owner-steps | requirements | state | archive [live|file|roadmap|audit] | all" >&2; exit 2 ;;
esac
if [ "$FAIL" = 0 ]; then echo "27-CHECK $cmd: PASS"; exit 0; else echo "27-CHECK $cmd: FAIL" >&2; exit 1; fi
