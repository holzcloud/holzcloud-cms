-- +goose Up

-- Was eine Person darf, jenseits ihrer Rolle.
--
-- Zwei Rollen sind für eine Installation dieser Grösse richtig: Administrator
-- für die Anlage selbst, Redakteur für die Inhalte. Was in der Praxis fehlt,
-- sind nicht mehr Rollen, sondern zwei Einschränkungen innerhalb der einen:
--
--   1. Welche Websites jemand betreten darf.
--   2. Ob jemand veröffentlichen darf oder nur einreichen.
--
-- Beide sind Rechte an einem Menschen und nicht an einer Rolle. Ein Verein hat
-- eine Person, die den Vorstandsbereich pflegt, und eine, die schreibt und
-- deren Text jemand ansieht — mit einer Rollenliste liesse sich das nur
-- abbilden, indem man für jede Kombination eine Rolle erfindet.

-- Darf veröffentlichen. Vorgabe ja, denn jeder bestehende Zugang konnte es
-- bisher, und eine Migration, die Leuten still etwas wegnimmt, ist eine
-- Migration, nach der montags niemand mehr arbeiten kann.
ALTER TABLE users ADD COLUMN may_publish INTEGER NOT NULL DEFAULT 1;

-- Die Websites, die jemand betreten darf.
--
-- Keine Zeile heisst: alle. Nicht "keine" — sonst wäre die Migration selbst
-- eine Aussperrung, und der Betreiber, der diese Funktion nie benutzt, merkt
-- von ihr nichts. Wer eine Zeile bekommt, ist ab da auf das eingeschränkt, was
-- dasteht.
CREATE TABLE user_websites (
    user_id    INTEGER NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, website_id)
) STRICT;

-- Für die Frage "wer darf auf diese Website?", die die Benutzerliste stellt.
CREATE INDEX idx_user_websites_website ON user_websites(website_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_websites_website;
DROP TABLE IF EXISTS user_websites;
ALTER TABLE users DROP COLUMN may_publish;
