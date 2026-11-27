package plugin

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto/sha256"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

const timeLayout = "2006-01-02T15:04:05Z"

// ErrNotFound is returned when a plugin is not installed.
var ErrNotFound = errors.New("plugin not installed")

// ErrNamespaceTaken is returned when another plugin's table names would overlap.
var ErrNamespaceTaken = errors.New("the table names of another plugin overlap")

// ErrRouteTaken is returned when another plugin already claims a route.
var ErrRouteTaken = errors.New("another plugin already claims that route")

// Installed is one plugin as the database knows it.
type Installed struct {
	ID       string
	Name     string
	Version  string
	Manifest *Manifest
	SHA256   string
	Enabled  bool
	// Websites are the sites this plugin acts on. Empty means it is installed
	// but idle, which is a legitimate state and not an error.
	Websites    []int64
	InstalledAt time.Time
	UpdatedAt   time.Time
	// LastError is what went wrong the last time it was loaded or called.
	LastError string
}

// Store keeps the record of what is installed.
//
// It holds no wasm and runs nothing. The runtime reads from here and compiles
// modules from disk; keeping the two apart means a broken module can be
// disabled and removed through ordinary SQL even when it cannot be loaded.
type Store struct {
	DB *db.DB

	// removeGuard takes the migration policy off a driver connection. Nil means
	// the real one; only a test sets it, to make the removal fail.
	removeGuard func(driverConn any) error

	// migrationTimeout overrides MigrationTimeout. Only a test sets it.
	migrationTimeout time.Duration
}

// NewStore creates a store.
func NewStore(database *db.DB) *Store { return &Store{DB: database} }

// MaxValueBytes bounds one entry in a plugin's key/value space.
//
// A plugin that could write without limit could fill the disk of a machine it
// shares with the site it is supposed to serve.
const MaxValueBytes = 256 << 10

// MaxKeyBytes bounds a key.
const MaxKeyBytes = 200

