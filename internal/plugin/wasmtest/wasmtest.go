// Package wasmtest entscheidet an einer Stelle, was passiert, wenn ein
// gebautes Plugin-Modul fehlt.
//
// Auf dem Rechner eines Mitwirkenden ist ein fehlendes .wasm kein Fehler: er
// hat vielleicht nur einen Teil des Baums ausgecheckt, und ein Test, der ihn
// dafür bestraft, vertreibt ihn. Auf einem Läufer ist es einer — ein Test, der
// sich selbst überspringt, meldet grün und prüft nichts, und genau das ist der
// falsche Erfolg, den niemand bemerkt, weil er wie ein Erfolg aussieht.
//
// HOLZCLOUD_TEST_REQUIRE_WASM unterscheidet die beiden Fälle. Die drei
// Arbeitsabläufe, die Tests ausführen — ci.yml, security.yml und release.yml —
// setzen die Variable; image.yml führt keine Tests aus und setzt sie nicht.
// Geprüft wird auf »nicht leer«, nicht auf den Wert 1: wer einen Fehler vom
// Läufer bei sich nachstellt, greift zu true oder yes, und ein strenger
// Vergleich würde ihn stillschweigend weiter überspringen lassen — dieselbe
// Lücke, nur eine Ebene tiefer.
//
// Das Paket ist kein Testpaket, weil die fünf Aufrufstellen in drei
// verschiedenen Go-Paketen liegen und ein in einer Testdatei erklärter Helfer
// die anderen beiden nicht erreicht. Es wird von nichts eingebunden, was
// ausgeliefert wird, und trägt deshalb nichts zur Binärdatei bei.
package wasmtest

import (
	"os"
	"testing"
)

// bauhinweis nennt den einen Befehl, der jedes fehlende Modul erzeugt. Er steht
// hier einmal, damit die überspringende und die fehlschlagende Meldung nicht
// auseinanderlaufen können.
const bauhinweis = "gebaut wird es mit: go run ./tools/wasm"

// Modul liest ein gebautes .wasm für einen Test. pfad ist relativ zum
// Verzeichnis des Tests.
//
// Fehlt die Datei, entscheidet HOLZCLOUD_TEST_REQUIRE_WASM: gesetzt (mit
// beliebigem nicht leerem Wert) lässt den Test fehlschlagen, nicht gesetzt
// lässt ihn überspringen. Beide Meldungen nennen den Pfad, den zugrunde
// liegenden Fehler und den Bauhinweis.
func Modul(t *testing.T, pfad string) []byte {
	t.Helper()
	b, err := os.ReadFile(pfad)
	if err == nil {
		return b
	}
	if os.Getenv("HOLZCLOUD_TEST_REQUIRE_WASM") != "" {
		t.Fatalf("%s fehlt und HOLZCLOUD_TEST_REQUIRE_WASM ist gesetzt: %v\n%s", pfad, err, bauhinweis)
	}
	t.Skipf("%s fehlt: %v\n%s", pfad, err, bauhinweis)
	return nil
}
