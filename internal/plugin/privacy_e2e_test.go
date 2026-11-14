package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/plugin/wasmtest"
)

// The privacy hook (Phase 33.2, DSR-01, D-09) against the real modules.
//
// Form submissions and farm-shop orders are not in a core table: each plugin
// keeps them as its own JSON in plugin_store, and only the plugin can say
// which of them belong to an address. These tests seed the stores in the
// plugins' own shape — that shape is the fixture here, never something the
// host reads — and ask the shipped modules through Manager.Privacy.

// privacyWorld is a manager with two websites and one shipped plugin switched
// on for both.
type privacyWorld struct {
	m     *Manager
	store *Store
	a, b  int64
	id    string
}

// privacyInstall installs plugins/<dir> from its plugin.json and its built
// module, the way an operator's upload arrives, and switches it on for both
// websites.
func privacyInstall(t *testing.T, dir string) privacyWorld {
	t.Helper()
	module := wasmtest.Module(t, filepath.Join("../../plugins", dir, ModuleName))
	manifest, err := os.ReadFile(filepath.Join("../../plugins", dir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	database, err := db.Open(filepath.Join(tmp, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sites := domain.NewStore(database)
	a, err := sites.CreateWebsite(ctx, "Farmstand", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := sites.CreateWebsite(ctx, "Bakery", "")
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(database)
	rt, err := NewRuntime(ctx, store, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rt.Close(context.Background()) })
	m, err := NewManager(ctx, store, rt, tmp, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(ManifestName)
	w.Write(manifest)
	w, _ = zw.Create(ModuleName)
	w.Write(module)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	man, err := m.Install(ctx, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Install %s: %v", dir, err)
	}
	if err := m.Enable(ctx, man.ID); err != nil {
		t.Fatalf("Enable %s: %v", dir, err)
	}
	if err := m.SetWebsites(ctx, man.ID, []int64{a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	return privacyWorld{m: m, store: store, a: a.ID, b: b.ID, id: man.ID}
}

// seed writes one value in the plugin's own store, as the plugin would.
func (w privacyWorld) seed(t *testing.T, websiteID int64, key string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.store.StoreSet(context.Background(), w.id, websiteID, key, string(raw)); err != nil {
		t.Fatal(err)
	}
}

// stored reads one value back, as the plugin's own JSON.
func (w privacyWorld) stored(t *testing.T, websiteID int64, key string) (map[string]any, bool) {
	t.Helper()
	raw, ok, err := w.store.StoreGet(context.Background(), w.id, websiteID, key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return nil, false
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v, true
}

func (w privacyWorld) privacy(t *testing.T, websiteID int64, in PrivacyIn) *PrivacyOut {
	t.Helper()
	out, err := w.m.Privacy(context.Background(), websiteID, w.id, in)
	if err != nil {
		t.Fatalf("Privacy(%s): %v", in.Op, err)
	}
	return out
}

func privacyKeys(recs []PrivacyRecord) []string {
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.Key)
	}
	sort.Strings(out)
	return out
}

func privacyField(rec PrivacyRecord, name string) (PrivacyField, bool) {
	for _, f := range rec.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return PrivacyField{}, false
}

// The contact form's four kinds of entry for one person and one stranger.
const (
	contactKeyMessage    = "nachricht:2025-03-01T09:00:00Z-aaaaaaaa"
	contactKeyQuarantine = "abgewiesen:2026-10-02T09:00:00Z-bbbbbbbb"
	contactKeyAssembled  = "nachricht:2026-10-03T09:00:00Z-cccccccc"
	contactKeyStranger   = "nachricht:2026-10-04T09:00:00Z-dddddddd"
)

func seedContactForm(t *testing.T, w privacyWorld) {
	t.Helper()
	w.seed(t, w.a, contactKeyMessage, map[string]any{
		"kennung": "2025-03-01T09:00:00Z-aaaaaaaa", "name": "Ada Lovelace", "email": "Ada@Example.com",
		"betreff": "Eier", "text": "Habt ihr am Samstag Eier?", "seite": "kontakt",
		"zeit": "2025-03-01T09:00:00Z", "gelesen": true,
		"einwilligung": "Ich bin einverstanden.", "einwilligungszeit": "2025-03-01T09:00:00Z",
		"anhaenge": []map[string]any{{"medien_id": 7, "name": "brief.pdf", "dateiname": "x1.pdf"}},
		"antwort":  "Ja, ab acht Uhr.", "antwortzeit": "2025-03-02T08:00:00Z",
	})
	w.seed(t, w.a, contactKeyQuarantine, map[string]any{
		"kennung": "2026-10-02T09:00:00Z-bbbbbbbb", "name": "Ada", "email": "ada@example.com",
		"text": "Noch eine Frage", "zeit": "2026-10-02T09:00:00Z", "abgewiesen": "honeypot",
	})
	w.seed(t, w.a, contactKeyAssembled, map[string]any{
		"kennung": "2026-10-03T09:00:00Z-cccccccc", "name": "Ada", "email": "",
		"text": "", "zeit": "2026-10-03T09:00:00Z", "formular": "fest", "formularname": "Hoffest",
		"felder": []map[string]any{
			{"beschriftung": "Wer kommt", "wert": "Ada"},
			{"beschriftung": "Adresse für die Bestätigung", "wert": " ADA@example.com "},
		},
	})
	w.seed(t, w.a, contactKeyStranger, map[string]any{
		"kennung": "2026-10-04T09:00:00Z-dddddddd", "name": "Bob", "email": "bob@example.com",
		"text": "Hallo", "zeit": "2026-10-04T09:00:00Z",
	})
	// Not a message: the hourly counter and the signing key live beside them.
	if err := w.store.StoreSet(context.Background(), w.id, w.a, "zaehler:2026-10-04T09", "3"); err != nil {
		t.Fatal(err)
	}
}

func TestTheContactFormAnswersFind(t *testing.T) {
	w := privacyInstall(t, "kontaktformular")
	seedContactForm(t, w)

	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"})
	got := privacyKeys(out.Records)
	want := []string{contactKeyQuarantine, contactKeyMessage, contactKeyAssembled}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("find returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("find returned %v, want %v", got, want)
		}
	}
	for _, rec := range out.Records {
		if rec.Kind != PrivacyKindFormSubmission {
			t.Errorf("%s: kind %q, want %q", rec.Key, rec.Kind, PrivacyKindFormSubmission)
		}
		if rec.Label == "" || rec.Time == "" {
			t.Errorf("%s: label %q time %q, want both", rec.Key, rec.Label, rec.Time)
		}
		if len(rec.Correctable) != 2 || rec.Correctable[0] != "name" || rec.Correctable[1] != "email" {
			t.Errorf("%s: correctable %v, want [name email]", rec.Key, rec.Correctable)
		}
		switch rec.Key {
		case contactKeyMessage:
			if len(rec.MediaIDs) != 1 || rec.MediaIDs[0] != 7 {
				t.Errorf("the message's attachment is missing: media ids %v", rec.MediaIDs)
			}
			for name, value := range map[string]string{
				"name": "Ada Lovelace", "email": "Ada@Example.com", "subject": "Eier",
				"text": "Habt ihr am Samstag Eier?", "page": "kontakt", "attachment_1": "brief.pdf",
				"reply": "Ja, ab acht Uhr.",
			} {
				if f, ok := privacyField(rec, name); !ok || f.Value != value || f.Label == "" {
					t.Errorf("field %s = %+v, want value %q with a label", name, f, value)
				}
			}
			if f, ok := privacyField(rec, "consent"); !ok || !bytes.Contains([]byte(f.Value), []byte("Ich bin einverstanden.")) {
				t.Errorf("the consent sentence is missing: %+v", f)
			}
		case contactKeyAssembled:
			if rec.Label != "Hoffest" {
				t.Errorf("an assembled form's message is labelled %q, want its form's name", rec.Label)
			}
			found := false
			for _, f := range rec.Fields {
				if f.Label == "Adresse für die Bestätigung" && f.Value == "ADA@example.com" {
					found = true
				}
			}
			if !found {
				t.Errorf("the e-mail answer is not among the fields: %+v", rec.Fields)
			}
		}
	}

	if out := w.privacy(t, w.b, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); len(out.Records) != 0 {
		t.Errorf("website B found %v", privacyKeys(out.Records))
	}
}

// find is a question, never a change: the host runs it read-only, and a
// plugin that writes anyway is a failed answer, not a quiet success.
func TestFindCannotWrite(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, func(man *Manifest) {
		man.Hooks = append(man.Hooks, HookPrivacy)
	}))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}

	_, err := m.Privacy(ctx, site, "echo", PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"})
	if !errors.Is(err, ErrWroteDuringFind) {
		t.Fatalf("a find that writes answered %v, want ErrWroteDuringFind", err)
	}
	if _, ok, _ := store.StoreGet(ctx, "echo", site, "privacy"); ok {
		t.Error("the write of a find reached the store")
	}

	// The refusal belongs to find, not to the plugin: the same write through
	// the event hook goes through.
	if err := m.rt.Dispatch(ctx, "echo", HookEvent, site, EventIn{Name: "x", WebsiteID: site,
		Data: map[string]string{"tue": "schreiben", "key": "privacy", "value": "written"}}, nil); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := store.StoreGet(ctx, "echo", site, "privacy"); !ok || v != "written" {
		t.Errorf("the event hook could not write: %q %v", v, ok)
	}
}

func TestAPluginWithoutTheHookIsNotServed(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()
	einspielen(t, m, echoArchive(t, nil))
	if err := m.Enable(ctx, "echo"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetWebsites(ctx, "echo", []int64{site}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Privacy(ctx, site, "echo", PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); !errors.Is(err, ErrNotServed) {
		t.Errorf("a plugin without the privacy hook answered %v, want ErrNotServed", err)
	}
	if _, err := m.Privacy(ctx, site, "nobody", PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); !errors.Is(err, ErrNotServed) {
		t.Errorf("a plugin that is not installed answered %v, want ErrNotServed", err)
	}
}
