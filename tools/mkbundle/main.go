// Command mkbundle packs a website that lives in the repository as source into
// the zip archive the admin import expects.
//
// A bundle written by Export is a zip with a manifest and the media bytes
// beside it. That is the right shape to hand somebody, and the wrong shape to
// keep in git: nobody can review a zip, and a one-word fix in a page rewrites
// thirty megabytes of binary. So the sites here are kept unpacked — the
// manifest as readable JSON, the pictures as ordinary files — and this turns
// them into the archive at the moment somebody needs one.
//
//	go run ./tools/mkbundle sites/beispiel
//
// It also checks the things a hand-written manifest gets wrong. An import that
// silently drops a mistyped field would leave the operator with a site that
// looks imported and is missing content, and they would find out weeks later.
// Better to refuse here, where there is nothing to undo yet.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/bundle"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mkbundle <site-dir> [<site-dir>...]")
		fmt.Fprintln(os.Stderr, "  writes <site-dir>.zip beside each directory")
		os.Exit(2)
	}
	for _, dir := range os.Args[1:] {
		out := strings.TrimSuffix(filepath.Clean(dir), string(filepath.Separator)) + ".zip"
		if err := pack(dir, out); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
			os.Exit(1)
		}
		info, _ := os.Stat(out)
		fmt.Printf("%s -> %s (%.1f MB)\n", dir, out, float64(info.Size())/(1<<20))
	}
}

func pack(dir, out string) error {
	manifest, err := readManifest(filepath.Join(dir, bundle.ManifestName))
	if err != nil {
		return err
	}

	mediaDir := filepath.Join(dir, "media")
	if err := checkMedia(manifest, mediaDir); err != nil {
		return err
	}
	if err := checkReferences(manifest); err != nil {
		return err
	}

	// The hashes are computed here rather than stored in the repository: a
	// checked-in hash is a second copy of the truth that goes stale the moment
	// somebody re-crops a photo, and a stale one fails the import with a
	// corruption message that is simply wrong.
	for i := range manifest.Media {
		sum, err := hashFile(filepath.Join(mediaDir, manifest.Media[i].Filename))
		if err != nil {
			return err
		}
		manifest.Media[i].SHA256 = sum
	}

	manifest.Version = bundle.Version
	manifest.ExportedAt = time.Now().UTC().Format(time.RFC3339)
	manifest.GeneratedBy = "mkbundle"

	return writeZip(out, manifest, mediaDir)
}

// readManifest refuses a field it does not know.
//
// The import is deliberately forgiving — it takes what it recognises and moves
// on, because a bundle from an older version should still land. That is the
// right call there and the wrong one here: at this end the manifest is written
// by hand, and "meta_desc" instead of "meta_description" would cost every
// description on the site without a single error message.
func readManifest(path string) (*bundle.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	var m bundle.Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", bundle.ManifestName, err)
	}
	if m.Site.Name == "" {
		return nil, fmt.Errorf("%s: site.name is empty", bundle.ManifestName)
	}
	if len(m.Pages) == 0 {
		return nil, fmt.Errorf("%s: no pages", bundle.ManifestName)
	}
	return &m, nil
}

// checkMedia insists the manifest and the directory agree in both directions.
//
// A listed file that does not exist breaks the import outright, which is the
// harmless half. The dangerous half is a file on disk that nothing lists: it
// travels nowhere, and the page that shows it renders a broken image on a
// machine where the author cannot see it.
func checkMedia(m *bundle.Manifest, mediaDir string) error {
	onDisk := map[string]bool{}
	entries, err := os.ReadDir(mediaDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			onDisk[e.Name()] = true
		}
	}

	listed := map[string]bool{}
	var problems []string
	for _, med := range m.Media {
		if med.Filename == "" {
			problems = append(problems, "a media entry has no filename")
			continue
		}
		if listed[med.Filename] {
			problems = append(problems, fmt.Sprintf("%s is listed twice", med.Filename))
		}
		listed[med.Filename] = true
		if !onDisk[med.Filename] {
			problems = append(problems, fmt.Sprintf("%s is listed but not in media/", med.Filename))
		}
		if med.AltText == "" {
			problems = append(problems, fmt.Sprintf("%s has no alt text", med.Filename))
		}
	}
	for name := range onDisk {
		if !listed[name] {
			problems = append(problems, fmt.Sprintf("media/%s is not listed in the manifest", name))
		}
	}
	return join(problems)
}

