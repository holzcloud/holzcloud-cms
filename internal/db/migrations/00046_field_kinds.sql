-- +goose Up
-- Vier Eigenschaften, die drei der neuen Feldarten brauchen und die die
-- Tabelle bisher nirgends unterbringen konnte.
--
-- Jede bekommt eine eigene Spalte und keine reitet in auswahl mit: auswahl
-- wird eine Möglichkeit je Zeile gelesen, und eine Zeile darin, die keine
-- Möglichkeit ist, wäre genau die Zweideutigkeit, die diese Schreibweise
-- vermeiden sollte.
--
-- Keine der vier trägt eine Prüfregel am Spaltenkopf, aus demselben Grund, den
-- die art-Spalte in 00028 nennt: ein neuer Anzeigemodus oder eine neue Art
-- wäre sonst wieder ein Tabellenumbau, und geprüft wird ohnehin beim
-- Speichern.

-- Leer heisst die Klappliste, die es heute schon gibt. Jedes Feld in einer
-- bestehenden Datenbank behält damit sein Aussehen, ohne dass Daten wandern.
ALTER TABLE page_field_defs ADD COLUMN darstellung TEXT NOT NULL DEFAULT '';

-- Null heisst keine Obergrenze — ebenfalls das heutige Verhalten.
ALTER TABLE page_field_defs ADD COLUMN max_werte INTEGER NOT NULL DEFAULT 0;

-- Die beiden Grenzen sind Text und keine Zahlen, weil „keine Grenze" und „die
-- Grenze ist null" zwei verschiedene Tatsachen sind. Eine Zahlenspalte mit
-- einem Vorgabewert könnte die beiden nicht auseinanderhalten; die leere
-- Zeichenkette kann es.
ALTER TABLE page_field_defs ADD COLUMN min_wert TEXT NOT NULL DEFAULT '';
ALTER TABLE page_field_defs ADD COLUMN max_wert TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE page_field_defs DROP COLUMN max_wert;
ALTER TABLE page_field_defs DROP COLUMN min_wert;
ALTER TABLE page_field_defs DROP COLUMN max_werte;
ALTER TABLE page_field_defs DROP COLUMN darstellung;
