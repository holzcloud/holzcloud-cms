-- +goose Up

-- Bausteine einer Seite, als JSON.
--
-- Leer heisst: diese Seite ist Markdown, so wie jede Seite es bisher war. Es
-- gibt keine Umstellung, durch die jemand hindurch muss — eine Seite wird zu
-- einer Baustein-Seite, wenn ein Redakteur das für diese eine Seite will.
--
-- JSON in einer Spalte und nicht eine Tabelle mit einer Zeile je Baustein: eine
-- Seite wird immer als Ganzes gelesen und als Ganzes geschrieben, nie ein
-- einzelner Baustein. Zeilen dafür hiessen ein JOIN bei jedem Seitenaufruf und
-- eine Reihenfolgespalte, die irgendwann nicht mehr stimmt.
--
-- content_html bleibt die ausgegebene Seite und content_markdown der reine
-- Text: beide werden aus den Bausteinen erzeugt, damit Auszug, Suche und
-- Kurzfassung weiter funktionieren, ohne von Bausteinen zu wissen.
ALTER TABLE pages ADD COLUMN blocks TEXT NOT NULL DEFAULT '';

-- Auch in den Fassungen, sonst stellt ein Zurücksetzen eine Seite ohne ihre
-- Bausteine wieder her — also leer.
ALTER TABLE page_revisions ADD COLUMN blocks TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE page_revisions DROP COLUMN blocks;
ALTER TABLE pages DROP COLUMN blocks;
