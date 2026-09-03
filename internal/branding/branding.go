// Package branding is the name and the mark this administration carries.
//
// Somebody who looks after five websites for a club, an association and two
// neighbours does not run "Holzcloud" — they run the thing they told those
// people about. The word in the corner is not decoration; it is the difference
// between a tool that looks borrowed and one that looks like theirs.
//
// Kept deliberately small: a name, a letter, and optionally a picture file.
// Everything else about the appearance is the design of the *website*, which
// has its own screen.
package branding

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DirName is the folder inside the data directory that holds the logo.
const DirName = "marke"

// MaxLogoBytes bounds the uploaded picture.
//
// A logo in the corner of a screen is a few kilobytes. Half a megabyte is far
// more than any of them and small enough that it can be read into memory
// without a thought.
const MaxLogoBytes = 512 << 10

// Defaults, which are what an installation that never opens the screen shows.
const (
	DefaultName = "Holzcloud"
	DefaultMark = "H"
)

// Brand is what the administration calls itself.
type Brand struct {
	// Name is the word in the corner and the second half of every page title.
	Name string
	// Mark is the letter in the square, for an installation without a picture.
	Mark string
	// LogoURL is the address of the uploaded picture, empty when there is none.
	// It carries the file's modification time so a replaced logo is not the
	// cached old one.
	LogoURL string
}

var (
	mu      sync.RWMutex
	current = Brand{Name: DefaultName, Mark: DefaultMark}
	dir     string
)

// SetDir tells the package where the logo lives. Called once at start-up with
// <data>/marke.
func SetDir(path string) {
	mu.Lock()
	dir = path
	mu.Unlock()
}

// Dir is the folder in use.
func Dir() string {
	mu.RLock()
	defer mu.RUnlock()
	return dir
}

// Current is the brand as it stands. Read on every rendered page, so it is a
// cached value and not a query.
func Current() Brand {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Load reads the stored settings. Called at start-up and after every change.
//
// A failure leaves the defaults in place: an administration that says
// "Holzcloud" because a query failed is a working administration.
func Load(ctx context.Context, db *sql.DB) {
	brand := Brand{Name: DefaultName, Mark: DefaultMark}

	rows, err := db.QueryContext(ctx,
		`SELECT key, value FROM app_settings WHERE key IN ('brand_name', 'brand_mark')`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				break
			}
			switch key {
			case "brand_name":
				if v := strings.TrimSpace(value); v != "" {
					brand.Name = v
				}
			case "brand_mark":
				if v := strings.TrimSpace(value); v != "" {
					brand.Mark = v
				}
			}
		}
	}
	brand.LogoURL = logoURL()

	mu.Lock()
	current = brand
	mu.Unlock()
}

// Save stores the name and the letter, then rereads.
func Save(ctx context.Context, db *sql.DB, name, mark string) error {
	name, mark = clean(name, DefaultName, 40), clean(mark, DefaultMark, 2)
	for _, kv := range []struct{ key, value string }{
		{"brand_name", name},
		{"brand_mark", mark},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO app_settings (key, value) VALUES ($1, $2)
			 ON CONFLICT (key) DO UPDATE SET value = excluded.value`, kv.key, kv.value); err != nil {
			return err
		}
	}
	Load(ctx, db)
	return nil
}

// clean trims a value, falls back to the default and bounds the length. The
// mark is one or two characters — it sits in a small square, and a third one
// would not fit whatever the value said.
func clean(value, fallback string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	runes := []rune(value)
	if len(runes) > max {
		value = string(runes[:max])
	}
	return value
}

// LogoPath is where the picture is kept, or empty when none is set.
func LogoPath() string {
	folder := Dir()
	if folder == "" {
		return ""
	}
	for _, ext := range []string{".svg", ".png", ".webp"} {
		path := filepath.Join(folder, "logo"+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// logoURL is the address of the picture with a cache-busting stamp, or empty.
func logoURL() string {
	path := LogoPath()
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	// The modification time in the address: a replaced logo is a different
	// address, so no browser shows yesterday's.
	return "/admin/marke/logo?v=" + info.ModTime().UTC().Format("20060102150405")
}

// RemoveLogo deletes every stored picture.
func RemoveLogo() error {
	folder := Dir()
	if folder == "" {
		return nil
	}
	for _, ext := range []string{".svg", ".png", ".webp"} {
		if err := os.Remove(filepath.Join(folder, "logo"+ext)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// WriteLogo stores a picture, replacing whatever was there.
//
// The extension decides the type and is checked by the caller against what the
// bytes actually are: a file called logo.svg that is something else would be
// served with the wrong content type, and an SVG is a document that can carry
// script.
func WriteLogo(ext string, data []byte) error {
	folder := Dir()
	if folder == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	if err := RemoveLogo(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(folder, "logo"+ext), data, 0o600)
}
