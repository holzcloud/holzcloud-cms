-- +goose Up

-- Installed plugins.
--
-- One row per plugin, not per website: a plugin is code, and code belongs to
-- the installation. Whether it does anything on a given website is a separate
-- question, answered by plugin_websites below — a hoster running six sites
-- should not have to install the same module six times, and a module that
-- existed twice could be at two versions at once.
--
-- The wasm module itself lives on disk under data/plugins/<id>/, not in a BLOB
-- here. The runtime compiles it from a file at startup, a backup of the
-- database stays small enough to matter, and an operator can see what is
-- installed without a SQL client.
CREATE TABLE plugins (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    version       TEXT NOT NULL,
    -- The manifest verbatim, as it was uploaded. It is the record of what this
    -- plugin declared it would do; parsing it again at startup means the
    -- columns here can never drift from what was actually installed.
    manifest      TEXT NOT NULL,
    -- Over the module. It tells two builds of the same version apart, which is
    -- the first thing anyone wants when a plugin behaves differently than it
    -- did yesterday.
    sha256        TEXT NOT NULL,
    -- Off by default. A plugin that started running the moment it was uploaded
    -- would be a plugin nobody had a chance to look at first.
    enabled       INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    installed_at  TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    -- The last thing that went wrong while loading or running it, or empty.
    -- A plugin that fails to compile has to say so somewhere the admin looks,
    -- rather than being quietly skipped at every startup.
    last_error    TEXT NOT NULL DEFAULT ''
) STRICT;

-- Which websites a plugin acts on.
--
-- Absence means "not on this site". A row that said so explicitly would have to
-- be created for every site that ever exists, and a site added later would
-- silently inherit whatever the migration guessed.
CREATE TABLE plugin_websites (
    plugin_id  TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    PRIMARY KEY (plugin_id, website_id)
) STRICT;

-- The plugin's own storage.
--
-- Deliberately a key/value space and not a schema of its own: a plugin that
-- could create tables could also collide with the core's, and a wasm module has
-- no business holding a SQL connection. What it can reach is exactly its own
-- keys, scoped to one website, enforced by the host on every call.
--
-- website_id 0 is the plugin's global space, for settings that are not about a
-- particular site. Zero rather than NULL so the primary key stays simple and
-- two rows can never differ only by a NULL that compares unequal to itself.
CREATE TABLE plugin_store (
    plugin_id  TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    website_id INTEGER NOT NULL DEFAULT 0,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (plugin_id, website_id, key)
) STRICT;

CREATE INDEX idx_plugin_store_site ON plugin_store(plugin_id, website_id);

-- Which of a plugin's own migrations have run.
--
-- goose keeps its own table for the core's migrations and knows nothing about
-- these; a plugin's SQL is applied by the plugin loader, in name order, once.
-- Recording the checksum as well as the name catches the case where a plugin
-- ships a changed migration under an old name — which would otherwise apply
-- nothing and leave a schema that does not match the code expecting it.
CREATE TABLE plugin_migrations (
    plugin_id  TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    applied_at TEXT NOT NULL,
    PRIMARY KEY (plugin_id, name)
) STRICT;

-- +goose Down
DROP TABLE plugin_migrations;
DROP INDEX idx_plugin_store_site;
DROP TABLE plugin_store;
DROP TABLE plugin_websites;
DROP TABLE plugins;
