package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ziele is a literal table, and a literal table is a list somebody has to
// remember to add to.
//
// Review finding M-02, confirmed by the phase 6 security audit on 2026-09-08 as
// still live: a sixth plugin directory would simply not be in it, and this tool
// would then report every artefact current while never having looked at that
// one. Green CI, no warning, and no listing of what was skipped — which is the
// worst shape a gate can fail in, because the number it prints is the number
// somebody trusts.
//
// The audit called it latent because five directories exist and all five are in
// the table. This test is what keeps it latent.
func TestEveryPluginDirectoryIsAZiel(t *testing.T) {
	root := filepath.Join("..", "..")

	entries, err := os.ReadDir(filepath.Join(root, "plugins"))
	if err != nil {
		t.Fatalf("read plugins/: %v", err)
	}

	inTable := map[string]bool{}
	for _, z := range ziele {
		inTable[z.name] = true
	}

	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// A plugin is a directory carrying a manifest. Anything else under
		// plugins/ is not one and is not this tool's business.
		if _, err := os.Stat(filepath.Join(root, "plugins", e.Name(), manifestName)); err != nil {
			continue
		}
		found++
		if !inTable[e.Name()] {
			t.Errorf("plugins/%s carries a %s and is not in ziele — this tool would "+
				"report every artefact current without ever having built that one, "+
				"and say nothing about having skipped it", e.Name(), manifestName)
		}
	}
	if found == 0 {
		t.Fatal("no plugin directory found at all — the walk is looking in the wrong place " +
			"and this test would pass over an empty set")
	}

	// The other direction: an entry naming a directory that is gone would make
	// the tool fail for a reason that has nothing to do with staleness.
	for _, z := range ziele {
		if _, err := os.Stat(filepath.Join(root, z.dir)); err != nil {
			t.Errorf("ziele names %q at %s and that directory is not there: %v", z.name, z.dir, err)
		}
	}
}
