-- +goose Up

-- Website settings. Deliberately extra columns on websites rather than a 1:1
-- settings table: a LEFT JOIN plus lazy insert would buy nothing here and every
-- read path already selects the website row.
--
-- The defaults are German because that is what this CMS is for; they apply to
-- existing rows automatically, so no backfill is needed.
ALTER TABLE websites ADD COLUMN locale TEXT NOT NULL DEFAULT 'de';
ALTER TABLE websites ADD COLUMN timezone TEXT NOT NULL DEFAULT 'Europe/Berlin';
ALTER TABLE websites ADD COLUMN meta_description TEXT NOT NULL DEFAULT '';
ALTER TABLE websites ADD COLUMN favicon_media_id INTEGER REFERENCES media(id) ON DELETE SET NULL;
ALTER TABLE websites ADD COLUMN logo_media_id INTEGER REFERENCES media(id) ON DELETE SET NULL;

-- Canonical redirect is off by default so a fresh install reachable only over a
-- bare IP does not redirect itself into a loop.
ALTER TABLE websites ADD COLUMN canonical_redirect INTEGER NOT NULL DEFAULT 0
    CHECK (canonical_redirect IN (0, 1));

-- Taking a site offline currently answers 404, which tells a search engine the
-- pages are gone for good. 'maintenance' answers 503 instead.
ALTER TABLE websites ADD COLUMN offline_mode TEXT NOT NULL DEFAULT 'notfound'
    CHECK (offline_mode IN ('notfound', 'maintenance'));
ALTER TABLE websites ADD COLUMN offline_message TEXT NOT NULL DEFAULT '';

-- is_primary had a CHECK but nothing stopped a website from having five primary
-- domains. Collapse any existing duplicates to the oldest row first, otherwise
-- the unique index cannot be created.
UPDATE website_domains SET is_primary = 0
WHERE is_primary = 1
  AND id NOT IN (
      SELECT MIN(id) FROM website_domains WHERE is_primary = 1 GROUP BY website_id
  );

CREATE UNIQUE INDEX idx_website_domains_one_primary
    ON website_domains(website_id) WHERE is_primary = 1;

-- Page metadata. Without these there is no field a theme could read for a
-- description or a preview image, which is why no layout emits either.
ALTER TABLE pages ADD COLUMN excerpt TEXT NOT NULL DEFAULT '';
ALTER TABLE pages ADD COLUMN meta_description TEXT NOT NULL DEFAULT '';
ALTER TABLE pages ADD COLUMN featured_media_id INTEGER REFERENCES media(id) ON DELETE SET NULL;
ALTER TABLE pages ADD COLUMN noindex INTEGER NOT NULL DEFAULT 0 CHECK (noindex IN (0, 1));

-- +goose Down
ALTER TABLE pages DROP COLUMN noindex;
ALTER TABLE pages DROP COLUMN featured_media_id;
ALTER TABLE pages DROP COLUMN meta_description;
ALTER TABLE pages DROP COLUMN excerpt;
DROP INDEX IF EXISTS idx_website_domains_one_primary;
ALTER TABLE websites DROP COLUMN offline_message;
ALTER TABLE websites DROP COLUMN offline_mode;
ALTER TABLE websites DROP COLUMN canonical_redirect;
ALTER TABLE websites DROP COLUMN logo_media_id;
ALTER TABLE websites DROP COLUMN favicon_media_id;
ALTER TABLE websites DROP COLUMN meta_description;
ALTER TABLE websites DROP COLUMN timezone;
ALTER TABLE websites DROP COLUMN locale;
