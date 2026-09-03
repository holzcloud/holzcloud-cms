package i18n

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

// Languages from a folder on disk, beside the ones compiled into the binary.
//
// The same rule as for themes: **disk wins, embedded is the fallback.** Drop
// nl.json into the folder and the administration speaks Dutch; delete it and it
// stops. Put a de.json there and you have corrected a wording of ours without
// building anything — the file only has to carry the strings you want changed,
// everything else keeps coming from the binary.
//
// Why not only on disk? Because a fresh installation is one file that runs. A
// binary whose administration is unreadable until somebody copies five files
// next to it is a binary that fails on the first evening.

// DirName is the folder inside the data directory. German, like data/vorlagen
// would be if that had not been named earlier — it is a folder an operator
// looks at with a file manager.
const DirName = "sprachen"

// MaxFileBytes bounds one language file.
//
// The German source is roughly 120 kB; a megabyte is far more than any real
// translation and small enough that a mistyped path cannot read a database
// dump into memory.
const MaxFileBytes = 1 << 20

var (
	dirMu sync.RWMutex
	dir   string
)

// SetDir tells the package where operator-supplied languages live, and reads
// them. Called once at start-up with <data>/sprachen.
func SetDir(path string) {
	dirMu.Lock()
	dir = path
	dirMu.Unlock()
	Reload()
}

// Dir is the folder in use, empty when none was set.
func Dir() string {
	dirMu.RLock()
	defer dirMu.RUnlock()
	return dir
}

// Reload reads everything again: the embedded catalogues first, then the folder
// on top.
//
// Cheap enough to do on demand — a handful of files of a hundred kilobytes —
// which is what makes "copy a file in and press the button" possible without a
// restart.
func Reload() {
	mu.Lock()
	defer mu.Unlock()
	loadLocked()
}

// FromDisk reports whether this language came from the folder rather than from
// the binary, so the administration can say which files it may delete.
func FromDisk(code string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return onDisk[code]
}

// readDir folds the files of the language folder into the catalogues.
//
// A broken file is skipped with a line in the log and never stops the server:
// the language it would have added simply is not there, and everything else
// keeps working. That is the difference between a folder anybody may drop a
// file into and a folder that can take the site down.
func readDir(into map[string]map[string]string, disk map[string]bool) {
	path := Dir()
	if path == "" {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("read language folder", "err", err, "path", path)
		}
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		// The file name is the tag, and it is normalised: somebody copying a
		// file in with a file manager types de-ch.json, and a language that
		// only works when the file name is capitalised correctly is a language
		// that does not work.
		code := Normalise(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		if !ValidTag(code) {
			slog.Warn("language file ignored: the file name is not a language tag",
				"file", e.Name(), "expected", "de.json, de-CH.json")
			continue
		}
		msgs, err := ReadFile(filepath.Join(path, e.Name()))
		if err != nil {
			slog.Error("language file ignored", "err", err, "file", e.Name())
			continue
		}
		// Merged into what is already there rather than replacing it: a file
		// carrying ten corrections should not blank out the other 870 strings.
		if existing, ok := into[code]; ok {
			for k, v := range msgs {
				existing[k] = v
			}
		} else {
			into[code] = msgs
		}
		disk[code] = true
		slog.Info("language file loaded", "code", code, "strings", len(msgs))
	}
}

// ReadFile reads and checks one language file.
//
// Everything that could produce a broken screen is caught here rather than at
// render time: bad JSON, an entry that is not a string, an empty translation,
// and — the one that matters — HTML somebody put in a value.
func ReadFile(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxFileBytes {
		return nil, fmt.Errorf("die Datei ist größer als %d Bytes", MaxFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse turns the bytes of a language file into a catalogue, or says why not.
func Parse(data []byte) (map[string]string, error) {
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("keine gültige JSON-Datei aus Text-Paaren: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("die Datei enthält keine Übersetzungen")
	}

	out := make(map[string]string, len(raw))
	for german, translated := range raw {
		translated = strings.TrimSpace(translated)
		if translated == "" {
			// Not an error: an unfinished file is the normal state of a
			// translation in progress, and the German shows through.
			continue
		}
		if verbs(german) != verbs(translated) {
			// A missing %s swallows a name, an extra one prints
			// "%!s(MISSING)" on somebody's screen. Neither is worth shipping,
			// and the German underneath is correct.
			slog.Warn("translation ignored: the placeholders do not match the German",
				"de", german, "translation", translated)
			continue
		}
		out[german] = sanitise(translated)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("die Datei enthält keine brauchbare Übersetzung")
	}
	return out, nil
}

// safeText is what a translation may contain in the way of markup.
//
// A few sentences carry a <code> or a <strong> and are rendered raw — see the
// th helper. Those sentences come from our own templates, but the *translation*
// of them now comes off the disk, so it is filtered: exactly the inline tags
// the source uses, no attributes, nothing else. Anything more is stripped,
// which turns a hostile file into a harmless one instead of a refused one.
var safeText = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("code", "strong", "em", "b", "i", "kbd", "abbr", "small", "sub", "sup", "br")
	return p
}()

func sanitise(s string) string {
	// The common case carries no markup at all, and running every one of nine
	// hundred strings through a sanitiser at start-up to find that out is work
	// for nothing.
	if !strings.ContainsAny(s, "<>") {
		return s
	}
	return safeText.Sanitize(s)
}

// verbs is the format verbs of a string in order. "%%" is an escaped percent
// sign and not a verb.
func verbs(s string) string {
	var found []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			continue
		}
		if s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && strings.ContainsRune("+-# 0123456789.", rune(s[j])) {
			j++
		}
		if j < len(s) {
			found = append(found, s[i:j+1])
			i = j
		}
	}
	return strings.Join(found, " ")
}

