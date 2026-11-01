package changelog

import "strings"

// Outcome is what the display rule says to do for one person on one page load.
type Outcome int

const (
	// Nothing: show nothing and write nothing.
	Nothing Outcome = iota
	// Record: write the running version down and show nothing. A new account
	// has no earlier version to catch up from.
	Record
	// Show: the person is behind the running release; show them its entry,
	// then record it.
	Show
)

// Decide is the display rule of the What's-new screen, in one place and with no
// I/O: given the version this person last had recorded and the version that
// runs, what happens.
//
//   - A build that is not a release in CHANGELOG.md ("dev", a git-describe
//     suffix like v0.1.2-3-gabc1234, a bare hash) has no entry to show, so it
//     never shows and never records. Recording would burn the announcement for
//     the real release that comes next.
//   - Nothing recorded yet is a new account (or a fresh installation): record
//     and show nothing.
//   - Recorded equal to the running version: nothing to do.
//   - Recorded is an older known release: show the running version.
//   - Recorded is a known release newer than the running one: a downgrade. The
//     highest version seen stays recorded and nothing is shown, so going back
//     and forth does not nag.
//   - Recorded is a value this list does not know: counted as older. Showing
//     once too often is better than never showing.
//
// The returned string is the running version as the changelog spells it
// (without a leading v).
func Decide(seen, running string) (Outcome, string) {
	all := All()
	versions := make([]string, len(all))
	for i, r := range all {
		versions[i] = r.Version
	}
	return decide(versions, seen, running)
}

// decide is Decide over an explicit newest-first list, so the rule can be
// tested without depending on what CHANGELOG.md holds today.
func decide(versions []string, seen, running string) (Outcome, string) {
	running = strings.TrimPrefix(strings.TrimSpace(running), "v")
	runIdx := index(versions, running)
	if runIdx < 0 {
		return Nothing, ""
	}
	seen = strings.TrimPrefix(strings.TrimSpace(seen), "v")
	if seen == "" {
		return Record, running
	}
	if seen == running {
		return Nothing, ""
	}
	seenIdx := index(versions, seen)
	// A smaller index is a newer release. Seen newer than running: downgrade.
	if seenIdx >= 0 && seenIdx < runIdx {
		return Nothing, ""
	}
	return Show, running
}

func index(versions []string, v string) int {
	for i, x := range versions {
		if x == v {
			return i
		}
	}
	return -1
}