// Install records a package, replacing an earlier version of the same plugin.
//
// The routes are checked against every other installed plugin first. A
// collision found here is an upload that fails with a message; found later it
// is two plugins that both believe they own /suche, one of which silently never
// runs.
//
// The checks and the insert are one write transaction. Checked through the
// read pool first, two installs at the same moment (nicht and nicht-gefunden,
// or two plugins claiming /suche) could both pass before either was written.
func (s *Store) Install(ctx context.Context, p *Package) error {
	raw, err := json.Marshal(p.Manifest)
	if err != nil {
		return err
	}
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := checkRoutes(ctx, tx, p.Manifest); err != nil {
		return err
	}
	if err := checkNamespaces(ctx, tx, p.Manifest.ID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(timeLayout)

	// An upgrade keeps the enabled state and the websites: an operator who
	// updates a plugin expects it to keep working, not to switch itself off.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO plugins (id, name, version, manifest, sha256, enabled, installed_at, updated_at, last_error)
		VALUES ($1, $2, $3, $4, $5, 0, $6, $6, '')
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, version = excluded.version,
			manifest = excluded.manifest, sha256 = excluded.sha256,
			updated_at = excluded.updated_at, last_error = ''`,
		p.Manifest.ID, p.Manifest.Name, p.Manifest.Version, string(raw), p.SHA256, now); err != nil {
		return err
	}
	return tx.Commit()
}

// checkRoutes refuses a manifest whose routes another plugin already holds.
func checkRoutes(ctx context.Context, tx *sql.Tx, m *Manifest) error {
	if len(m.Routes) == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT name, manifest FROM plugins WHERE id <> $1`, m.ID)
	if err != nil {
		return fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()
	claimed := map[string]string{}
	for rows.Next() {
		var name, raw string
		if err := rows.Scan(&name, &raw); err != nil {
			return err
		}
		// A manifest that no longer parses claims nothing, as in List.
		o, err := ParseManifest([]byte(raw))
		if err != nil {
			continue
		}
		for _, r := range o.Routes {
			claimed[r] = name
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range m.Routes {
		if by, ok := claimed[r]; ok {
			return fmt.Errorf("%w: %q already belongs to %q", ErrRouteTaken, r, by)
		}
	}
	return nil
}

// checkNamespaces refuses an id whose table prefix starts another installed
// plugin's, or the other way round. Ids may contain hyphens, which become
// underscores in the prefix: plugin_nicht_ is the beginning of
// plugin_nicht_gefunden_, so a plugin called nicht could otherwise reach the
// tables of nicht-gefunden by its prefix alone.
func checkNamespaces(ctx context.Context, tx *sql.Tx, id string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM plugins WHERE id <> $1`, id)
	if err != nil {
		return fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()
	mine := migrationPrefix(id)
	for rows.Next() {
		var other string
		if err := rows.Scan(&other); err != nil {
			return err
		}
		theirs := migrationPrefix(other)
		if strings.HasPrefix(theirs, mine) || strings.HasPrefix(mine, theirs) {
			return fmt.Errorf("%w: plugin %q and plugin %q would share table names (%s and %s)",
				ErrNamespaceTaken, id, other, mine, theirs)
		}
	}
	return rows.Err()
}

const pluginColumns = `id, name, version, manifest, sha256, enabled, installed_at, updated_at, last_error`

func scanPlugin(row interface{ Scan(...any) error }) (*Installed, error) {
	var p Installed
	var raw, installed, updated string
	var enabled int
	if err := row.Scan(&p.ID, &p.Name, &p.Version, &raw, &p.SHA256,
		&enabled, &installed, &updated, &p.LastError); err != nil {
		return nil, err
	}
	p.Enabled = enabled == 1
	p.InstalledAt, _ = time.Parse(timeLayout, installed)
	p.UpdatedAt, _ = time.Parse(timeLayout, updated)

	// A manifest that no longer parses is not a reason to hide the row: it is
	// exactly the plugin the operator needs to find in order to remove it.
	if m, err := ParseManifest([]byte(raw)); err == nil {
		p.Manifest = m
	}
	return &p, nil
}

// List returns every installed plugin, with its websites filled in.
func (s *Store) List(ctx context.Context) ([]Installed, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+pluginColumns+` FROM plugins ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	defer rows.Close()

	var out []Installed
	for rows.Next() {
		p, err := scanPlugin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sites, err := s.websitesByPlugin(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Websites = sites[out[i].ID]
	}
	return out, nil
}

// websitesByPlugin reads every assignment in one query.
//
// One query rather than one per plugin: the list is drawn on a page an operator
// visits, and a dozen plugins should not become a dozen round trips to a
// database file on an SD card.
func (s *Store) websitesByPlugin(ctx context.Context) (map[string][]int64, error) {
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT plugin_id, website_id FROM plugin_websites ORDER BY website_id`)
	if err != nil {
		return nil, fmt.Errorf("list plugin websites: %w", err)
	}
	defer rows.Close()

	out := map[string][]int64{}
	for rows.Next() {
		var id string
		var site int64
		if err := rows.Scan(&id, &site); err != nil {
			return nil, err
		}
		out[id] = append(out[id], site)
	}
	return out, rows.Err()
}

// Get returns one plugin.
func (s *Store) Get(ctx context.Context, id string) (*Installed, error) {
	row := s.DB.Read.QueryRowContext(ctx,
		`SELECT `+pluginColumns+` FROM plugins WHERE id = $1`, id)
	p, err := scanPlugin(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get plugin: %w", err)
	}
	sites, err := s.websitesByPlugin(ctx)
	if err != nil {
		return nil, err
	}
	p.Websites = sites[id]
	return p, nil
}

// SetEnabled switches a plugin on or off.
func (s *Store) SetEnabled(ctx context.Context, id string, on bool) error {
	v := 0
	if on {
		v = 1
	}
	res, err := s.DB.Write.ExecContext(ctx,
		`UPDATE plugins SET enabled = $1, updated_at = $2 WHERE id = $3`,
		v, time.Now().UTC().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetWebsites replaces the set of sites a plugin acts on.
func (s *Store) SetWebsites(ctx context.Context, id string, sites []int64) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM plugin_websites WHERE plugin_id = $1`, id); err != nil {
		return fmt.Errorf("clear plugin websites: %w", err)
	}
	for _, site := range sites {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO plugin_websites (plugin_id, website_id) VALUES ($1, $2)`, id, site); err != nil {
			return fmt.Errorf("assign plugin to website %d: %w", site, err)
		}
	}
	return tx.Commit()
}

// SetError records why a plugin could not be loaded or ran badly.
//
// Stored rather than only logged: a module that fails to compile is skipped at
// every startup, and a log line from three reboots ago is not where anyone
// looks for the reason a feature is missing.
func (s *Store) SetError(ctx context.Context, id, msg string) error {
	const max = 2000
	if len(msg) > max {
		msg = msg[:max] + "…"
	}
	_, err := s.DB.Write.ExecContext(ctx,
		`UPDATE plugins SET last_error = $1 WHERE id = $2`, msg, id)
	return err
}

// Remove deletes a plugin and everything it stored.
//
// The rows go with it, by foreign key, and so do the tables its migrations
// made, in the same transaction. Keeping a plugin's data after it was removed
// would mean a reinstall silently inherits state from a version that may have
// kept it in a different shape. Its tables would be worse than that: a later
// plugin with the same id would own them outright, and one whose prefix starts
// theirs (kontakt after kontakt-formular) could reach them, because a name
// belongs to the installed plugin with the longest prefix and the removed one
// is no longer installed. A table that belongs to an installed plugin with a
// longer prefix stays.
func (s *Store) Remove(ctx context.Context, id string) error {
	tx, err := s.DB.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("remove plugin: %w", err)
	}
	defer tx.Rollback()

	// A key between two of the plugin's tables would make the order of the
	// drops matter. Checked at the commit instead, when both are gone.
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return fmt.Errorf("remove plugin: %w", err)
	}
	tables, err := ownTables(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("remove plugin: %w", err)
	}
	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, `DROP TABLE "`+strings.ReplaceAll(t, `"`, `""`)+`"`); err != nil {
			return fmt.Errorf("remove plugin: drop %s: %w", t, err)
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM plugins WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("remove plugin: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// ownTables lists the tables in a plugin's namespace, by the rule its
// migrations are held to.
func ownTables(ctx context.Context, tx *sql.Tx, id string) ([]string, error) {
	g := newMigrationGuard(id)
	others, err := readOtherPrefixes(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	g.others = others
	rows, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if g.owns(name) {
			out = append(out, name)
		}
	}
	return out, rows.Err()
}

// --- the plugin's own key/value space ---------------------------------------

// StoreGet reads one value. A missing key is not an error: a plugin asking for
// a setting it has never written is the ordinary first run.
func (s *Store) StoreGet(ctx context.Context, id string, websiteID int64, key string) (string, bool, error) {
	var v string
	err := s.DB.Read.QueryRowContext(ctx,
		`SELECT value FROM plugin_store WHERE plugin_id = $1 AND website_id = $2 AND key = $3`,
		id, websiteID, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("plugin store get: %w", err)
	}
	return v, true, nil
}

// StoreSet writes one value.
func (s *Store) StoreSet(ctx context.Context, id string, websiteID int64, key, value string) error {
	if key == "" || len(key) > MaxKeyBytes {
		return fmt.Errorf("the key is empty or longer than %d characters", MaxKeyBytes)
	}
	if len(value) > MaxValueBytes {
		return fmt.Errorf("the value is larger than %d KB", MaxValueBytes>>10)
	}
	_, err := s.DB.Write.ExecContext(ctx, `
		INSERT INTO plugin_store (plugin_id, website_id, key, value, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(plugin_id, website_id, key) DO UPDATE SET
			value = excluded.value, updated_at = excluded.updated_at`,
		id, websiteID, key, value, time.Now().UTC().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("plugin store set: %w", err)
	}
	return nil
}

// StoreDelete removes one value.
func (s *Store) StoreDelete(ctx context.Context, id string, websiteID int64, key string) error {
	_, err := s.DB.Write.ExecContext(ctx,
		`DELETE FROM plugin_store WHERE plugin_id = $1 AND website_id = $2 AND key = $3`,
		id, websiteID, key)
	return err
}

// StoreList returns the keys under a prefix, with their values.
//
// Bounded on purpose: a plugin that has written ten thousand entries should not
// be able to pull all of them through the wasm boundary in one call.
func (s *Store) StoreList(ctx context.Context, id string, websiteID int64, prefix string, limit int) (map[string]string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.DB.Read.QueryContext(ctx, `
		SELECT key, value FROM plugin_store
		WHERE plugin_id = $1 AND website_id = $2 AND key LIKE $3 ESCAPE '\'
		ORDER BY key LIMIT $4`,
		id, websiteID, escapeLike(prefix)+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("plugin store list: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// escapeLike keeps a prefix from being read as a pattern.
//
// Without it a plugin storing a key that happens to contain a percent sign
// would match rows it never wrote — within its own space, so not a breach, but
// a bug that would take a long afternoon to find.
func escapeLike(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '%', '_', '\\':
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// --- the plugin's own migrations --------------------------------------------

// ApplyMigrations runs a plugin's SQL, once each, in name order.
//
// Each runs in a transaction of its own on a connection of its own, under a
// policy given to SQLite's authorizer and a pre-update hook: the plugin may
// create and change tables with its own prefix, read what its own tables and a
// short list of columns of the legacy tables give it, and write plugin_store
// rows of its own id, and nothing else. The policy is armed only around the
// plugin's SQL and is off the connection before it goes back to the pool.
//
// The checksum is recorded as well as the name: a plugin that ships changed SQL
// under a name that already ran would otherwise apply nothing and leave a
// schema that does not match the code expecting it — the kind of mismatch that
// only shows up as a strange error much later.
func (s *Store) ApplyMigrations(ctx context.Context, id string, ms []Migration) error {
	for _, m := range ms {
		sum := sha256.Sum256([]byte(m.SQL))
		want := hex.EncodeToString(sum[:])

		var have string
		err := s.DB.Read.QueryRowContext(ctx,
			`SELECT sha256 FROM plugin_migrations WHERE plugin_id = $1 AND name = $2`,
			id, m.Name).Scan(&have)
		switch {
		case err == nil && have == want:
			continue
		case err == nil:
			return fmt.Errorf("migration %q of plugin %q has already been applied but has changed — please ship it under a new name", m.Name, id)
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("plugin migration lookup: %w", err)
		}

		if err := s.applyGuarded(ctx, id, m, want); err != nil {
			return err
		}
	}
	return nil
}