// SourceStrings are every German string the administration can show, sorted.
//
// It is what the starter file for a new language is built from, so a translator
// gets a complete list instead of having to hunt through the templates.
func SourceStrings() []string {
	mu.RLock()
	defer mu.RUnlock()

	// The English catalogue ships complete, so its keys are the full set. If it
	// were ever removed, the union of everything is still the best answer
	// available at runtime.
	seen := map[string]bool{}
	for _, catalog := range catalogs {
		for german := range catalog {
			seen[german] = true
		}
	}
	out := make([]string, 0, len(seen))
	for german := range seen {
		out = append(out, german)
	}
	sort.Strings(out)
	return out
}

// Starter builds the file a translator begins with: every German string as a
// key, every value empty, sorted and indented so a diff stays readable.
func Starter() []byte {
	var b strings.Builder
	b.WriteString("{\n")
	keys := SourceStrings()
	for i, k := range keys {
		key, _ := json.Marshal(k)
		b.Write(key)
		b.WriteString(": \"\"")
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// Stat is what the administration shows about one language.
type Stat struct {
	Code string
	Name string
	// Translated is how many of the source strings this language has.
	Translated int
	// Total is how many there are.
	Total int
	// OnDisk marks a language that came from the folder — the only kind that
	// can be deleted from the administration.
	OnDisk bool
	// Source marks German, which needs no catalogue at all.
	Source bool
	// BaseName is the language a regional fassung leans on, "Deutsch" for
	// de-CH. Empty for a language that stands on its own.
	BaseName string
	// BaseMissing marks a fassung whose base language is not installed —
	// nl-BE without nl. It still works, but everything the file does not say
	// itself comes out in German, and that is worth saying on the screen
	// rather than leaving somebody to wonder.
	BaseMissing bool
	// Own is how many sentences this file says itself. For a fassung that is
	// the interesting number: everything else is inherited, and inherited is
	// not missing.
	Own int
}

// Percent is how far along this language is, for a progress bar.
func (s Stat) Percent() int {
	if s.Source || s.Total == 0 {
		return 100
	}
	return s.Translated * 100 / s.Total
}

// Complete reports whether nothing is missing.
func (s Stat) Complete() bool { return s.Percent() >= 100 }

// Stats describes every language this installation can show.
//
// A regional fassung is counted the way it is read: a sentence it does not
// translate itself is not missing, it comes from the base language, and de-CH
// is complete the moment German is. What the file actually says is Own.
func Stats() []Stat {
	sources := SourceStrings()
	total := len(sources)

	mu.RLock()
	defer mu.RUnlock()

	out := []Stat{{Code: Source, Name: nameOf(Source), Translated: total, Total: total, Source: true}}
	codes := make([]string, 0, len(catalogs))
	for code := range catalogs {
		if code != Source {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	for _, code := range codes {
		stat := Stat{
			Code: code, Name: nameOf(code),
			Translated: len(catalogs[code]), Total: total,
			OnDisk: onDisk[code],
			Own:    len(catalogs[code]),
		}
		if base := Base(code); base != "" {
			stat.BaseName = nameOf(base)
			stat.Translated = coveredLocked(code, base, sources)
			_, installed := catalogs[base]
			stat.BaseMissing = base != Source && !installed
		}
		out = append(out, stat)
	}
	return out
}

// coveredLocked counts the sentences a regional fassung actually shows in its
// own language: its own, plus everything its base language answers. When the
// base is German there is nothing to count — German answers everything.
func coveredLocked(code, base string, sources []string) int {
	own := catalogs[code]
	if base == Source {
		return len(sources)
	}
	parent := catalogs[base]
	covered := 0
	for _, german := range sources {
		if own[german] != "" || parent[german] != "" {
			covered++
		}
	}
	return covered
}

// Export writes one installed catalogue back out as a language file.
//
// It is what "download" hands over: a correction should start from what is
// actually running, not from an empty file somebody has to fill in twice. Every
// German string is a key, so a partly translated language comes back with its
// gaps visible.
func Export(code string) []byte {
	keys := SourceStrings()

	mu.RLock()
	catalog := map[string]string{}
	for k, v := range catalogs[code] {
		catalog[k] = v
	}
	mu.RUnlock()

	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		key, _ := json.Marshal(k)
		value, _ := json.Marshal(catalog[k])
		b.Write(key)
		b.WriteString(": ")
		b.Write(value)
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return []byte(b.String())
}
