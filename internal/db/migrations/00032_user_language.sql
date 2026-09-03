-- +goose Up

-- Die Sprache der Verwaltung, je Person.
--
-- Nicht je Website und nicht je Installation: welche Sprache jemand liest, ist
-- eine Eigenschaft dieses Menschen. Auf derselben Website kann eine deutsche
-- Redaktorin und ein englischsprachiger Entwickler arbeiten, und beide sollen
-- ihre eigene Verwaltung sehen — der Inhalt der Website hat damit nichts zu
-- tun.
--
-- Leer heisst: keine Wahl getroffen. Dann entscheidet Accept-Language, was der
-- Browser ohnehin schon weiss. Ein Vorgabewert 'de' wäre schlechter, denn er
-- liesse sich nicht von "hat sich für Deutsch entschieden" unterscheiden, und
-- jeder bestehende Zugang käme mit einer Entscheidung zur Welt, die niemand
-- getroffen hat.
ALTER TABLE users ADD COLUMN locale TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN locale;
