-- +goose Up

-- Abschnitte und Bedingungen im Feldformular.
--
-- Ein Formular mit zwanzig eigenen Feldern ist eine Wand. Zwei Dinge machen
-- daraus etwas, das jemand ausfüllt:
--
--   Ein *Abschnitt* ist eine Überschrift zwischen den Feldern — "Preis und
--   Verfügbarkeit", "Masse". Er ist selbst ein Feld mit der Art "abschnitt",
--   nur eben eines ohne Wert. Eine eigene Tabelle hätte dieselbe Reihenfolge,
--   dasselbe Verschieben und dieselbe Rechteprüfung noch einmal gebraucht.
--
--   Eine *Bedingung* lässt ein Feld erst erscheinen, wenn ein anderes
--   ausgefüllt ist: "Sonderpreis" erst, wenn "Im Angebot" angekreuzt ist.
--   bedingung trägt die Kennung des Feldes, an dem es hängt, oder ist leer.
--
-- Kein CHECK und kein Fremdschlüssel auf die Kennung: sie ist innerhalb einer
-- Website eindeutig, aber nicht in dieser Tabelle als Ganzes (siehe 00029,
-- die Teilindizes), und ein Feld, dessen Bedingung ins Leere zeigt, soll nicht
-- die Zeile blockieren, sondern schlicht immer sichtbar sein. Geprüft wird
-- beim Speichern.
ALTER TABLE page_field_defs ADD COLUMN bedingung TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE page_field_defs DROP COLUMN bedingung;
