-- +goose Up

-- Die Zwischenablage des CSV-Imports.
--
-- Der Import führt über vier Bildschirme: hochladen, zuordnen, Probelauf,
-- schreiben. Die Datei muss alle vier überleben, und dafür gibt es genau einen
-- Weg — sie liegt zwischendurch hier.
--
-- Drei Sätze, die ein späterer Leser braucht:
--
-- Erstens: eine hochgeladene Tabelle kann nicht in der Sitzung mitfahren. Die
-- Sitzungen liegen in derselben SQLite-Datei, und eine Sitzungszeile mit zwei
-- Megabyte Inhalt wird bei jeder einzelnen Anfrage dieses Kontos gelesen und
-- geschrieben. Das ist ein schlechter Tag für die ganze Installation, nicht nur
-- für den Import.
--
-- Zweitens: ein Server kann kein Dateifeld ausfüllen. Der Wert von
-- <input type="file"> ist aus dem Markup heraus nicht setzbar, und ein
-- abgeschicktes Formular lädt ein neues Dokument — das Dateifeld auf Bildschirm
-- 2 käme also leer an. „Die Datei zusammen mit der Zuordnung noch einmal
-- mitschicken" hiesse deshalb: die Bedienerin sucht dieselbe Datei bei jedem
-- Schritt neu heraus, und beim Blättern durch die Beispielzeilen sogar bei
-- jedem Klick. Darum trägt Bildschirm 1 die Datei hierher, und die Bildschirme
-- 2 bis 4 tragen nur noch eine Marke.
--
-- Drittens, und das ist die eigentliche Sorge, die hier gewahrt bleibt: was
-- abgelegt wird, sind die ROHEN BYTES und niemals eine geparste Tabelle. Die
-- Zeile wird einmal geschrieben und danach nie geändert. Daraus folgt, was den
-- Probelauf überhaupt erst glaubwürdig macht: Probelauf und Schreiblauf lesen
-- dieselben Bytes, also kann der Bericht keine andere Datei beschreiben als die,
-- die anschliessend geschrieben wird.
CREATE TABLE csv_imports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Nur der Hash der Marke, nie die Marke selbst. Dieselbe Zucht, die
    -- user_tokens in 00012 führt, und dort steht der Grund in eben diesen
    -- Worten: eine gestohlene Datenbank darf keine funktionierenden Verweise
    -- hergeben. Hier wiegt er schwerer als dort. Eine Einladung führt zu einem
    -- Anmeldeformular; diese Zeile trägt die hochgeladene Datei der Bedienerin
    -- bereits bei sich. Eine Sicherungskopie gibt damit zwar die Bytes preis,
    -- aber keinen fortsetzbaren Import.
    token_hash TEXT NOT NULL UNIQUE,

    -- Hier, und nur hier, hängt die Kaskade nicht an websites.
    --
    -- Das ist Absicht und kein Versehen. Wer die anderen siebenundvierzig
    -- Wanderungen gelesen hat, erwartet an dieser Stelle
    -- websites(id) ON DELETE CASCADE und liest die nächste Zeile sonst als
    -- Fehler. Der Vorgänger ist 00012: user_tokens ist die eine Tabelle in
    -- diesem Baum, die an einem Konto hängt statt an einer Website, und aus
    -- demselben Grund wie diese.
    --
    -- Eine angefangene Ablage gehört der Person, die sie angelegt hat. Das ist
    -- es, was „ein Admin kann den halbfertigen Import eines anderen nicht
    -- fortsetzen" zu einer Tatsache der Datenbank macht statt zu einer Sitte
    -- der Handler — ein fünfter Bildschirm, den jemand später hinzufügt, kann
    -- die Prüfung nicht vergessen. Und es ist der Grund, warum ein gelöschtes
    -- Konto keine verwaisten zehn Megabyte zurücklassen kann.
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Die Zielwebsite. Darf leer sein, und zwar mit Absicht: ein Import, der
    -- eine neue Website anlegt, hat noch keine.
    --
    -- ON DELETE SET NULL und nicht CASCADE. Wird die Zielwebsite mitten im
    -- Ablauf gelöscht, muss der Ablauf mit einer Meldung enden — nicht damit,
    -- dass der Bedienerin die eigene Datei unter den Händen verschwindet. Die
    -- Zeile bleibt also stehen und der Bildschirm sagt, was passiert ist.
    website_id INTEGER REFERENCES websites(id) ON DELETE SET NULL,

    -- Der Name der Website, die angelegt werden soll, wenn website_id leer ist.
    website_name TEXT NOT NULL DEFAULT '',

    -- Ob in eine neue oder in eine bestehende Website importiert wird. Ein
    -- geschlossener Wortschatz gehört in die Spalte und nicht bloss in den
    -- Handler.
    modus TEXT NOT NULL CHECK (modus IN ('neu', 'bestehend')),

    -- Was mit einer Zeile geschieht, deren Adresse es schon gibt: übergehen
    -- oder aktualisieren. Übergehen ist die Vorgabe, weil das Übergehen einer
    -- Zeile rückgängig zu machen ist und das Überschreiben einer Seite nicht.
    kollision TEXT NOT NULL DEFAULT 'uebergehen'
        CHECK (kollision IN ('uebergehen', 'aktualisieren')),

    -- Der Name, unter dem die Datei hochgeladen wurde. Nur damit die Bedienerin
    -- auf Bildschirm 2 sieht, worüber sie gerade redet.
    dateiname TEXT NOT NULL DEFAULT '',

    -- Die rohen Bytes des Uploads, ungeparst. Der Handler begrenzt sie auf zehn
    -- Megabyte, bevor sie überhaupt hierher gelangen. SQLite trägt ein
    -- zehn-Megabyte-BLOB ohne Klage, es wird einmal geschrieben und dreimal
    -- gelesen, und BLOB ist einer der fünf Typen, die STRICT zulässt.
    daten BLOB NOT NULL,

    erstellt_am TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- Der Aufräumlauf fragt genau danach: was ist älter als ein Tag. Ohne den Index
-- ist dieser Lauf ein Durchgang durch die Tabelle mit den grossen BLOBs darin.
CREATE INDEX idx_csv_imports_alter ON csv_imports(erstellt_am);

-- +goose Down

-- Diese Rücknahme ist kurz, und das ist hier die richtige Länge — der Satz
-- steht da, damit die nächste Person, die in diesem Verzeichnis eine
-- Rückwärtshälfte schreibt, den Unterschied sieht. 00047 und 00048 sind
-- Indexaustausche: sie ändern etwas Bestehendes, also muss ihre Rücknahme den
-- Zustand von davor wiederherstellen, und wer dabei die neue Form abschreibt
-- statt der alten, nimmt gar nichts zurück, ohne dass es auffällt. 00048 hat
-- genau dafür einen eigenen Absatz.
--
-- Diese Wanderung legt an und ändert nichts. Es gibt also keinen früheren
-- Zustand, den sie zu weit oder zu kurz wiederherstellen könnte: sie nimmt
-- weg, was sie angelegt hat, und nichts sonst. Zwei Anweisungen sind die
-- vollständige Rücknahme, nicht die abgekürzte.
DROP INDEX IF EXISTS idx_csv_imports_alter;
DROP TABLE IF EXISTS csv_imports;
