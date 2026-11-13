package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests read the deployment documents an operator copies from. Until the
// Phase 10 audit nothing did: the delete lines in the Caddyfile and the minimum
// Caddy version in DEPLOY.md were checked once, by hand, when the plan was
// executed, and could have disappeared the next day without a test noticing
// (threat T-10-44). Against a Caddy that forwards a visitor's own identity header
// under the canonical name, those lines are the only defence there is.

func readDeploy(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "deploy", name))
	if err != nil {
		t.Fatalf("read deploy/%s: %v", name, err)
	}
	return string(b)
}

// caddyLines returns the example's lines with the comment markers taken off,
// because the shipped example carries the forward-auth block commented out.
func caddyLines(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(readDeploy(t, "Caddyfile.example"), "\n") {
		out = append(out, strings.TrimLeft(line, "#\t "))
	}
	return out
}

func TestCaddyfileDeletesEveryCopiedHeaderInBothSpellings(t *testing.T) {
	lines := caddyLines(t)
	copyAt, forwardAt := -1, -1
	var copied []string
	for i, l := range lines {
		if strings.HasPrefix(l, "copy_headers ") {
			copyAt = i
			copied = strings.Fields(strings.TrimPrefix(l, "copy_headers "))
		}
		if strings.HasPrefix(l, "forward_auth ") {
			forwardAt = i
		}
	}
	if copyAt < 0 || forwardAt < 0 || len(copied) == 0 {
		t.Fatal("deploy/Caddyfile.example has no forward_auth block with copy_headers")
	}
	for _, header := range copied {
		for _, spelling := range []string{header, strings.ReplaceAll(header, "-", "_")} {
			want := "request_header -" + spelling
			at := -1
			for i, l := range lines {
				if l == want {
					at = i
				}
			}
			switch {
			case at < 0:
				t.Errorf("deploy/Caddyfile.example copies %s but has no %q", header, want)
			case at > forwardAt:
				t.Errorf("%q stands after forward_auth; the visitor's copy must be gone before the outpost is asked", want)
			}
		}
	}
}

func TestCaddyfilePassesTheSecretFromTheEnvironment(t *testing.T) {
	found := false
	for _, l := range caddyLines(t) {
		if strings.HasPrefix(l, "header_up X-Holzcloud-Proxy-Secret") {
			found = true
			if got := strings.TrimSpace(strings.TrimPrefix(l, "header_up X-Holzcloud-Proxy-Secret")); got != "{env.HOLZCLOUD_SSO_SECRET}" {
				t.Errorf("the proxy secret is set to %q; a Caddyfile is a file people paste into forums, the value belongs in the environment", got)
			}
		}
	}
	if !found {
		t.Error("deploy/Caddyfile.example never sends X-Holzcloud-Proxy-Secret")
	}
}

func TestDeployNamesTheMinimumCaddy(t *testing.T) {
	doc := readDeploy(t, "DEPLOY.md")
	for _, want := range []string{"2.11.2", "CVE-2026-30851"} {
		if !strings.Contains(doc, want) {
			t.Errorf("deploy/DEPLOY.md does not name %s", want)
		}
	}
}

// TestDeployTellsHowTheSecretReachesCaddy: the Caddyfile reads the secret from
// Caddy's environment, and nothing said how it gets there, while the service
// unit asks for payment and mail secrets in a 0600 environment file
// (threat T-10-45).
func TestDeployTellsHowTheSecretReachesCaddy(t *testing.T) {
	doc := readDeploy(t, "DEPLOY.md")
	for _, want := range []string{"systemctl edit caddy", "EnvironmentFile=/etc/holzcloud-sso.env"} {
		if !strings.Contains(doc, want) {
			t.Errorf("deploy/DEPLOY.md does not say how HOLZCLOUD_SSO_SECRET reaches Caddy (missing %q)", want)
		}
	}
}

