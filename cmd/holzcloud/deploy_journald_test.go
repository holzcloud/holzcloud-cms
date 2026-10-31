package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// journaldDirectives returns the lines of the drop-in that are neither blank
// nor comments.
func journaldDirectives(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(readDeploy(t, "journald-holzcloud.conf"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// The retention the privacy text states is enforced by the host's journal, so
// the number in the drop-in and the constant the text is built from must be the
// same number. Every extra directive is a decision somebody must take on purpose.
func TestJournaldDropInEnforcesTheStatedRetention(t *testing.T) {
	lines := journaldDirectives(t)
	if len(lines) != 3 {
		t.Fatalf("the drop-in holds %d directive lines, want exactly 3 ([Journal], MaxRetentionSec, MaxFileSec): %q", len(lines), lines)
	}
	if lines[0] != "[Journal]" {
		t.Errorf("first line = %q, want [Journal]", lines[0])
	}

	value, ok := strings.CutPrefix(lines[1], "MaxRetentionSec=")
	if !ok {
		t.Fatalf("second line = %q, want MaxRetentionSec=<n>d", lines[1])
	}
	days, ok := strings.CutSuffix(value, "d")
	if !ok {
		t.Fatalf("MaxRetentionSec=%q has no day suffix d", value)
	}
	n, err := strconv.Atoi(days)
	if err != nil {
		t.Fatalf("MaxRetentionSec=%q: the days are not a decimal number: %v", value, err)
	}
	if n != web.LogRetentionDays {
		t.Errorf("the drop-in keeps the journal %d days but web.LogRetentionDays is %d", n, web.LogRetentionDays)
	}

	if lines[2] != "MaxFileSec=1d" {
		t.Errorf("third line = %q, want MaxFileSec=1d", lines[2])
	}
}

func TestJournaldDropInSaysWhereItGoes(t *testing.T) {
	text := readDeploy(t, "journald-holzcloud.conf")
	for _, want := range []string{
		"/etc/systemd/journald.conf.d/holzcloud.conf",
		"systemctl restart systemd-journald",
		"systemd-analyze cat-config",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the drop-in header does not contain %q", want)
		}
	}
}

func TestDeploymentPageListsTheDropIn(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "deployment.md"))
	if err != nil {
		t.Fatalf("read docs/deployment.md: %v", err)
	}
	if !strings.Contains(string(b), "journald-holzcloud.conf") {
		t.Error("docs/deployment.md does not name journald-holzcloud.conf")
	}
}
