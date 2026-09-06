// Was diese Datei bewacht, sind zwei Sätze, die keine Sitte bleiben dürfen.
//
// Der erste: eine angefangene Ablage gehört genau einer Person. Zwei Admins,
// die gleichzeitig importieren, halten zwei Marken und zwei Dateien, und keiner
// von beiden kann die des anderen sehen oder fortsetzen. Das steht in der
// Datenbank — user_id mit ON DELETE CASCADE von users — und wird hier gegen
// einen zweiten Benutzer geprüft und nicht bloss geglaubt.
//
// Der zweite: zwei Tabs zerstören einander nicht. user/token.go:59-62 löscht
// die vorige Marke desselben Benutzers, und dort ist das richtig. Hier wurde
// diese Zeile bewusst nicht abgeschrieben, und eine nicht abgeschriebene Zeile
// ist unsichtbar — deshalb behauptet TestZweiTabsUeberlebenEinander ihre
// Abwesenheit, statt sie dem Kommentar zu überlassen.
package csvimport_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// aufbau öffnet eine gewanderte Datenbank im Testverzeichnis und legt einen
// Benutzer und eine Website an. Die Ablage ist die eine Hälfte dieser Phase,
// die eine Datenbank haben darf — internal/csv ist die andere und hat keine.
func aufbau(t *testing.T) (*csvimport.Store, *db.DB, int64, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return csvimport.NewStore(database), database, benutzer(t, database, "admin@example.com"), website(t, database, "Prüfsite")
}

