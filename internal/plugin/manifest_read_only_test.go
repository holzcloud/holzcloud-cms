package plugin

import (
	"strings"
	"testing"
)

// A plugin promises in its manifest that it reads AdminIn.ReadOnly and refuses
// its own public-changing writes (phase 31, D-13). The promise is a field of
// the admin entry, and ParseManifest refuses unknown fields — so until the
// field exists, a plugin that makes the promise does not install at all.
func TestTheAdminEntryCarriesRespectsReadOnly(t *testing.T) {
	manifest := func(admin string) []byte {
		return []byte(`{
  "id": "gaestebuch",
  "abi": 1,
  "name": "Guestbook",
  "version": "1.0.0",
  "hooks": ["admin"],
  "admin": ` + admin + `
}`)
	}

	m, err := ParseManifest(manifest(`{"label": "Entries", "per_website": true, "respects_read_only": true}`))
	if err != nil {
		t.Fatalf("a manifest that declares respects_read_only was refused: %v", err)
	}
	if !m.Admin.RespectsReadOnly {
		t.Error("respects_read_only: true was read as false")
	}

	// Without the declaration the field is false, and the manifest is exactly
	// one this program accepted before phase 31.
	m, err = ParseManifest(manifest(`{"label": "Entries", "per_website": true}`))
	if err != nil {
		t.Fatalf("a manifest without the declaration was refused: %v", err)
	}
	if m.Admin.RespectsReadOnly {
		t.Error("a manifest that does not declare it respects the read-only rule")
	}

	// A misspelling is still caught: the field is added, the strictness stays.
	if _, err := ParseManifest(manifest(`{"label": "Entries", "respect_read_only": true}`)); err == nil ||
		!strings.Contains(err.Error(), "respect_read_only") {
		t.Errorf("a misspelt declaration was accepted or not named: %v", err)
	}
}
