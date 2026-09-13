package template

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubWords is a WordResolver that answers from a map, or fails.
type stubWords struct {
	words map[string]map[string]string
	fail  bool
}

func (s stubWords) ThemeWords(_ context.Context, _ int64, locale string) (map[string]string, error) {
	if s.fail {
		return nil, errors.New("the database is busy")
	}
	return s.words[locale], nil
}

// installTheme writes a theme the way an upload would.
func installTheme(t *testing.T, files map[string]string) (dir, slug string) {
	t.Helper()
	dir, slug = t.TempDir(), "eigen"
	themeDir := filepath.Join(dir, "templates", slug)
	if err := os.MkdirAll(filepath.Join(themeDir, "lang"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(themeDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, slug
}

var wordingTheme = map[string]string{
	"layout.html":  `<html><body><nav>{{t "Cart"}}|{{t "Search"}}</nav>{{template "content" .}}</body></html>`,
	"page.html":    `{{define "content"}}x{{end}}`,
	"lang/de.json": `{"Cart":"Warenkorb","Search":"Suche"}`,
}

// Precedence: the operator's word, then the theme's, then the key.
//
// All three have to be reachable from one render, because the mistake this
// guards against is a merge that drops the theme's words for every key the
// operator did not touch — which would leave a page with one right word and
// twenty English ones.
func TestTheOperatorsWordWinsAndTheRestStay(t *testing.T) {
	dir, slug := installTheme(t, wordingTheme)
	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: slug})
	loader.SetWording(stubWords{words: map[string]map[string]string{
		"de": {"Cart": "Korb"},
	}})

	data := testData()
	data.Site.Locale = "de"
	out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "Korb|Suche") {
		t.Fatalf("want the operator's word and the theme's beside it, got:\n%s", body)
	}
}

// A language the operator has said nothing about is the theme's, untouched.
func TestALanguageWithNoOwnWordsIsTheThemesEntirely(t *testing.T) {
	dir, slug := installTheme(t, wordingTheme)
	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: slug})
	loader.SetWording(stubWords{words: map[string]map[string]string{
		"fr": {"Cart": "Panier"},
	}})

	data := testData()
	data.Site.Locale = "de"
	out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "Warenkorb|Suche") {
		t.Errorf("French overrides reached a German page:\n%s", out)
	}
}

// A resolver that fails renders the theme's words rather than an error page.
//
// The wording is a convenience; the site is not. A settings table that is busy
// for a moment must not be able to answer a visitor with a 500.
func TestAFailingResolverFallsBackToTheThemeRatherThanFailing(t *testing.T) {
	dir, slug := installTheme(t, wordingTheme)
	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: slug})
	loader.SetWording(stubWords{fail: true})

	data := testData()
	data.Site.Locale = "de"
	out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("a failing wording store took the page down: %v", err)
	}
	if !strings.Contains(string(out), "Warenkorb|Suche") {
		t.Errorf("the theme's own words are not on the page:\n%s", out)
	}
}

// A changed word reaches the page only once the cache is invalidated, and the
// test says so both ways round — the stale render is the bug that would
// otherwise be found by an operator saving and seeing nothing happen.
func TestAChangedWordNeedsTheCacheInvalidated(t *testing.T) {
	dir, slug := installTheme(t, wordingTheme)
	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: slug})
	live := map[string]map[string]string{"de": {"Cart": "Korb"}}
	loader.SetWording(stubWords{words: live})

	data := testData()
	data.Site.Locale = "de"
	render := func() string {
		t.Helper()
		out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return string(out)
	}

	if !strings.Contains(render(), "Korb") {
		t.Fatal("the first render does not carry the operator's word")
	}

	live["de"]["Cart"] = "Merkliste"
	if !strings.Contains(render(), "Korb") {
		t.Error("the cache is not holding the parsed set; this test no longer proves anything")
	}

	loader.InvalidateTemplateCache(1)
	if !strings.Contains(render(), "Merkliste") {
		t.Error("after invalidating, the new word does not reach the page — an operator saves and sees nothing")
	}
}

// An empty override is not a word.
//
// The store deletes an emptied entry rather than storing "", but a row from an
// older build or a hand-edited database must not be able to blank a button.
func TestAnEmptyOverrideDoesNotBlankTheWord(t *testing.T) {
	dir, slug := installTheme(t, wordingTheme)
	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: slug})
	loader.SetWording(stubWords{words: map[string]map[string]string{
		"de": {"Cart": ""},
	}})

	data := testData()
	data.Site.Locale = "de"
	out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "Warenkorb|Suche") {
		t.Errorf("an empty override blanked a word:\n%s", out)
	}
}

// An override for a word the theme does not mint changes nothing and breaks
// nothing. That is the ordinary state after switching themes.
func TestAnOverrideForAWordTheThemeDoesNotMintIsHarmless(t *testing.T) {
	dir, slug := installTheme(t, wordingTheme)
	loader := NewLoader(dir, testTemplateFS(), nil, stubResolver{slug: slug})
	loader.SetWording(stubWords{words: map[string]map[string]string{
		"de": {"Checkout": "Zur Kasse", "Cart": "Korb"},
	}})

	data := testData()
	data.Site.Locale = "de"
	out, err := loader.RenderPage(context.Background(), 1, "page.html", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(out)
	if !strings.Contains(body, "Korb|Suche") {
		t.Errorf("the words that do exist are wrong:\n%s", body)
	}
	if strings.Contains(body, "Zur Kasse") {
		t.Errorf("a word no template asks for reached the page:\n%s", body)
	}
}
