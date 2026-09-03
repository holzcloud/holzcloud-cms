-- +goose Up

-- Die Seitenliste, wie sie eine bestimmte Person braucht.
--
-- Zwei Kleinigkeiten aus dem Vergleich mit Statamic, die dieselbe Wurzel
-- haben: eine Liste ist das Werkzeug, in dem jemand den halben Tag verbringt,
-- und wessen Arbeit "alle Entwürfe auf Französisch" heisst, der tippt das
-- sonst dreimal täglich in die Filterleiste.

-- Welche Spalten diese Person sehen will, als Kürzel mit Komma getrennt.
--
-- Leer heisst: die Vorgabe. Nicht "keine Spalten" — sonst begänne jeder
-- bestehende Zugang mit einer Liste aus lauter Titeln, und niemand hätte
-- danach gefragt.
ALTER TABLE users ADD COLUMN page_columns TEXT NOT NULL DEFAULT '';

-- Eine gemerkte Ansicht: der Filterteil der Adresse unter einem Namen.
--
-- Je Person und je Website. Nicht je Website allein, denn was eine Ansicht
-- wert ist, hängt an der Arbeit dieses Menschen; und nicht je Person allein,
-- weil "zur Prüfung" auf der einen Website etwas anderes bedeutet als auf der
-- anderen.
CREATE TABLE saved_views (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    -- Der Abfrageteil ohne Fragezeichen: "status=draft&sprache=fr". Gespeichert
    -- wird, was in der Adresse steht, und beim Anzeigen wird daraus wieder eine
    -- Adresse — so kann eine Ansicht nichts erfassen, was die Liste nicht
    -- ohnehin kann.
    query      TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (user_id, website_id, name)
) STRICT;

CREATE INDEX idx_saved_views_owner ON saved_views(user_id, website_id);

-- +goose Down
DROP INDEX IF EXISTS idx_saved_views_owner;
DROP TABLE IF EXISTS saved_views;
ALTER TABLE users DROP COLUMN page_columns;