// checkReferences follows every pointer inside the manifest.
//
// These are all references by name rather than by id, which is what makes the
// file readable and what makes a typo survive unnoticed until a visitor hits
// the menu entry that goes nowhere.
//
// A page's terms are the one place where the spelling had to be settled: they
// carry the label's name as it is shown, not its slug, because that is what the
// export writes and what the import reads. So the lookup is keyed by the name,
// and the slugs are kept only to tell somebody who wrote the wrong one which
// spelling belongs there.
func checkReferences(m *bundle.Manifest) error {
	slugs := map[string]bool{}
	files := map[string]bool{}
	terms := map[string]bool{}
	termBySlug := map[string]string{}
	for _, p := range m.Pages {
		slugs[p.Slug] = true
	}
	for _, med := range m.Media {
		files[med.Filename] = true
	}
	for _, t := range m.Terms {
		terms[t.Name] = true
		termBySlug[t.Slug] = t.Name
	}

	var problems []string
	for _, p := range m.Pages {
		if p.FeaturedImage != "" && !files[p.FeaturedImage] {
			problems = append(problems, fmt.Sprintf("page %q: featured_image %s is not in the media list", p.Slug, p.FeaturedImage))
		}
		for _, t := range p.Terms {
			if terms[t] {
				continue
			}
			// A manifest written against the check as it stood before
			// 2026-09-03 spells its page terms as slugs. Telling its author
			// the entry is missing would send them looking for a declaration
			// they can see sitting right there, so say what to write instead.
			if name, ok := termBySlug[t]; ok {
				problems = append(problems, fmt.Sprintf("page %q: term %q is a slug — a page's terms carry the name, so write %q here", p.Slug, t, name))
				continue
			}
			problems = append(problems, fmt.Sprintf("page %q: term %q has no entry in terms", p.Slug, t))
		}
		// Every /media/<file> the page points at has to travel with it. The
		// path is checked rather than merely the name so a page cannot smuggle
		// in a reference to a file that only exists on the author's machine.
		for _, ref := range mediaRefs(p.Markdown) {
			if !files[ref] {
				problems = append(problems, fmt.Sprintf("page %q: links to the picture %s, which is not in the media list", p.Slug, ref))
			}
		}
		for _, bad := range unsafeAltTexts(p.Markdown) {
			problems = append(problems, fmt.Sprintf("page %q: the alt text %q would be dropped entirely — %s",
				p.Slug, bad.text, bad.why))
		}
	}
	for _, s := range m.Snippets {
		for _, ref := range mediaRefs(s.Markdown) {
			if !files[ref] {
				problems = append(problems, fmt.Sprintf("snippet %q: links to the picture %s, which is not in the media list", s.Key, ref))
			}
		}
	}
	for _, menu := range m.Menus {
		problems = append(problems, checkItems(menu.Name, menu.Items, slugs)...)
	}
	return join(problems)
}

func checkItems(menu string, items []bundle.MenuItem, slugs map[string]bool) []string {
	var problems []string
	for _, it := range items {
		if it.PageSlug != "" && !slugs[it.PageSlug] {
			problems = append(problems, fmt.Sprintf("menu %q: item %q points at page %q, which does not exist", menu, it.Title, it.PageSlug))
		}
		problems = append(problems, checkItems(menu, it.Children, slugs)...)
	}
	return problems
}

// mediaPathPattern is the same shape the import rewrites: /media/<id>/<file>.
//
// Deliberately the same expression as internal/bundle. If the two ever drifted,
// this would pass a path the import does not recognise, and the check would be
// worse than none — it would promise the pictures work.
var mediaPathPattern = regexp.MustCompile(`/media/[0-9]+/([^)"'\s<>]+)`)

