package branding

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// The tests of the one package under internal/ that had none. 0.0 % before,
// 95.7 % after; what is left is a driver fault in rows.Scan, a file that
// disappears between LogoPath and Stat, and a write that fails after MkdirAll
// succeeded — three races and a hardware failure, and none of them worth a
// fake.
//
// TEST-03 says every test here is driven red before it is driven green, and
// these were — by mutating the package fifteen ways and watching which test
// caught which. Thirteen of the fifteen were caught. The two that were not are
// recorded here rather than papered over, because both are **equivalent**: the
// mutated code does the same thing, so no test could catch them.
//
//  1. `len(runes) > max` → `>= max`. At the bound the branch slices
//     `runes[:max]`, which is the whole value again. Above and below it the two
//     spellings already agree.
//  2. `WriteLogo`'s empty-folder guard, removed. Without it the next line is
//     `os.MkdirAll("")`, which fails with ENOENT — so the function still
//     returns a not-exist error and still writes nothing. Measured, not
//     assumed: `&fs.PathError{Op:"mkdir", Path:"", Err:0x2}`, `IsNotExist`
//     true. The guard is worth keeping (it says what it means, where a reader
//     looks for it), but it is belt to MkdirAll's braces.
//
// The sweep also found three tests that passed for the wrong reason, and they
// are why `TestCleanIsWhatMayBeStored` exists and why
// `TestWhatSaveAcceptsAndWhatItCorrects` reads the stored row: `Load` applies
// the same trim-and-fall-back a second time, so a `Save` that stored `"   "`
// still produced the right brand on screen. Both guards are right; tested only
// through each other, only one of them was really held.

// This package keeps its state in package variables, because the brand is read
// on every rendered page and a query per page would be absurd. So the tests may
// not run in parallel, and each one puts the package back the way it found it.
func reset(t *testing.T) string {
	t.Helper()
	folder := t.TempDir()
	SetDir(folder)
	t.Cleanup(func() {
		SetDir("")
		mu.Lock()
		current = Brand{Name: DefaultName, Mark: DefaultMark}
		mu.Unlock()
	})
	return folder
}

func newDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return database
}

// An installation that never opens the screen shows "Holzcloud" and an H. That
// is the state most installations are in and the one no test would otherwise
// exercise, because it is the state before anything happens.
func TestAnInstallationThatSavedNothingCarriesTheDefaults(t *testing.T) {
	reset(t)
	database := newDB(t)

	Load(context.Background(), database.Read)

	got := Current()
	if got.Name != DefaultName || got.Mark != DefaultMark {
		t.Errorf("Current() = %+v, want %q/%q", got, DefaultName, DefaultMark)
	}
	if got.LogoURL != "" {
		t.Errorf("a brand with no picture carries an address: %q", got.LogoURL)
	}
}

// The sentence in Load's doc comment, held: "an administration that says
// Holzcloud because a query failed is a working administration". A database
// without the table is the cheapest way to make the query fail for real.
func TestAFailedQueryLeavesTheDefaultsStanding(t *testing.T) {
	reset(t)
	// Set something first, so a bug that empties the brand is distinguishable
	// from one that never filled it.
	mu.Lock()
	current = Brand{Name: "Vorher", Mark: "V"}
	mu.Unlock()

	bare, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "leer.sqlite"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer bare.Close()

	Load(context.Background(), bare)

	got := Current()
	if got.Name != DefaultName || got.Mark != DefaultMark {
		t.Errorf("a failed query produced %+v instead of the defaults", got)
	}
}

