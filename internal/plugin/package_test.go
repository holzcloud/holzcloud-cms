package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// gutesManifest ist die Vorlage, von der jeder Test das Eine abweichen lässt,
// das er prüfen will.
func gutesManifest() Manifest {
	return Manifest{
		ID: "weiterleitungen", ABI: ABIVersion,
		Name: "Weiterleitungen", Version: "1.0.0",
		Hooks:       []string{HookRequest, HookAdmin},
		Permissions: []string{PermStore},
		Admin:       &AdminEntry{Label: "Weiterleitungen", PerWebsite: true},
	}
}

func archiv(t *testing.T, m any, dateien map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if m != nil {
		w, err := zw.Create(ManifestName)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(w).Encode(m); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range dateien {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// wasm ist das Kleinste, was der Prüfung als Modul durchgeht.
var wasm = []byte("\x00asm\x01\x00\x00\x00")

func lies(t *testing.T, data []byte) (*Package, error) {
	t.Helper()
	return ReadPackage(bytes.NewReader(data), int64(len(data)))
}

func TestGutesPaketWirdGelesen(t *testing.T) {
	a := archiv(t, gutesManifest(), map[string][]byte{
		ModuleName:                    wasm,
		AssetDir + "stil.css":         []byte("a{}"),
		AssetDir + "bild/logo.svg":    []byte("<svg/>"),
		MigrationDir + "0001_tab.sql": []byte("CREATE TABLE x(a);"),
		MigrationDir + "0002_idx.sql": []byte("CREATE INDEX y ON x(a);"),
	})
	p, err := lies(t, a)
	if err != nil {
		t.Fatalf("ReadPackage: %v", err)
	}
	if p.Manifest.ID != "weiterleitungen" {
		t.Errorf("Kennung: %q", p.Manifest.ID)
	}
	if len(p.Assets) != 2 || p.Assets["bild/logo.svg"] == nil {
		t.Errorf("Beigaben: %v", p.Assets)
	}
	// Die Reihenfolge muss der Name bestimmen und nicht das Archiv.
	if len(p.Migrations) != 2 || p.Migrations[0].Name != "0001_tab.sql" {
		t.Errorf("Migrationen: %+v", p.Migrations)
	}
	if len(p.SHA256) != 64 {
		t.Errorf("Prüfsumme: %q", p.SHA256)
	}
}

func TestPaketOhneModulOderManifest(t *testing.T) {
	t.Run("ohne Modul", func(t *testing.T) {
		if _, err := lies(t, archiv(t, gutesManifest(), nil)); err == nil ||
			!strings.Contains(err.Error(), ModuleName) {
			t.Errorf("erwartet: fehlendes Modul, bekommen: %v", err)
		}
	})
	t.Run("ohne Manifest", func(t *testing.T) {
		if _, err := lies(t, archiv(t, nil, map[string][]byte{ModuleName: wasm})); err == nil {
			t.Error("ein Paket ohne Manifest wurde angenommen")
		}
	})
	t.Run("Modul ist kein WebAssembly", func(t *testing.T) {
		a := archiv(t, gutesManifest(), map[string][]byte{ModuleName: []byte("#!/bin/sh\nrm -rf /")})
		if _, err := lies(t, a); err == nil || !strings.Contains(err.Error(), "WebAssembly") {
			t.Errorf("erwartet: kein WebAssembly, bekommen: %v", err)
		}
	})
}

func TestPaketMitEntkommendemPfad(t *testing.T) {
	// Zip-Slip durch die dritte Tür. Der Vorlagen-Upload und der Bundle-Import
	// sind je einzeln dagegen abgesichert; hier muss es auch halten.
	for _, name := range []string{
		AssetDir + "../../etc/holzcloud.conf",
		AssetDir + "../heimlich.txt",
		AssetDir + "/absolut.txt",
		MigrationDir + "../boese.sql",
		AssetDir + ".versteckt",
	} {
		t.Run(name, func(t *testing.T) {
			a := archiv(t, gutesManifest(), map[string][]byte{ModuleName: wasm, name: []byte("x")})
			if _, err := lies(t, a); err == nil {
				t.Errorf("der Pfad %q wurde angenommen", name)
			}
		})
	}
}

func TestUnbekannteDateienWerdenAbgelehnt(t *testing.T) {
	// Nicht überspringen: eine Datei, die der Host nicht kennt, ist entweder
	// ein Fehler im Bau oder sollte irgendwo landen, wo sie nichts zu suchen hat.
	a := archiv(t, gutesManifest(), map[string][]byte{
		ModuleName: wasm, "README.md": []byte("hallo"),
	})
	if _, err := lies(t, a); err == nil || !strings.Contains(err.Error(), "README.md") {
		t.Errorf("erwartet: unbekannte Datei genannt, bekommen: %v", err)
	}
}

func TestMigrationenNurFlachUndAlsSQL(t *testing.T) {
	for _, name := range []string{MigrationDir + "unter/ordner.sql", MigrationDir + "kein.txt"} {
		a := archiv(t, gutesManifest(), map[string][]byte{ModuleName: wasm, name: []byte("x")})
		if _, err := lies(t, a); err == nil {
			t.Errorf("%q wurde angenommen", name)
		}
	}
}

func TestManifestLehntUnbekannteFelderAb(t *testing.T) {
	// Ein vertipptes Feld würde sonst still verworfen: das Plugin liesse sich
	// einspielen, liefe, und scheiterte beim ersten Aufruf an einer
	// Berechtigung, die niemand erklären kann.
	roh := []byte(`{"id":"x","abi":1,"name":"X","version":"1.0.0",
	                "hooks":["content"],"permissons":["store"]}`)
	if _, err := ParseManifest(roh); err == nil ||
		!strings.Contains(err.Error(), "permissons") {
		t.Errorf("erwartet: unbekanntes Feld genannt, bekommen: %v", err)
	}
}

func TestManifestPrueftJedesFeld(t *testing.T) {
	faelle := []struct {
		name    string
		aendern func(*Manifest)
		suche   string
	}{
		{"Kennung mit Grossbuchstaben", func(m *Manifest) { m.ID = "Weiterleitungen" }, "Kennung"},
		{"Kennung mit Schrägstrich", func(m *Manifest) { m.ID = "a/b" }, "Kennung"},
		{"Kennung reserviert", func(m *Manifest) { m.ID = "admin" }, "reserviert"},
		{"falsche Schnittstelle", func(m *Manifest) { m.ABI = 99 }, "Schnittstelle"},
		{"kein Name", func(m *Manifest) { m.Name = "" }, "Anzeigename"},
		{"krumme Fassung", func(m *Manifest) { m.Version = "eins" }, "Versionsnummer"},
		{"unbekannter Haken", func(m *Manifest) { m.Hooks = []string{"irgendwas"} }, "Haken"},
		{"unbekannte Berechtigung", func(m *Manifest) { m.Permissions = []string{"root"} }, "Berechtigung"},
		{"Adresse ohne Schrägstrich", func(m *Manifest) {
			m.Hooks = append(m.Hooks, HookRoute)
			m.Routes = []string{"suche"}
		}, "Schrägstrich"},
		{"Adresse gehört dem Server", func(m *Manifest) {
			m.Hooks = append(m.Hooks, HookRoute)
			m.Routes = []string{"/feed.xml"}
		}, "gehört bereits"},
		{"Adressen ohne Haken", func(m *Manifest) { m.Routes = []string{"/suche"} }, "Haken"},
		{"Verwaltung ohne Haken", func(m *Manifest) {
			m.Hooks = []string{HookRequest}
			m.Admin = &AdminEntry{Label: "X"}
		}, "Verwaltung"},
		{"javascript-Adresse", func(m *Manifest) { m.URL = "javascript:alert(1)" }, "Adresse"},
		{"weder Haken noch Adresse", func(m *Manifest) { m.Hooks = nil; m.Admin = nil }, "nie aufgerufen"},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			m := gutesManifest()
			f.aendern(&m)
			err := m.Validate()
			if err == nil {
				t.Fatal("angenommen, obwohl fehlerhaft")
			}
			if !strings.Contains(err.Error(), f.suche) {
				t.Errorf("die Meldung nennt %q nicht: %v", f.suche, err)
			}
		})
	}
}

func TestZuVieleDateien(t *testing.T) {
	dateien := map[string][]byte{ModuleName: wasm}
	for i := 0; i < MaxEntries+5; i++ {
		dateien[AssetDir+string(rune('a'+i%26))+string(rune('a'+i/26))+".txt"] = []byte("x")
	}
	if _, err := lies(t, archiv(t, gutesManifest(), dateien)); err == nil {
		t.Error("ein Archiv mit zu vielen Dateien wurde angenommen")
	}
}