// mediaRefs pulls the file names out of every /media/… path in the text.
//
// Scanning the Markdown rather than parsing it is on purpose: the same path
// turns up in an image, in a link, and in a raw <img>, and all three have to
// be checked. A plain scan catches all of them and cannot be fooled by
// whichever syntax the author happened to reach for.
func mediaRefs(markdown string) []string {
	var found []string
	for _, m := range mediaPathPattern.FindAllStringSubmatch(markdown, -1) {
		found = append(found, m[1])
	}
	return found
}

// altTextPattern finds the description of a Markdown image, and altAttrPattern
// the same thing written as raw HTML. Both end up in the same attribute.
var (
	altTextPattern = regexp.MustCompile(`!\[([^\]]*)\]\(`)
	altAttrPattern = regexp.MustCompile(`<img\b[^>]*\balt="([^"]*)"`)
)

// altAllowed is the shape bluemonday accepts for an alt attribute
// (UGCPolicy, helpers.go: AllowImages). Anything outside it is not escaped or
// trimmed — the whole attribute is removed.
var altAllowed = regexp.MustCompile(`^[\p{L}\p{N}\s\-_',\[\]!\./\\()]*$`)

type badAlt struct {
	text string
	why  string
}

// unsafeAltTexts finds the descriptions that will not survive being saved.
//
// This is the quietest way a page can lose something that matters. A colon, a
// question mark, an em dash, an ampersand, a percent sign — any one of them and
// bluemonday drops the ENTIRE alt attribute rather than the offending
// character. The picture still renders, the page looks finished, and it is
// simply unreadable to anyone using a screen reader. Nobody sighted will ever
// notice, which is why it has to be caught by a machine.
//
// Rewriting the text automatically would be worse: "Nahida: die erste Hündin"
// is not the author's sentence once a tool has been at it, and the author is
// the only one who knows what it should say instead.
func unsafeAltTexts(markdown string) []badAlt {
	var bad []badAlt
	check := func(text string) {
		if text == "" || altAllowed.MatchString(text) {
			return
		}
		var offenders []string
		seen := map[rune]bool{}
		for _, r := range text {
			if !altAllowed.MatchString(string(r)) && !seen[r] {
				seen[r] = true
				offenders = append(offenders, string(r))
			}
		}
		bad = append(bad, badAlt{
			text: text,
			why:  "bluemonday allows no " + strings.Join(offenders, " ") + " in an alt attribute",
		})
	}
	for _, m := range altTextPattern.FindAllStringSubmatch(markdown, -1) {
		check(m[1])
	}
	for _, m := range altAttrPattern.FindAllStringSubmatch(markdown, -1) {
		check(m[1])
	}
	return bad
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeZip(out string, m *bundle.Manifest, mediaDir string) error {
	// Built in memory and written in one go: a zip that was interrupted
	// half-way still looks like a file, and the next person to pick it up finds
	// out it is truncated during an import they cannot cleanly undo.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	w, err := zw.Create(bundle.ManifestName)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(body, '\n')); err != nil {
		return err
	}

	names := make([]string, 0, len(m.Media))
	for _, med := range m.Media {
		names = append(names, med.Filename)
	}
	sort.Strings(names)

	for _, name := range names {
		src, err := os.Open(filepath.Join(mediaDir, name))
		if err != nil {
			return err
		}
		// Already-compressed photographs: deflate spends real CPU time to make
		// them a percent smaller, occasionally larger.
		dst, err := zw.CreateHeader(&zip.FileHeader{Name: bundle.MediaDir + name, Method: zip.Store})
		if err != nil {
			src.Close()
			return err
		}
		_, err = io.Copy(dst, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

// join reports the problems once each.
//
// The same picture legitimately appears twice on a page — once as the preview,
// once as the link to the full size — so a broken name would otherwise be
// listed twice and the count would overstate how much is wrong.
func join(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	seen := map[string]bool{}
	unique := problems[:0]
	for _, p := range problems {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	sort.Strings(unique)
	return fmt.Errorf("%d problem(s):\n  - %s", len(unique), strings.Join(unique, "\n  - "))
}