func TestSaveKeepsTheNameAndTheMark(t *testing.T) {
	reset(t)
	database := newDB(t)
	ctx := context.Background()

	if err := Save(ctx, database.Write, "Holzbau Schmidt", "HS"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := Current(); got.Name != "Holzbau Schmidt" || got.Mark != "HS" {
		t.Errorf("Current() = %+v", got)
	}

	// And it is stored rather than merely remembered: a fresh Load finds it.
	mu.Lock()
	current = Brand{}
	mu.Unlock()
	Load(ctx, database.Read)
	if got := Current(); got.Name != "Holzbau Schmidt" || got.Mark != "HS" {
		t.Errorf("after a second Load: %+v", got)
	}
}

// clean, directly and on its own.
//
// It is tested here rather than only through Save, and that is a finding rather
// than a preference: Load applies the same trim-and-fall-back a second time, so
// a Save that stored "   " still produces the right brand on screen and every
// test that goes through both passes. The dirty value would sit in the database
// unseen until something else read it. Two guards are right — the stored row
// comes from an older version or from a hand edit — but they have to be tested
// apart or only one of them is really held.
func TestCleanIsWhatMayBeStored(t *testing.T) {
	for _, c := range []struct {
		what, value, fallback string
		max                   int
		want                  string
	}{
		{"nothing at all falls back", "", "Holzcloud", 40, "Holzcloud"},
		{"whitespace is nothing", " \t\n ", "Holzcloud", 40, "Holzcloud"},
		{"the surrounding whitespace goes", "  Holzbau  ", "Holzcloud", 40, "Holzbau"},
		{"the inner whitespace stays", "Holzbau Schmidt", "Holzcloud", 40, "Holzbau Schmidt"},
		{"a value at the bound is untouched", strings.Repeat("a", 40), "H", 40, strings.Repeat("a", 40)},
		{"a longer one is cut to it", strings.Repeat("a", 45), "H", 40, strings.Repeat("a", 40)},
		// The case a byte count gets wrong: three umlauts are six bytes and
		// three characters, and it is the characters that have to fit. A byte
		// slice would also cut one of them in half and produce an invalid
		// string.
		{"the bound counts characters and not bytes", "ÄÖÜ", "H", 2, "ÄÖ"},
		{"and trims before it counts", "  ÄÖÜ  ", "H", 2, "ÄÖ"},
	} {
		if got := clean(c.value, c.fallback, c.max); got != c.want {
			t.Errorf("%s: clean(%q, %q, %d) = %q, want %q",
				c.what, c.value, c.fallback, c.max, got, c.want)
		}
	}
}

// What the form can send and what may be stored are two different things, and
// clean is the whole of the difference. This is the same table through Save,
// which also asserts the row that lands in the database — without that, Load's
// own trim hides a Save that stored whatever it was given.
func TestWhatSaveAcceptsAndWhatItCorrects(t *testing.T) {
	reset(t)
	database := newDB(t)
	ctx := context.Background()

	for _, c := range []struct {
		what             string
		name, mark       string
		wantName, wantMk string
	}{
		{"nothing at all falls back",
			"", "", DefaultName, DefaultMark},
		{"whitespace is nothing",
			"   ", "\t\n ", DefaultName, DefaultMark},
		{"the surrounding whitespace goes",
			"  Holzbau  ", " H ", "Holzbau", "H"},
		{"a name longer than forty characters is cut",
			strings.Repeat("a", 45), "X", strings.Repeat("a", 40), "X"},
		{"a mark is one or two characters, because the square holds no more",
			"Name", "ABC", "Name", "AB"},
		// The case a byte count gets wrong: three umlauts are six bytes and
		// three characters, and it is the characters that have to fit.
		{"the bound counts characters and not bytes",
			"Ärger", "ÄÖÜ", "Ärger", "ÄÖ"},
	} {
		if err := Save(ctx, database.Write, c.name, c.mark); err != nil {
			t.Fatalf("%s: Save: %v", c.what, err)
		}
		got := Current()
		if got.Name != c.wantName || got.Mark != c.wantMk {
			t.Errorf("%s: got %q/%q, want %q/%q",
				c.what, got.Name, got.Mark, c.wantName, c.wantMk)
		}
		// And the same in the database, not merely in the cached value.
		for _, kv := range [][2]string{
			{"brand_name", c.wantName}, {"brand_mark", c.wantMk},
		} {
			var stored string
			if err := database.Read.QueryRowContext(ctx,
				`SELECT value FROM app_settings WHERE key = $1`, kv[0]).Scan(&stored); err != nil {
				t.Fatalf("%s: read %s: %v", c.what, kv[0], err)
			}
			if stored != kv[1] {
				t.Errorf("%s: %s is stored as %q, want %q", c.what, kv[0], stored, kv[1])
			}
		}
	}
}

// A value that is only whitespace in the database is the same as no value: the
// screen must not show a blank corner because somebody once wrote a space.
func TestAStoredBlankIsNotAName(t *testing.T) {
	reset(t)
	database := newDB(t)
	ctx := context.Background()

	// Written past Save, because Save would have cleaned it — this is the row
	// an older version or a hand-edited database leaves behind.
	for _, kv := range [][2]string{{"brand_name", "   "}, {"brand_mark", ""}} {
		if _, err := database.Write.ExecContext(ctx,
			`INSERT INTO app_settings (key, value) VALUES ($1, $2)`, kv[0], kv[1]); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	Load(ctx, database.Read)

	if got := Current(); got.Name != DefaultName || got.Mark != DefaultMark {
		t.Errorf("a blank row became the brand: %+v", got)
	}
}

func TestTheLogoIsFoundWrittenAndRemoved(t *testing.T) {
	folder := reset(t)

	if got := LogoPath(); got != "" {
		t.Errorf("a folder without a picture answered %q", got)
	}

	if err := WriteLogo(".png", []byte("nicht wirklich ein png")); err != nil {
		t.Fatalf("WriteLogo: %v", err)
	}
	if got, want := LogoPath(), filepath.Join(folder, "logo.png"); got != want {
		t.Errorf("LogoPath = %q, want %q", got, want)
	}

	if err := RemoveLogo(); err != nil {
		t.Fatalf("RemoveLogo: %v", err)
	}
	if got := LogoPath(); got != "" {
		t.Errorf("after RemoveLogo: %q", got)
	}
	// Twice is not an error: the screen's remove button may be pressed on an
	// installation that has none.
	if err := RemoveLogo(); err != nil {
		t.Errorf("a second RemoveLogo: %v", err)
	}
}

// The case the extension makes possible: LogoPath prefers .svg, so a new .png
// beside an old .svg would be invisible and the old picture would go on being
// served. WriteLogo removes first for exactly this reason.
func TestANewLogoReplacesOneWithAnotherExtension(t *testing.T) {
	folder := reset(t)

	if err := WriteLogo(".svg", []byte("<svg/>")); err != nil {
		t.Fatalf("WriteLogo svg: %v", err)
	}
	if err := WriteLogo(".png", []byte("PNG")); err != nil {
		t.Fatalf("WriteLogo png: %v", err)
	}

	if _, err := os.Stat(filepath.Join(folder, "logo.svg")); !os.IsNotExist(err) {
		t.Errorf("the old .svg is still there: %v", err)
	}
	if got, want := LogoPath(), filepath.Join(folder, "logo.png"); got != want {
		t.Errorf("LogoPath = %q, want %q", got, want)
	}
}

// Without a folder the package does nothing rather than guessing at one: this
// is the state between process start and SetDir, and a command-line tool that
// never calls it at all.
func TestWithoutAFolderNothingHappens(t *testing.T) {
	SetDir("")
	t.Cleanup(func() { SetDir("") })

	if got := Dir(); got != "" {
		t.Errorf("Dir = %q", got)
	}
	if got := LogoPath(); got != "" {
		t.Errorf("LogoPath = %q", got)
	}
	if err := RemoveLogo(); err != nil {
		t.Errorf("RemoveLogo: %v", err)
	}
	// Run from an empty directory, because the thing the guard prevents is not
	// an error — it is filepath.Join("", "logo.png") writing `logo.png` into
	// whatever the working directory happens to be. An error alone would be
	// produced by os.MkdirAll("") too, so asserting only on the error proves
	// nothing.
	here := t.TempDir()
	back, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(here); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(back)

	if err := WriteLogo(".png", []byte("x")); !os.IsNotExist(err) {
		t.Errorf("WriteLogo = %v, want a not-exist error", err)
	}
	entries, err := os.ReadDir(here)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("WriteLogo wrote into the working directory: %v", entries)
	}
}

// The address carries the file's time so that a replaced logo is a different
// address. Without it a browser shows yesterday's picture and the operator
// concludes the upload did not work.
func TestTheAddressChangesWhenThePictureDoes(t *testing.T) {
	folder := reset(t)
	database := newDB(t)
	ctx := context.Background()

	if err := WriteLogo(".png", []byte("erstes")); err != nil {
		t.Fatalf("WriteLogo: %v", err)
	}
	Load(ctx, database.Read)
	first := Current().LogoURL
	if !strings.HasPrefix(first, "/admin/marke/logo?v=") {
		t.Fatalf("LogoURL = %q", first)
	}

	// A second apart, because the stamp is to the second. Set rather than
	// waited for: a test that sleeps for a second is a test people delete.
	stamp := time.Now().Add(-90 * time.Second)
	if err := os.Chtimes(filepath.Join(folder, "logo.png"), stamp, stamp); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	Load(ctx, database.Read)
	second := Current().LogoURL

	if second == first {
		t.Errorf("the address did not change with the picture: %q", second)
	}
	if want := "/admin/marke/logo?v=" + stamp.UTC().Format("20060102150405"); second != want {
		t.Errorf("LogoURL = %q, want %q", second, want)
	}
}

// The paths where something goes wrong say so rather than pretending.
//
// These are what a handler leans on: handleBrandingPost returns the error from
// Save and from RemoveLogo, which becomes a 500 with an entry in the log. A
// version that swallowed them would show "Brand saved" over a brand that was
// not saved, and that is the failure this project spends its comments avoiding.
func TestTheFailuresAreReported(t *testing.T) {
	folder := reset(t)
	ctx := context.Background()

	// Save against a closed database: the error comes back AND the cached
	// brand is left alone, because Save returns before it rereads.
	database := newDB(t)
	if err := Save(ctx, database.Write, "Vorher", "V"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	database.Close()
	if err := Save(ctx, database.Write, "Nachher", "N"); err == nil {
		t.Error("Save against a closed database reported success")
	}
	if got := Current().Name; got != "Vorher" {
		t.Errorf("a failed Save changed the brand to %q", got)
	}

	// RemoveLogo when the file will not go. A directory in the logo's place is
	// the cheapest way to produce an error that is not "it was not there" —
	// and "it was not there" is the one case that must stay silent, which
	// TestTheLogoIsFoundWrittenAndRemoved holds.
	inTheWay := filepath.Join(folder, "logo.png")
	if err := os.MkdirAll(filepath.Join(inTheWay, "drin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RemoveLogo(); err == nil {
		t.Error("RemoveLogo reported success on something it could not remove")
	}
	if err := os.RemoveAll(inTheWay); err != nil {
		t.Fatal(err)
	}

	// WriteLogo when the folder cannot be made: a plain file where a directory
	// would have to be.
	blocker := filepath.Join(folder, "datei")
	if err := os.WriteFile(blocker, []byte("keine mappe"), 0o600); err != nil {
		t.Fatal(err)
	}
	SetDir(filepath.Join(blocker, "marke"))
	if err := WriteLogo(".png", []byte("x")); err == nil {
		t.Error("WriteLogo reported success although the folder could not be made")
	}
}
