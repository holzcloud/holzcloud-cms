package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// The packer writes what the package format admits. Nothing keeps the two in
// step except a test that reads the format from its owner rather than from a
// second copy of the same four strings — which is how the fixed two-entry
// layout survived long enough to need an exception for kontaktformular.
func TestPackerAndFormatAgreeOnTheirNames(t *testing.T) {
	for _, f := range []struct {
		was  string
		hier string
		dort string
	}{
		{"manifest", manifestName, plugin.ManifestName},
		{"module", modulName, plugin.ModuleName},
		{"assets", assetDir, plugin.AssetDir},
		{"migrations", migrationDir, plugin.MigrationDir},
	} {
		if f.hier != f.dort {
			t.Errorf("%s: packer says %q, internal/plugin says %q", f.was, f.hier, f.dort)
		}
	}
}

// A packed archive must survive the reader that installs it, and must carry
// everything the source directory holds. Reading it back is the only check that
// looks at the archive the way an operator's installation does.
func TestPackedArchiveRoundTripsThroughReadPackage(t *testing.T) {
	quelle := t.TempDir()
	manifest := []byte(`{"id":"pruef","abi":1,"name":"Pruefung","version":"1.0.0","hooks":["content"]}`)
	modul := []byte("\x00asm\x01\x00\x00\x00")

	schreiben(t, filepath.Join(quelle, "assets", "stil.css"), []byte("b{color:red}"))
	schreiben(t, filepath.Join(quelle, "assets", "tief", "bild.svg"), []byte("<svg/>"))
	schreiben(t, filepath.Join(quelle, "migrations", "0001_start.sql"), []byte("CREATE TABLE t(a);"))
	// Source that must NOT be packed: it is not part of the format.
	schreiben(t, filepath.Join(quelle, "main.go"), []byte("package main"))

	archiv, err := packen(quelle, manifest, modul)
	if err != nil {
		t.Fatalf("packen: %v", err)
	}

	pkg, err := plugin.ReadPackage(bytes.NewReader(archiv), int64(len(archiv)))
	if err != nil {
		t.Fatalf("ReadPackage rejected an archive this tool produced: %v", err)
	}

	if !bytes.Equal(pkg.Module, modul) {
		t.Errorf("module changed in transit: %d bytes in, %d out", len(modul), len(pkg.Module))
	}
	for name, want := range map[string]string{
		"stil.css":      "b{color:red}",
		"tief/bild.svg": "<svg/>",
	} {
		got, ok := pkg.Assets[name]
		if !ok {
			t.Errorf("asset %q did not survive packing — the loss the fixed two-entry layout used to cause", name)
			continue
		}
		if string(got) != want {
			t.Errorf("asset %q = %q, want %q", name, got, want)
		}
	}
	switch {
	case len(pkg.Migrations) != 1:
		t.Errorf("expected exactly one migration, got %d — the loss that left kontaktformular without an archive at all", len(pkg.Migrations))
	case pkg.Migrations[0].Name != "0001_start.sql":
		t.Errorf("migration name = %q", pkg.Migrations[0].Name)
	case pkg.Migrations[0].SQL != "CREATE TABLE t(a);":
		t.Errorf("migration SQL = %q", pkg.Migrations[0].SQL)
	}
	if _, ok := pkg.Assets["../main.go"]; ok {
		t.Error("source file was packed as an asset")
	}
}

// Two runs over the same directory must produce identical bytes, or the byte
// comparison this whole tool exists for cannot hold for archives.
func TestPackingTwiceProducesTheSameBytes(t *testing.T) {
	quelle := t.TempDir()
	manifest := []byte(`{"id":"pruef","abi":1,"name":"Pruefung","version":"1.0.0","hooks":["content"]}`)
	modul := []byte("\x00asm\x01\x00\x00\x00")
	schreiben(t, filepath.Join(quelle, "assets", "b.css"), []byte("b{}"))
	schreiben(t, filepath.Join(quelle, "assets", "a.css"), []byte("a{}"))
	schreiben(t, filepath.Join(quelle, "migrations", "0001_x.sql"), []byte("SELECT 1;"))

	erst, err := packen(quelle, manifest, modul)
	if err != nil {
		t.Fatalf("packen: %v", err)
	}
	zweit, err := packen(quelle, manifest, modul)
	if err != nil {
		t.Fatalf("packen: %v", err)
	}
	if !bytes.Equal(erst, zweit) {
		t.Errorf("two runs differ: %d and %d bytes", len(erst), len(zweit))
	}
}

// A plugin with neither directory is the ordinary case — four of the five have
// neither — and must still pack.
func TestAPluginWithoutAssetsOrMigrationsStillPacks(t *testing.T) {
	quelle := t.TempDir()
	archiv, err := packen(quelle, []byte(`{"id":"leer","abi":1,"name":"Leer","version":"1.0.0","hooks":["content"]}`), []byte("\x00asm\x01\x00\x00\x00"))
	if err != nil {
		t.Fatalf("packen: %v", err)
	}
	pkg, err := plugin.ReadPackage(bytes.NewReader(archiv), int64(len(archiv)))
	if err != nil {
		t.Fatalf("ReadPackage: %v", err)
	}
	if len(pkg.Assets) != 0 || len(pkg.Migrations) != 0 {
		t.Errorf("expected an empty package, got %d assets and %d migrations", len(pkg.Assets), len(pkg.Migrations))
	}
}

func schreiben(t *testing.T, pfad string, inhalt []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(pfad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pfad, inhalt, 0o644); err != nil {
		t.Fatal(err)
	}
}