// TestNoDeployDocumentCallsAForwardingProxyHarmless: the stripping inside the
// CMS runs after the identity headers are read, so it does not turn a proxy
// that forwards a visitor's value into a mere misconfiguration. Three documents
// said it did until the Phase 10 code review (WR-03); this holds the one that
// is copied most.
func TestNoDeployDocumentCallsAForwardingProxyHarmless(t *testing.T) {
	for _, name := range []string{"Caddyfile.example", "DEPLOY.md"} {
		doc := readDeploy(t, name)
		for _, claim := range []string{"not a way in", "rather than a way in", "rather than a bypass"} {
			if strings.Contains(doc, claim) {
				t.Errorf("deploy/%s still says %q about the strip", name, claim)
			}
		}
	}
}

// deploySection returns the part of doc from the heading line to the next
// heading of the same or a higher level, so a sentence pinned to a section
// cannot pass by standing somewhere else in the document. Lines inside a code
// fence are not headings: a shell comment starts with "#" too.
func deploySection(t *testing.T, doc, heading string) string {
	t.Helper()
	level := strings.IndexFunc(heading, func(r rune) bool { return r != '#' })
	if level <= 0 {
		t.Fatalf("deploySection: %q is not a heading", heading)
	}
	lines := strings.Split(doc, "\n")
	start := -1
	for i, line := range lines {
		if line == heading {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("deploy/DEPLOY.md has no heading %q", heading)
	}
	inFence := false
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		n := strings.IndexFunc(line, func(r rune) bool { return r != '#' })
		if n > 0 && n <= level && line[n] == ' ' {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// TestDeploySaysOlderBinariesLackTheSchemaRefusal: from 0.1.5 the binary
// refuses a database a newer binary migrated (STB-03). Rolling back said until
// then that nothing in the program enforced it; an operator who goes back to a
// binary from before 0.1.5 must still read that such a binary does not refuse.
func TestDeploySaysOlderBinariesLackTheSchemaRefusal(t *testing.T) {
	doc := readDeploy(t, "DEPLOY.md")
	section := deploySection(t, doc, "## Rolling back")
	for _, want := range []string{
		"Binaries released before 0.1.5 do not carry this refusal",
		"refusing to start: the database schema is newer than this binary",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("Rolling back does not say %q", want)
		}
	}
	for _, stale := range []string{"Nothing in the program enforces", "60 of 60"} {
		if strings.Contains(doc, stale) {
			t.Errorf("deploy/DEPLOY.md still says %q", stale)
		}
	}
}

// sectionContains fails the test for every sentence of want the section does
// not hold.
func sectionContains(t *testing.T, heading string, want ...string) {
	t.Helper()
	section := deploySection(t, readDeploy(t, "DEPLOY.md"), heading)
	for _, w := range want {
		if !strings.Contains(section, w) {
			t.Errorf("%s does not say %q", strings.TrimLeft(heading, "# "), w)
		}
	}
}

// TestDeployNamesTheChecksumFileAndItsRecord: from 0.1.5 every backup carries
// SHA256SUMS, named by its MANIFEST (BAK-01), and the record that shows it is
// deploy/BACKUP-VERIFY-RUN.md (BAK-04).
func TestDeployNamesTheChecksumFileAndItsRecord(t *testing.T) {
	sectionContains(t, "## Backups", "SHA256SUMS", "checksums=SHA256SUMS", "deploy/BACKUP-VERIFY-RUN.md")
}

// TestDeployRetentionFailureReachesTheJournal: a retention step that cannot
// remove an old backup says so and ends the night with status 40 (BAK-03).
func TestDeployRetentionFailureReachesTheJournal(t *testing.T) {
	sectionContains(t, "## Backups", "retention could not remove", "status 40")
}

// TestDeployStatesWhatRestoreEndsWith: restore.sh checks the checksums, reports
// every row whose file the backup lacks through check -files, and ends with
// status 3 then (BAK-01, BAK-02).
func TestDeployStatesWhatRestoreEndsWith(t *testing.T) {
	sectionContains(t, "### Restore from a backup", "made before 0.1.5", "check -files", "ends with status 3")
}

// TestDeployRestoreChecksTheListBothWays: restore.sh refuses a backup whose
// SHA256SUMS is empty or leaves a file out, the snapshot included, and the
// document says so instead of "it checks every file" (code review 32, CR-01).
func TestDeployRestoreChecksTheListBothWays(t *testing.T) {
	sectionContains(t, "### Restore from a backup",
		"every file of the backup must be listed there, the database snapshot included",
		"is not listed",
		"refuses an empty `SHA256SUMS`")
}

// TestDeployRestoreNamesTheWayBackAfterAFailure: a restore that fails after
// the stop leaves the site down; status 1 then has a third meaning, and the
// script prints the way back. A data directory that is a mount point has its
// contents moved instead (code review 32, WR-04).
func TestDeployRestoreNamesTheWayBackAfterAFailure(t *testing.T) {
	sectionContains(t, "### Restore from a backup",
		"fails after the service was stopped",
		"the commands that put the previous data back and start the service",
		"is a mount point")
}

// TestDeploySaysSubcommandsSnapshotBeforeTheyMigrate: from 0.1.5 a subcommand
// that migrates writes the pre-upgrade snapshot too, and Updating said the
// opposite (code review 32, IN-05).
func TestDeploySaysSubcommandsSnapshotBeforeTheyMigrate(t *testing.T) {
	sectionContains(t, "## Updating", "writes the same pre-upgrade snapshot as the service")
	if strings.Contains(readDeploy(t, "DEPLOY.md"), "and takes\nno snapshot") {
		t.Error("deploy/DEPLOY.md still says a subcommand takes no snapshot")
	}
}

// bashBlocks returns the lines inside the ```bash fences of section.
func bashBlocks(section string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(section, "\n") {
		switch {
		case !in && line == "```bash":
			in = true
		case in && strings.HasPrefix(line, "```"):
			in = false
		case in:
			out = append(out, line)
		}
	}
	return out
}

// TestDeployUpdatingCopiesTheScripts: Install copies backup.sh and restore.sh
// once, so an update that replaces only the binary keeps scripts that write no
// checksum file and say nothing about retention. The two lines are played by
// deploy/test/run.sh as they stand, which refuses an angle-bracket placeholder
// (threat T-32-14). They go through the operator's home directory: restore.sh
// runs as root, and a fixed name under /tmp could be prepared by any local
// account (code review 32, WR-05).
func TestDeployUpdatingCopiesTheScripts(t *testing.T) {
	lines := bashBlocks(deploySection(t, readDeploy(t, "DEPLOY.md"), "## Updating"))
	for _, want := range []string{
		"scp deploy/backup.sh deploy/restore.sh user@your-server:",
		"sudo install -m 755 -o root -g root ~/backup.sh ~/restore.sh /opt/holzcloud/deploy/",
	} {
		found := false
		for _, line := range lines {
			if line == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no bash block of Updating holds the line %q", want)
		}
	}
	placeholder := regexp.MustCompile(`<[A-Za-z][A-Za-z0-9_.-]*>`)
	for _, line := range lines {
		if placeholder.MatchString(line) {
			t.Errorf("a bash block of Updating holds a placeholder: %q", line)
		}
	}
}

// TestDeployInstallsNothingFromAFixedTmpPath: whatever root installs comes
// from the operator's home directory, never from a fixed name in the
// world-writable /tmp, where another account can create it first and have its
// own files installed (code review 32, WR-05).
func TestDeployInstallsNothingFromAFixedTmpPath(t *testing.T) {
	for i, line := range strings.Split(readDeploy(t, "DEPLOY.md"), "\n") {
		for _, fixed := range []string{"/tmp/deploy", "/tmp/holzcloud"} {
			if strings.Contains(line, fixed) {
				t.Errorf("deploy/DEPLOY.md:%d names %s: %q", i+1, fixed, line)
			}
		}
	}
}