func benutzer(t *testing.T, database *db.DB, email string) int64 {
	t.Helper()
	res, err := database.Write.Exec(
		`INSERT INTO users (email, password, role, created_at) VALUES ($1, 'hash', 'admin', '2026-01-01T00:00:00Z')`,
		email)
	if err != nil {
		t.Fatalf("Benutzer %s anlegen: %v", email, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func website(t *testing.T, database *db.DB, name string) int64 {
	t.Helper()
	res, err := database.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ($1, '')`, name)
	if err != nil {
		t.Fatalf("Website %s anlegen: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func musterAblage(userID, websiteID int64, daten []byte) csvimport.Ablage {
	return csvimport.Ablage{
		UserID:    userID,
		WebsiteID: websiteID,
		Modus:     "bestehend",
		Kollision: "uebergehen",
		Dateiname: "produkte.csv",
		Daten:     daten,
	}
}

// TestAblageKommtByteFuerByteZurueck: was hochgeladen wurde, kommt so zurück.
//
// Mit der BOM vorne, die Excel schreibt, und mit einem Schwanz aus hohen Bytes.
// Eine Spalte, die den Upload als TEXT führte, würde beides still verändern —
// darum steht in 00049 ein BLOB.
func TestAblageKommtByteFuerByteZurueck(t *testing.T) {
	ctx := context.Background()
	store, _, userID, websiteID := aufbau(t)

	daten := []byte("\xef\xbb\xbfTitel,Text\nStuhl,\"eiche, geölt\"\n")
	for b := 0x80; b <= 0xff; b++ {
		daten = append(daten, byte(b))
	}

	marke, err := store.Stage(ctx, musterAblage(userID, websiteID, daten))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if len(marke) != 32 {
		t.Errorf("Marke ist %d Zeichen lang, erwartet 32 — 128 Bit als Hex", len(marke))
	}

	zurueck, err := store.Holen(ctx, marke, userID)
	if err != nil {
		t.Fatalf("Holen: %v", err)
	}
	if !bytes.Equal(zurueck.Daten, daten) {
		t.Errorf("Daten kommen verändert zurück:\n  hin  = %q\n  zurück = %q", daten, zurueck.Daten)
	}
	if !bytes.HasPrefix(zurueck.Daten, []byte("\xef\xbb\xbf")) {
		t.Error("die BOM ist unterwegs verlorengegangen — sie gehört zur Datei und wird erst vom Leser gestreift")
	}
	if zurueck.WebsiteID != websiteID {
		t.Errorf("WebsiteID = %d, erwartet %d", zurueck.WebsiteID, websiteID)
	}
	if zurueck.Modus != "bestehend" || zurueck.Kollision != "uebergehen" {
		t.Errorf("Modus/Kollision = %q/%q, erwartet \"bestehend\"/\"uebergehen\"", zurueck.Modus, zurueck.Kollision)
	}
	if zurueck.Dateiname != "produkte.csv" {
		t.Errorf("Dateiname = %q", zurueck.Dateiname)
	}
	if zurueck.ErstelltAm.IsZero() {
		t.Error("ErstelltAm ist leer — der Aufräumlauf hängt an dieser Spalte")
	}
}

// TestFremdeMarkeWirdAbgelehnt: der zweite Admin bekommt die Datei des ersten
// nicht, auch wenn er die Marke kennt. (T-09-06)
func TestFremdeMarkeWirdAbgelehnt(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := aufbau(t)
	fremder := benutzer(t, database, "zweiter@example.com")

	marke, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nStuhl\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	if _, err := store.Holen(ctx, marke, fremder); !errors.Is(err, csvimport.ErrFremd) {
		t.Fatalf("Holen mit fremder Marke = %v, erwartet ErrFremd", err)
	}
	// Und der Eigentümer kommt weiterhin heran — die Ablehnung des einen darf
	// den anderen nicht mitnehmen.
	if _, err := store.Holen(ctx, marke, userID); err != nil {
		t.Errorf("Holen durch den Eigentümer: %v", err)
	}
}

// TestUnbekannteMarkeIstAbgelaufen: eine Marke, die es nie gab oder die der
// Aufräumlauf geholt hat, ist abgelaufen und nicht verboten. Die beiden Fälle
// sind unterscheidbar, weil sie zwei verschiedene Leute beschreiben. (D-33)
func TestUnbekannteMarkeIstAbgelaufen(t *testing.T) {
	ctx := context.Background()
	store, _, userID, _ := aufbau(t)

	_, err := store.Holen(ctx, "00000000000000000000000000000000", userID)
	if !errors.Is(err, csvimport.ErrAbgelaufen) {
		t.Fatalf("Holen mit unbekannter Marke = %v, erwartet ErrAbgelaufen", err)
	}
	if errors.Is(err, csvimport.ErrFremd) {
		t.Error("ErrAbgelaufen und ErrFremd sind nicht auseinanderzuhalten — der Bildschirm kann dann nicht sagen, welcher der beiden Fälle vorliegt")
	}
}

// TestZweiTabsUeberlebenEinander behauptet die Abwesenheit von
// user/token.go:59-62.
//
// Ein Admin mit zwei Tabs lädt zweimal hoch. Nach dem zweiten Mal stehen zwei
// Zeilen da und die erste Marke holt weiterhin die erste Datei. Wäre die Zeile
// aus token.go abgeschrieben, wäre die erste Datei jetzt weg — und niemand
// hätte es gemerkt, weil eine gelöschte Ablage wie eine abgelaufene aussieht.
// (D-33)
func TestZweiTabsUeberlebenEinander(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := aufbau(t)

	ersteDaten := []byte("Titel\nErster Tab\n")
	zweiteDaten := []byte("Titel\nZweiter Tab\n")

	ersteMarke, err := store.Stage(ctx, musterAblage(userID, websiteID, ersteDaten))
	if err != nil {
		t.Fatalf("erstes Stage: %v", err)
	}
	zweiteMarke, err := store.Stage(ctx, musterAblage(userID, websiteID, zweiteDaten))
	if err != nil {
		t.Fatalf("zweites Stage: %v", err)
	}
	if ersteMarke == zweiteMarke {
		t.Fatal("beide Marken sind gleich")
	}

	var zeilen int
	if err := database.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM csv_imports WHERE user_id = $1`, userID).Scan(&zeilen); err != nil {
		t.Fatalf("Zeilen zählen: %v", err)
	}
	if zeilen != 2 {
		t.Fatalf("nach zwei Uploads stehen %d Zeilen da, erwartet 2 — die vorige Ablage wurde gelöscht", zeilen)
	}

	erste, err := store.Holen(ctx, ersteMarke, userID)
	if err != nil {
		t.Fatalf("die erste Marke holt nichts mehr: %v", err)
	}
	if !bytes.Equal(erste.Daten, ersteDaten) {
		t.Errorf("die erste Marke holt %q, erwartet %q", erste.Daten, ersteDaten)
	}
	zweite, err := store.Holen(ctx, zweiteMarke, userID)
	if err != nil {
		t.Fatalf("Holen der zweiten Marke: %v", err)
	}
	if !bytes.Equal(zweite.Daten, zweiteDaten) {
		t.Errorf("die zweite Marke holt %q, erwartet %q", zweite.Daten, zweiteDaten)
	}
}

// TestMarkeStehtNichtInDerDatenbank: die Marke selbst steht in keiner Spalte,
// nur ihr SHA-256. Eine Sicherungskopie gibt damit keinen fortsetzbaren Import
// her. (T-09-09, dieselbe Zucht wie 00012)
func TestMarkeStehtNichtInDerDatenbank(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := aufbau(t)

	marke, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nStuhl\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	var hash, name, dateiname, modus, kollision string
	var daten []byte
	if err := database.Read.QueryRowContext(ctx,
		`SELECT token_hash, website_name, dateiname, modus, kollision, daten FROM csv_imports`).
		Scan(&hash, &name, &dateiname, &modus, &kollision, &daten); err != nil {
		t.Fatalf("Zeile lesen: %v", err)
	}
	for spalte, wert := range map[string]string{
		"token_hash":   hash,
		"website_name": name,
		"dateiname":    dateiname,
		"modus":        modus,
		"kollision":    kollision,
		"daten":        string(daten),
	} {
		if bytes.Contains([]byte(wert), []byte(marke)) {
			t.Errorf("die Marke steht in Spalte %s — gespeichert wird nur ihr Hash", spalte)
		}
	}
	if len(hash) != 64 {
		t.Errorf("token_hash ist %d Zeichen lang, erwartet 64 — SHA-256 als Hex", len(hash))
	}
}

// TestLoeschenNimmtGenauEine: nach dem Schreiblauf verschwindet die Ablage, und
// ein Neuladen des Berichts findet keine Marke mehr — statt die Datei ein
// zweites Mal zu importieren. Die Ablage daneben bleibt stehen.
func TestLoeschenNimmtGenauEine(t *testing.T) {
	ctx := context.Background()
	store, _, userID, websiteID := aufbau(t)

	eine, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\neins\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	andere, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nzwei\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	a, err := store.Holen(ctx, eine, userID)
	if err != nil {
		t.Fatalf("Holen: %v", err)
	}
	if err := store.Loeschen(ctx, a.ID); err != nil {
		t.Fatalf("Loeschen: %v", err)
	}
	if _, err := store.Holen(ctx, eine, userID); !errors.Is(err, csvimport.ErrAbgelaufen) {
		t.Errorf("nach dem Löschen = %v, erwartet ErrAbgelaufen", err)
	}
	if _, err := store.Holen(ctx, andere, userID); err != nil {
		t.Errorf("die andere Ablage ist mitgegangen: %v", err)
	}
}

// TestPruneRaeumtNurAltes: was zwei Tage alt ist, geht; was eine Stunde alt
// ist, bleibt. Der Rückgabewert ist die Zahl der geräumten Zeilen, damit der
// Auftrag in main.go dieselbe Form hat wie PurgeExpiredTokens.
func TestPruneRaeumtNurAltes(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := aufbau(t)

	alt, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nalt\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	frisch, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nfrisch\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// Zurückdatieren geht nur hier im Test von aussen: die Ablage selbst
	// schreibt ihre Zeile einmal und ändert sie nie.
	vorZweiTagen := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02T15:04:05Z")
	altAblage, err := store.Holen(ctx, alt, userID)
	if err != nil {
		t.Fatalf("Holen: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`UPDATE csv_imports SET erstellt_am = $1 WHERE id = $2`, vorZweiTagen, altAblage.ID); err != nil {
		t.Fatalf("zurückdatieren: %v", err)
	}

	n, err := store.Prune(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("Prune meldet %d geräumte Zeilen, erwartet 1", n)
	}
	if _, err := store.Holen(ctx, alt, userID); !errors.Is(err, csvimport.ErrAbgelaufen) {
		t.Errorf("die alte Ablage steht noch: %v", err)
	}
	if _, err := store.Holen(ctx, frisch, userID); err != nil {
		t.Errorf("die frische Ablage wurde mitgeräumt: %v", err)
	}
}

// TestBenutzerLoeschenRaeumtAblageMit: ein gelöschtes Konto lässt keine
// verwaisten zehn Megabyte zurück. Das ist eine Tatsache der Datenbank —
// ON DELETE CASCADE von users — und keine Sitte eines Handlers. (T-09-12)
func TestBenutzerLoeschenRaeumtAblageMit(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := aufbau(t)

	if _, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nStuhl\n"))); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("Benutzer löschen: %v", err)
	}

	var zeilen int
	if err := database.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM csv_imports`).Scan(&zeilen); err != nil {
		t.Fatalf("Zeilen zählen: %v", err)
	}
	if zeilen != 0 {
		t.Errorf("nach dem Löschen des Kontos stehen %d Ablagen da, erwartet 0", zeilen)
	}
}

// TestWebsiteLoeschenLaesstAblageStehen ist die Gegenprobe zur vorigen: die
// Zielwebsite verschwindet, die Datei der Bedienerin nicht. website_id fällt
// auf NULL zurück und kommt als 0 an — genau der Zustand, den ein Import hat,
// der die Website erst noch anlegt. Der Ablauf endet dann mit einer Meldung
// und nicht damit, dass der Upload unter den Händen verschwindet. (D-29)
func TestWebsiteLoeschenLaesstAblageStehen(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := aufbau(t)

	marke, err := store.Stage(ctx, musterAblage(userID, websiteID, []byte("Titel\nStuhl\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx, `DELETE FROM websites WHERE id = $1`, websiteID); err != nil {
		t.Fatalf("Website löschen: %v", err)
	}

	zurueck, err := store.Holen(ctx, marke, userID)
	if err != nil {
		t.Fatalf("die Ablage ist mit der Website verschwunden: %v", err)
	}
	if zurueck.WebsiteID != 0 {
		t.Errorf("WebsiteID = %d, erwartet 0 — die gelöschte Website hinterlässt NULL", zurueck.WebsiteID)
	}
	if !bytes.Equal(zurueck.Daten, []byte("Titel\nStuhl\n")) {
		t.Errorf("Daten = %q", zurueck.Daten)
	}
}

// TestAblageOhneWebsiteIstErlaubt: der Import, der die Website erst anlegt, hat
// noch keine — website_id ist mit Absicht leer erlaubt.
func TestAblageOhneWebsiteIstErlaubt(t *testing.T) {
	ctx := context.Background()
	store, _, userID, _ := aufbau(t)

	a := musterAblage(userID, 0, []byte("Titel\nStuhl\n"))
	a.Modus = "neu"
	a.WebsiteName = "Neue Site"

	marke, err := store.Stage(ctx, a)
	if err != nil {
		t.Fatalf("Stage ohne Website: %v", err)
	}
	zurueck, err := store.Holen(ctx, marke, userID)
	if err != nil {
		t.Fatalf("Holen: %v", err)
	}
	if zurueck.WebsiteID != 0 {
		t.Errorf("WebsiteID = %d, erwartet 0", zurueck.WebsiteID)
	}
	if zurueck.WebsiteName != "Neue Site" || zurueck.Modus != "neu" {
		t.Errorf("WebsiteName/Modus = %q/%q", zurueck.WebsiteName, zurueck.Modus)
	}
}
