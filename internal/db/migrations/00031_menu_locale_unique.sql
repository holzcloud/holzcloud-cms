-- +goose Up

-- Ein Hauptmenü je Sprache.
--
-- 00030 hat den Menüs eine Sprache gegeben, aber der Zwang aus 00005 steht
-- weiterhin auf (website_id, location_key): es kann pro Website genau ein
-- Menü an der Stelle "main" geben. Genau das ist mit zwei Sprachen falsch —
-- das deutsche und das französische Hauptmenü stehen beide an "main", sie
-- unterscheiden sich nur in der Sprache.
--
-- Eine Bedingung am Tabellenkopf lässt sich in SQLite nicht ändern, die
-- Tabelle muss neu gebaut werden. Heikel ist dabei nur eines: DROP TABLE
-- führt bei eingeschalteten Fremdschlüsseln ein stilles DELETE FROM aus, und
-- menu_items hängt mit ON DELETE CASCADE an menus — ein naives Neubauen
-- löschte also sämtliche Menüeinträge. Deshalb wird menu_items mitgebaut und
-- vor menus abgeräumt, damit beim Löschen von menus nichts mehr daran hängt.

-- +goose StatementBegin
CREATE TABLE menus_neu (
    id           INTEGER PRIMARY KEY,
    website_id   INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    location_key TEXT NOT NULL,
    locale       TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(website_id, location_key, locale)
) STRICT;
-- +goose StatementEnd

INSERT INTO menus_neu (id, website_id, name, location_key, locale, created_at)
SELECT id, website_id, name, location_key, locale, created_at FROM menus;

-- +goose StatementBegin
CREATE TABLE menu_items_neu (
    id         INTEGER PRIMARY KEY,
    menu_id    INTEGER NOT NULL REFERENCES menus_neu(id) ON DELETE CASCADE,
    parent_id  INTEGER REFERENCES menu_items_neu(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    item_type  TEXT NOT NULL DEFAULT 'url',
    url        TEXT NOT NULL DEFAULT '',
    page_id    INTEGER REFERENCES pages(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;
-- +goose StatementEnd

INSERT INTO menu_items_neu
    (id, menu_id, parent_id, title, item_type, url, page_id, sort_order, created_at)
SELECT id, menu_id, parent_id, title, item_type, url, page_id, sort_order, created_at
FROM menu_items;

DROP INDEX IF EXISTS idx_menus_locale;
DROP TABLE menu_items;
DROP TABLE menus;

-- Das Umbenennen zieht die Fremdschlüssel mit: menu_items_neu zeigt danach auf
-- menus, nicht mehr auf menus_neu.
ALTER TABLE menus_neu RENAME TO menus;
ALTER TABLE menu_items_neu RENAME TO menu_items;

CREATE INDEX idx_menus_locale ON menus(website_id, locale, location_key);
CREATE INDEX idx_menu_items_menu ON menu_items(menu_id, sort_order);

-- +goose Down
DROP INDEX IF EXISTS idx_menu_items_menu;
-- Der Zwang wird nicht zurückgebaut: dazu müssten Menüs gelöscht werden, die
-- es unter der alten Bedingung nicht geben durfte.
