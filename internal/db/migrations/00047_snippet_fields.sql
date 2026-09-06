-- +goose Up

-- Eigene Felder an einem Textbaustein.
--
-- Ein Textbaustein war bisher ein Markdown-Kasten. Die Adresse, die
-- Öffnungszeiten, die Kontaktzeile — alles, was mehrfach auf einer Website
-- steht — lag darin als ein Stück Fliesstext. Wer die Telefonnummer allein
-- ansprechen wollte, konnte es nicht: das Theme bekam einen Block HTML und
-- keine Nummer.
--
-- Ein Textbaustein wird deshalb zum vierten Träger von Feldern, neben der
-- Seite, der Gruppe und der Bausteinart. Die Felder stehen wieder in
-- page_field_defs. Eine fünfte Feldtabelle hätte dieselben Spalten, dieselben
-- Prüfungen, dieselbe Reihenfolge-Logik und dieselben Eingabemasken noch
-- einmal gebraucht — genau die Überlegung, aus der in 00029 die Gruppen und in
-- 00038 die Bausteinarten in dieselbe Tabelle gewandert sind.

-- Die Spalte darf keinen anderen Vorgabewert als NULL haben — SQLite lässt
-- ALTER TABLE ADD COLUMN nicht zu, wenn REFERENCES und ein Vorgabewert
-- ungleich NULL zusammentreffen. Verboten ist also das Paar und nicht die
-- Verweisklausel für sich; snippet_id bekommt keinen Vorgabewert, deshalb ist
-- der Fremdschlüssel hier zu haben und die Beziehung steht in der Datenbank
-- statt in Go.
--
-- NULL heisst „gehört keinem Textbaustein", und das trifft auf jede bestehende
-- Zeile zu.
ALTER TABLE page_field_defs
    ADD COLUMN snippet_id INTEGER REFERENCES snippets(id) ON DELETE CASCADE;

-- Die Eindeutigkeit wird neu gezogen. Sie steht seit 00029 als eigener Index
-- da und nicht am Tabellenkopf — deshalb ist das hier ein Austausch von zwei
-- Indizes und kein Tabellenneubau. Zum vierten Mal derselbe Handgriff.
--
-- Was die zusätzliche Bedingung einbringt: „telefon" darf es in den
-- Seitenfeldern einmal geben, in jeder Bausteinart noch einmal und an jedem
-- Textbaustein noch einmal. Vier Namensräume, und keiner von ihnen trifft
-- einen anderen.
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;

CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL AND snippet_id IS NULL;

-- Das Gegenstück zu idx_page_field_defs_kennung_baustein aus 00038, und
-- absichtlich anders benannt: „Baustein" heisst im Baum die Bausteinart,
-- „Textbaustein" der wiederverwendbare Textblock. Ein Leser soll nicht raten
-- müssen, welcher der beiden gemeint ist.
CREATE UNIQUE INDEX idx_page_field_defs_kennung_textbaustein
    ON page_field_defs(snippet_id, kennung)
    WHERE snippet_id IS NOT NULL;

-- Dieselbe Spalte, die pages schon trägt: darin steht, was field.Encode
-- schreibt. Die leere Zeichenkette liest field.Decode als „nichts ausgefüllt",
-- und genau das ist jeder bestehende Textbaustein.
--
-- snippets ist STRICT, also ist TEXT NOT NULL DEFAULT '' die einzige Form, die
-- STRICT erfüllt und zugleich jede bestehende Zeile gültig lässt.
ALTER TABLE snippets ADD COLUMN fields TEXT NOT NULL DEFAULT '';

-- +goose Down
DROP INDEX IF EXISTS idx_page_field_defs_kennung_textbaustein;
DROP INDEX IF EXISTS idx_page_field_defs_kennung_oben;

-- Gelöscht wird nur, was es ohne diese Wanderung nicht geben konnte. Die
-- Felder der Seite, der Gruppen und der Bausteinarten bleiben unangetastet.
DELETE FROM page_field_defs WHERE snippet_id IS NOT NULL;

ALTER TABLE page_field_defs DROP COLUMN snippet_id;

-- Achtung, und darum steht dieser Satz hier: der Index wird in der Form aus
-- 00038 wiederhergestellt, mit block_type_id IS NULL. 00038 stellt in seinem
-- eigenen Down die Form aus 00029 her — nur WHERE parent_id IS NULL —, und wer
-- diese Zeile von dort abschreibt, rollt die Datenbank eine Wanderung weiter
-- zurück, als diese hier gegangen ist. Nichts würde das melden.
CREATE UNIQUE INDEX idx_page_field_defs_kennung_oben
    ON page_field_defs(website_id, kennung)
    WHERE parent_id IS NULL AND block_type_id IS NULL;

ALTER TABLE snippets DROP COLUMN fields;
