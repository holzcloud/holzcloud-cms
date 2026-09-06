-- +goose Up

-- Eine Klausel, die 00047 gefehlt hat.
--
-- 00047 zog idx_page_field_defs_kennung_textbaustein als
--     ON page_field_defs(snippet_id, kennung) WHERE snippet_id IS NOT NULL
-- und schrieb dabei die Form aus 00038 ab. Dort stimmt sie: eine Bausteinart
-- kann keine Gruppe tragen, also gibt es unter ihr keine zweite Ebene, die der
-- Teilindex mitfassen könnte. Ein Textbaustein kann eine Gruppe tragen — das
-- ist gerade, was Phase 8 beschlossen hat —, und seit die Unterfelder einer
-- Gruppe ihren Träger von der Gruppe erben, tragen sie snippet_id. Der
-- Teilindex fasste sie damit mit ein, und die Unterfelder fielen in denselben
-- Namensraum wie die Felder der obersten Ebene.
--
-- Was das an einem Bildschirm heisst: wer an einem Textbaustein
-- „Öffnungszeiten{tag, von}" und „Ferien{tag, von}" anlegt, wird bei der
-- zweiten Gruppe mit „Ein Feld mit dieser Kennung gibt es schon" abgewiesen.
-- Auf einer Seite geht dasselbe seit 00029 durch. 00047 sagt in seinem eigenen
-- Kommentar „Vier Namensräume, und keiner von ihnen trifft einen anderen" —
-- der vierte traf die Namensräume seiner eigenen Gruppen.
--
-- Die Eindeutigkeit innerhalb einer Gruppe geht dabei nicht verloren: dafür
-- steht seit 00029 idx_page_field_defs_kennung_gruppe
--     ON page_field_defs(parent_id, kennung) WHERE parent_id IS NOT NULL
-- und deckt jedes Unterfeld ab, gleich an welchem Träger seine Gruppe hängt.
-- Der Teilindex hier wird also enger gezogen und nicht weggenommen.
--
-- Wieder ein Austausch von zwei Indizes und kein Tabellenneubau — zum fünften
-- Mal derselbe Handgriff. Eine ausgelieferte Wanderung wird nicht bearbeitet;
-- eine Berichtigung ist eine eigene Datei, und das ist diese.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_textbaustein;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL AND parent_id IS NULL;

-- +goose Down

-- Achtung, und darum steht dieser Satz hier: wiederhergestellt wird die Form
-- aus 00047 — WHERE snippet_id IS NOT NULL, ohne parent_id —, denn genau die
-- hat diese Wanderung ersetzt. 00047 hat dieselbe Falle bei seiner eigenen
-- Rücknahme umgangen und den Grund aufgeschrieben; hier ist es der Index
-- daneben. Wer stattdessen die neue Form abschreibt, nimmt gar nichts zurück,
-- und nichts würde das melden.
--
-- Die Rücknahme kann fehlschlagen, und das ist richtig so: sind unter diesem
-- Textbaustein inzwischen zwei Gruppen mit derselben Unterkennung angelegt,
-- lässt sich der engere Index nicht mehr über die Daten ziehen. Der Fehler ist
-- dann die Wahrheit — die Rücknahme würde Daten verlieren, die es unter 00047
-- nicht geben konnte.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_textbaustein;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL;
