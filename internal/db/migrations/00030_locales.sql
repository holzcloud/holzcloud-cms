-- +goose Up

-- Mehrsprachigkeit innerhalb einer Website.
--
-- Eine Website hat eine Hauptsprache — die steht seit jeher in websites.locale —
-- und ab hier beliebig viele weitere. Die Hauptsprache behält ihre Adressen
-- unverändert (/hofladen), jede weitere bekommt ein Präfix (/fr/magasin). Das
-- ist die eine Entscheidung, an der alles andere hängt: kein bestehender Link
-- ändert sich, wenn jemand eine zweite Sprache einschaltet.
ALTER TABLE websites ADD COLUMN extra_locales TEXT NOT NULL DEFAULT '';

-- Die Sprache einer Seite. Leer heisst Hauptsprache — damit ist jede
-- bestehende Seite ohne Umschreiben richtig eingeordnet.
ALTER TABLE pages ADD COLUMN locale TEXT NOT NULL DEFAULT '';

-- translation_of zeigt auf die Seite in der Hauptsprache. NULL heisst: das ist
-- sie selbst, oder die Seite hat keine Übersetzungen.
--
-- Ein Stern und keine Kette: alle Übersetzungen zeigen auf dieselbe Mitte.
-- Zeigte jede auf die vorige, wäre die Reihenfolge des Anlegens dauerhaft
-- eingebaut, und das Löschen einer mittleren Sprache zerrisse die Gruppe.
--
-- ON DELETE SET NULL, nicht CASCADE: wer die deutsche Seite löscht, wollte
-- nicht auch die französische löschen. Sie steht dann für sich.
ALTER TABLE pages ADD COLUMN translation_of INTEGER REFERENCES pages(id) ON DELETE SET NULL;

CREATE INDEX idx_pages_locale ON pages(website_id, locale, slug);
CREATE INDEX idx_pages_translation ON pages(translation_of);

-- Menüs gibt es je Sprache: eine französische Seite mit deutschen Menütiteln
-- ist keine französische Seite. Leer heisst wieder Hauptsprache.
ALTER TABLE menus ADD COLUMN locale TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_menus_locale ON menus(website_id, locale, location_key);

-- +goose Down
DROP INDEX IF EXISTS idx_menus_locale;
ALTER TABLE menus DROP COLUMN locale;
DROP INDEX IF EXISTS idx_pages_translation;
DROP INDEX IF EXISTS idx_pages_locale;
ALTER TABLE pages DROP COLUMN translation_of;
ALTER TABLE pages DROP COLUMN locale;
ALTER TABLE websites DROP COLUMN extra_locales;
