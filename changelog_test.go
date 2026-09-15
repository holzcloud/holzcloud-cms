package holzcloud

import (
	"os"
	"strings"
	"testing"
)

// The changelog is in the binary.
//
// A smoke test and not a parser test — internal/changelog does the parsing and
// tests it. What this holds is that the embed found a file at all, which is the
// one thing that can go wrong at the repository root.
func TestTheChangelogIsEmbedded(t *testing.T) {
	if len(Changelog) < 1000 {
		t.Fatalf("CHANGELOG.md embedded as %d bytes; that is not a changelog", len(Changelog))
	}
	if !strings.Contains(Changelog, "\n## ") {
		t.Error("the embedded file carries no version heading")
	}
}

// The image build has to be able to see it too.
//
// This is a real regression and the reason this test exists: .dockerignore
// excludes *.md at the repository root, so the file was in the repository, in
// every working copy and in `go build` — and absent from the Docker build
// context. The image build failed with "pattern CHANGELOG.md: no matching files
// found" while everything a developer runs locally stayed green.
//
// A go:embed at the root is the only place this can happen, because it is the
// only place *.md excludes. Held here rather than in a comment, because a
// comment does not fail.
func TestTheDockerContextKeepsTheChangelog(t *testing.T) {
	raw, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}

	excluded := false
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Docker applies the patterns in order, last match wins.
		switch line {
		case "*.md", "CHANGELOG.md":
			excluded = true
		case "!CHANGELOG.md":
			excluded = false
		}
	}
	if excluded {
		t.Error("CHANGELOG.md is excluded from the Docker build context, and changelog.go " +
			"embeds it: the image build will fail with \"pattern CHANGELOG.md: no matching " +
			"files found\" while go build here stays green")
	}
}
