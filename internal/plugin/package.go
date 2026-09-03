package plugin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Limits on what an upload may contain.
//
// Every one of them is a defence against the same thing: a small archive that
// costs a great deal to unpack. A zip entry may claim any uncompressed size, so
// nothing is ever read without a ceiling.
const (
	// MaxModuleBytes bounds the wasm module. A Go guest is around two
	// megabytes; sixteen leaves room for a large one and stops a module that
	// would not fit in a small node's memory anyway.
	MaxModuleBytes = 16 << 20
	// MaxAssetBytes bounds one file under assets/.
	MaxAssetBytes = 4 << 20
	// MaxMigrationBytes bounds one SQL file.
	MaxMigrationBytes = 256 << 10
	// MaxEntries bounds how many files an archive may hold.
	MaxEntries = 200
	// MaxTotalBytes bounds everything together, uncompressed.
	MaxTotalBytes = 32 << 20
)

// Package is an archive that has been read and checked, held in memory.
//
// The bytes are kept rather than a path: an installation writes them once, and
// a package that was only inspected leaves nothing behind to clean up.
type Package struct {
	Manifest *Manifest
	Module   []byte
	// Assets are files served verbatim, keyed by their path below assets/.
	Assets map[string][]byte
	// Migrations are the plugin's own SQL, in the order they must run.
	Migrations []Migration
	// SHA256 is over the module, so an admin can tell two builds apart and an
	// upgrade that changes nothing can be recognised as such.
	SHA256 string
}

// Migration is one SQL file belonging to a plugin.
type Migration struct {
	Name string
	SQL  string
}

// ReadPackage reads an uploaded archive.
//
// It refuses rather than repairs. A plugin runs code on the operator's server;
// the moment something about the package is not as expected, the honest answer
// is to say so while nothing has been installed yet.
func ReadPackage(r io.ReaderAt, size int64) (*Package, error) {
	if size > MaxTotalBytes {
		return nil, fmt.Errorf("das Archiv ist größer als %d MB", MaxTotalBytes>>20)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("die Datei ist kein gültiges Archiv: %w", err)
	}
	if len(zr.File) > MaxEntries {
		return nil, fmt.Errorf("das Archiv enthält mehr als %d Dateien", MaxEntries)
	}

	manifestRaw, err := readEntry(zr, ManifestName, MaxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("im Archiv fehlt %s", ManifestName)
	}
	manifest, err := ParseManifest(manifestRaw)
	if err != nil {
		return nil, err
	}

	module, err := readEntry(zr, ModuleName, MaxModuleBytes)
	if err != nil {
		return nil, fmt.Errorf("im Archiv fehlt %s", ModuleName)
	}
	if !bytes.HasPrefix(module, []byte("\x00asm")) {
		return nil, fmt.Errorf("%s ist kein WebAssembly-Modul", ModuleName)
	}

	pkg := &Package{
		Manifest: manifest,
		Module:   module,
		Assets:   map[string][]byte{},
	}
	sum := sha256.Sum256(module)
	pkg.SHA256 = hex.EncodeToString(sum[:])

	total := len(manifestRaw) + len(module)
	for _, f := range zr.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			continue
		}
		if name == ManifestName || name == ModuleName {
			continue
		}

		switch {
		case strings.HasPrefix(name, AssetDir):
			rel, err := safeRelative(name, AssetDir)
			if err != nil {
				return nil, err
			}
			data, err := readEntry(zr, name, MaxAssetBytes)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			total += len(data)
			pkg.Assets[rel] = data

		case strings.HasPrefix(name, MigrationDir):
			rel, err := safeRelative(name, MigrationDir)
			if err != nil {
				return nil, err
			}
			if !strings.HasSuffix(rel, ".sql") || strings.Contains(rel, "/") {
				return nil, fmt.Errorf("%s: unter %s sind nur .sql-Dateien ohne Unterordner erlaubt",
					name, MigrationDir)
			}
			data, err := readEntry(zr, name, MaxMigrationBytes)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			total += len(data)
			pkg.Migrations = append(pkg.Migrations, Migration{Name: rel, SQL: string(data)})

		default:
			// Not ignored: a file the host does not understand is either a
			// mistake in the build or something that was meant to end up
			// somewhere it should not.
			return nil, fmt.Errorf("das Archiv enthält %q, das dort nicht hingehört", name)
		}

		if total > MaxTotalBytes {
			return nil, fmt.Errorf("das Archiv entpackt sich auf mehr als %d MB", MaxTotalBytes>>20)
		}
	}

	// Migrations run in name order, so the order is the author's to decide and
	// does not depend on how the zip happened to be written.
	sort.Slice(pkg.Migrations, func(i, j int) bool {
		return pkg.Migrations[i].Name < pkg.Migrations[j].Name
	})
	return pkg, nil
}

// safeRelative strips the prefix and refuses anything that is not a plain
// relative path below it.
//
// Zip slip: an entry called "assets/../../etc/holzcloud.conf" is how an archive
// writes outside the directory it was unpacked into. The template upload and
// the bundle import each guard against this; a plugin package is the same
// hazard through a third door.
func safeRelative(name, prefix string) (string, error) {
	rel := strings.TrimPrefix(name, prefix)
	if rel == "" {
		return "", fmt.Errorf("%q hat keinen Dateinamen", name)
	}
	clean := path.Clean(rel)
	if clean != rel || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("der Eintrag %q ist kein zulässiger Pfad", name)
	}
	for _, seg := range strings.Split(clean, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("der Eintrag %q ist kein zulässiger Pfad", name)
		}
	}
	return clean, nil
}

// readEntry reads one archive member under a hard ceiling.
func readEntry(zr *zip.Reader, name string, max int64) ([]byte, error) {
	f, err := zr.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// One byte past the limit, so a file that is exactly at the ceiling still
	// reads and one that is over it is recognised rather than silently cut.
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%s ist größer als %d Bytes", name, max)
	}
	return data, nil
}
