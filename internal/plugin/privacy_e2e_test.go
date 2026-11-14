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
	"time"

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
	return privacyInstallEdited(t, dir, nil)
}

// privacyInstallEdited is privacyInstall with the shipped manifest changed
// first, for a plugin made to claim what it does not do.
func privacyInstallEdited(t *testing.T, dir string, edit func(*Manifest)) privacyWorld {
	t.Helper()
	module := wasmtest.Module(t, filepath.Join("../../plugins", dir, ModuleName))
	manifest, err := os.ReadFile(filepath.Join("../../plugins", dir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		var m Manifest
		if err := json.Unmarshal(manifest, &m); err != nil {
			t.Fatal(err)
		}
		edit(&m)
		if manifest, err = json.Marshal(m); err != nil {
			t.Fatal(err)
		}
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

// A plugin that fails to answer must not read as one that found nothing:
// the Privacy screen would show "no messages" with the messages still there.
// The guest's other hooks turn an error into a log line and an empty answer;
// the privacy hook carries it back.
func TestAPrivacyErrorIsNotAnEmptyAnswer(t *testing.T) {
	w := privacyInstall(t, "kontaktformular")
	seedContactForm(t, w)
	ctx := context.Background()

	_, err := w.m.Privacy(ctx, w.a, w.id, PrivacyIn{Op: PrivacyFind, Email: "  "})
	if !errors.Is(err, ErrPrivacyFailed) {
		t.Errorf("a find the plugin refused answered %v, want ErrPrivacyFailed", err)
	}

	// A plugin that declares the hook and registered nothing for it.
	silent := privacyInstallEdited(t, "jahreszahl", func(m *Manifest) {
		m.Hooks = append(m.Hooks, HookPrivacy)
	})
	_, err = silent.m.Privacy(ctx, silent.a, silent.id, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"})
	if !errors.Is(err, ErrNoPrivacyAnswer) {
		t.Errorf("a plugin that answered nothing gave %v, want ErrNoPrivacyAnswer", err)
	}
}

func TestTheContactFormErasesWhatItFound(t *testing.T) {
	w := privacyInstall(t, "kontaktformular")
	seedContactForm(t, w)
	ctx := context.Background()

	// A key outside the plugin's records refuses the whole call, before
	// anything is deleted.
	for _, bad := range []string{"zaehler:2026-10-04T09", "other:1", "signaturschluessel"} {
		_, err := w.m.Privacy(ctx, w.a, w.id, PrivacyIn{Op: PrivacyErase, Keys: []string{contactKeyMessage, bad}})
		if err == nil {
			t.Errorf("erase with %q was accepted", bad)
		}
		if _, ok := w.stored(t, w.a, contactKeyMessage); !ok {
			t.Fatalf("erase with %q deleted the message before refusing", bad)
		}
	}

	found := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"})
	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyErase, Keys: privacyKeys(found.Records)})
	if out.Deleted != 3 {
		t.Errorf("Deleted = %d, want 3", out.Deleted)
	}
	if len(out.MediaIDs) != 1 || out.MediaIDs[0] != 7 {
		t.Errorf("MediaIDs = %v, want [7]: the attachment goes with the message", out.MediaIDs)
	}
	if again := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); len(again.Records) != 0 {
		t.Errorf("after erasing, find still returns %v", privacyKeys(again.Records))
	}
	if _, ok := w.stored(t, w.a, contactKeyStranger); !ok {
		t.Error("the stranger's message went too")
	}
	if v, ok, _ := w.store.StoreGet(ctx, w.id, w.a, "zaehler:2026-10-04T09"); !ok || v != "3" {
		t.Error("the hourly counter went too")
	}
}

func TestTheContactFormCorrectsNameAndEmail(t *testing.T) {
	w := privacyInstall(t, "kontaktformular")
	seedContactForm(t, w)
	ctx := context.Background()

	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyCorrect, Key: contactKeyMessage, Field: "email", Value: "ada.new@example.com"})
	if len(out.Records) != 1 || out.Records[0].Key != contactKeyMessage {
		t.Errorf("correct answered %+v, want the record as it now stands", out.Records)
	}
	found := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ADA.NEW@example.com"})
	if keys := privacyKeys(found.Records); len(keys) != 1 || keys[0] != contactKeyMessage {
		t.Errorf("a find for the new address returns %v", keys)
	}

	// The assembled form's address stands in its answer; correcting it there
	// is what makes the old address stop finding it.
	w.privacy(t, w.a, PrivacyIn{Op: PrivacyCorrect, Key: contactKeyAssembled, Field: "email", Value: "ada.new@example.com"})
	if found := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); len(found.Records) != 1 ||
		found.Records[0].Key != contactKeyQuarantine {
		t.Errorf("after correcting, the old address finds %v, want only the quarantined submission", privacyKeys(found.Records))
	}

	w.privacy(t, w.a, PrivacyIn{Op: PrivacyCorrect, Key: contactKeyQuarantine, Field: "name", Value: "Ada King"})
	if v, _ := w.stored(t, w.a, contactKeyQuarantine); v["name"] != "Ada King" {
		t.Errorf("the name was not stored: %v", v["name"])
	}

	for _, bad := range []PrivacyIn{
		{Op: PrivacyCorrect, Key: contactKeyMessage, Field: "text", Value: "anders"},
		{Op: PrivacyCorrect, Key: contactKeyMessage, Field: "email", Value: "a@example.com, b@example.com"},
		{Op: PrivacyCorrect, Key: contactKeyMessage, Field: "email", Value: "Ada <ada@example.com>"},
		{Op: PrivacyCorrect, Key: contactKeyMessage, Field: "email", Value: "kein-at"},
		{Op: PrivacyCorrect, Key: contactKeyMessage, Field: "name", Value: "  "},
		{Op: PrivacyCorrect, Key: "nachricht:2020-01-01T00:00:00Z-gibtsnicht", Field: "name", Value: "X"},
		{Op: PrivacyCorrect, Key: "zaehler:2026-10-04T09", Field: "name", Value: "X"},
	} {
		if _, err := w.m.Privacy(ctx, w.a, w.id, bad); err == nil {
			t.Errorf("correct %s=%q on %s was accepted", bad.Field, bad.Value, bad.Key)
		}
	}
	if v, _ := w.stored(t, w.a, contactKeyMessage); v["email"] != "ada.new@example.com" || v["text"] != "Habt ihr am Samstag Eier?" {
		t.Errorf("a refused correction changed the message: %v", v)
	}
}

func TestTheContactFormExpiresByTime(t *testing.T) {
	w := privacyInstall(t, "kontaktformular")
	now := time.Now().UTC()
	nowKey := now.Format(time.RFC3339) + "-eeeeeeee"
	w.seed(t, w.a, "nachricht:2024-01-10T09:00:00Z-old1", map[string]any{
		"kennung": "2024-01-10T09:00:00Z-old1", "name": "A", "email": "a@example.com", "text": "x",
		"zeit": "2024-01-10T09:00:00Z", "gelesen": false,
		"anhaenge": []map[string]any{{"medien_id": 11, "name": "a.pdf", "dateiname": "a.pdf"}},
	})
	w.seed(t, w.a, "abgewiesen:2024-02-10T09:00:00Z-old2", map[string]any{
		"kennung": "2024-02-10T09:00:00Z-old2", "name": "B", "email": "b@example.com", "text": "x",
		"zeit": "2024-02-10T09:00:00Z", "abgewiesen": "honeypot",
	})
	w.seed(t, w.a, "nachricht:2025-06-10T09:00:00Z-mid", map[string]any{
		"kennung": "2025-06-10T09:00:00Z-mid", "name": "C", "email": "c@example.com", "text": "x",
		"zeit": "2025-06-10T09:00:00Z",
	})
	w.seed(t, w.a, "nachricht:"+nowKey, map[string]any{
		"kennung": nowKey, "name": "D", "email": "d@example.com", "text": "x", "zeit": now.Format(time.RFC3339),
	})
	// Website B's old message is B's to expire, not A's.
	w.seed(t, w.b, "nachricht:2024-01-10T09:00:00Z-oldb", map[string]any{
		"kennung": "2024-01-10T09:00:00Z-oldb", "name": "E", "email": "e@example.com", "text": "x",
		"zeit": "2024-01-10T09:00:00Z",
	})

	if out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyExpire, Kind: PrivacyKindOrder, Before: "2025-01-01T00:00:00Z", Limit: 10}); out.Deleted != 0 || out.More {
		t.Errorf("expire of orders answered %+v, want nothing: the contact form keeps no orders", out)
	}

	var media []int64
	first := w.privacy(t, w.a, PrivacyIn{Op: PrivacyExpire, Kind: PrivacyKindFormSubmission, Before: "2025-01-01T00:00:00Z", Limit: 1})
	if first.Deleted != 1 || !first.More {
		t.Errorf("first batch = %+v, want 1 deleted and more", first)
	}
	media = append(media, first.MediaIDs...)
	second := w.privacy(t, w.a, PrivacyIn{Op: PrivacyExpire, Kind: PrivacyKindFormSubmission, Before: "2025-01-01T00:00:00Z", Limit: 1})
	if second.Deleted != 1 || second.More {
		t.Errorf("second batch = %+v, want 1 deleted and no more", second)
	}
	media = append(media, second.MediaIDs...)
	if len(media) != 1 || media[0] != 11 {
		t.Errorf("media ids of the expired messages = %v, want [11]", media)
	}

	for _, key := range []string{"nachricht:2024-01-10T09:00:00Z-old1", "abgewiesen:2024-02-10T09:00:00Z-old2"} {
		if _, ok := w.stored(t, w.a, key); ok {
			t.Errorf("%s is past the period and still there", key)
		}
	}
	for _, key := range []string{"nachricht:2025-06-10T09:00:00Z-mid", "nachricht:" + nowKey} {
		if _, ok := w.stored(t, w.a, key); !ok {
			t.Errorf("%s is inside the period and was expired", key)
		}
	}
	if _, ok := w.stored(t, w.b, "nachricht:2024-01-10T09:00:00Z-oldb"); !ok {
		t.Error("expiring website A took website B's message")
	}

	if _, err := w.m.Privacy(context.Background(), w.a, w.id, PrivacyIn{Op: PrivacyExpire, Kind: PrivacyKindFormSubmission, Before: "gestern"}); err == nil {
		t.Error("expire with a cut-off that is not a time was accepted")
	}
}

// The farm shop's orders, in the plugin's own shape.
func farmOrder(id, received, email string, done bool) map[string]any {
	return map[string]any{
		"id": id, "eingegangen": received, "name": "Ada Lovelace", "email": email,
		"telefon": "+41 79 123 45 67", "adresse": "Bahnhofstrasse 1, 8001 Zürich",
		"bemerkung": "Bitte vor 10 Uhr", "seite": "bestellen",
		"posten": []map[string]any{
			{"slug": "seife", "titel": "Schafmilchseife", "menge": 3, "preis": "8,50", "einheit": "Stück"},
		},
		"summe": 25.5, "summe_bekannt": true, "waehrung": "CHF", "erledigt": done,
	}
}

const (
	farmKeyOld      = "bestellung:20240315T100000-aaaaaaaa"
	farmKeyStranger = "bestellung:20250101T100000-cccccccc"
)

func seedFarmShop(t *testing.T, w privacyWorld) (nowKey string) {
	t.Helper()
	now := time.Now().UTC()
	nowKey = "bestellung:" + now.Format("20060102T150405") + "-bbbbbbbb"
	w.seed(t, w.a, farmKeyOld, farmOrder(farmKeyOld[len("bestellung:"):], "2024-03-15T10:00:00Z", "Ada@Example.com", true))
	w.seed(t, w.a, nowKey, farmOrder(nowKey[len("bestellung:"):], now.Format(time.RFC3339), "ada@example.com", false))
	w.seed(t, w.a, farmKeyStranger, farmOrder(farmKeyStranger[len("bestellung:"):], "2025-01-01T10:00:00Z", "bob@example.com", false))
	return nowKey
}

func TestTheFarmShopFindsOrders(t *testing.T) {
	w := privacyInstall(t, "bestellung")
	nowKey := seedFarmShop(t, w)

	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ADA@example.com"})
	got := privacyKeys(out.Records)
	if len(got) != 2 || got[0] != farmKeyOld || got[1] != nowKey {
		t.Fatalf("find returned %v, want [%s %s]", got, farmKeyOld, nowKey)
	}
	for _, rec := range out.Records {
		if rec.Kind != PrivacyKindOrder || rec.Label == "" || rec.Time == "" {
			t.Errorf("%s: kind %q label %q time %q", rec.Key, rec.Kind, rec.Label, rec.Time)
		}
		if want := "name,email,phone,address"; joinStrings(rec.Correctable) != want {
			t.Errorf("%s: correctable %v, want %s", rec.Key, rec.Correctable, want)
		}
		for _, name := range []string{"name", "email", "phone", "address", "note", "item_1", "sum", "currency", "received"} {
			if f, ok := privacyField(rec, name); !ok || f.Label == "" || f.Value == "" {
				t.Errorf("%s: field %s = %+v", rec.Key, name, f)
			}
		}
		if f, _ := privacyField(rec, "item_1"); !bytes.Contains([]byte(f.Value), []byte("Schafmilchseife")) {
			t.Errorf("%s: the item does not name the product: %q", rec.Key, f.Value)
		}
		_, done := privacyField(rec, "done")
		if done != (rec.Key == farmKeyOld) {
			t.Errorf("%s: done field present = %v", rec.Key, done)
		}
	}
	if out := w.privacy(t, w.b, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); len(out.Records) != 0 {
		t.Errorf("website B found %v", privacyKeys(out.Records))
	}
}

func joinStrings(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

func TestTheFarmShopReducesAnErasedOrder(t *testing.T) {
	w := privacyInstall(t, "bestellung")
	nowKey := seedFarmShop(t, w)
	ctx := context.Background()
	keys := []string{farmKeyOld, nowKey}

	// Without the date there is nothing to tell the person, so nothing is
	// erased.
	if _, err := w.m.Privacy(ctx, w.a, w.id, PrivacyIn{Op: PrivacyErase, Keys: keys}); err == nil {
		t.Error("an erase without retain_until was accepted")
	}
	if _, err := w.m.Privacy(ctx, w.a, w.id, PrivacyIn{Op: PrivacyErase, Keys: []string{farmKeyOld, "einstellungen"},
		RetainUntil: "2035-01-01T00:00:00Z"}); err == nil {
		t.Error("an erase with a key outside the orders was accepted")
	}
	if v, _ := w.stored(t, w.a, farmKeyOld); v["email"] != "Ada@Example.com" || v["telefon"] == "" {
		t.Fatalf("a refused erase changed the order: %v", v)
	}

	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyErase, Keys: keys, RetainUntil: "2035-01-01T00:00:00Z"})
	if out.Deleted != 0 || len(out.Kept) != 2 {
		t.Fatalf("erase answered %+v, want nothing deleted and two kept", out)
	}
	for _, k := range out.Kept {
		if joinStrings(k.Fields) != "name,address,items,sum,note" || k.Until != "2035-01-01T00:00:00Z" {
			t.Errorf("kept %+v, want name,address,items,sum,note until 2035-01-01T00:00:00Z", k)
		}
	}
	if again := w.privacy(t, w.a, PrivacyIn{Op: PrivacyFind, Email: "ada@example.com"}); len(again.Records) != 0 {
		t.Errorf("an erased order is still found by the address: %v", privacyKeys(again.Records))
	}
	v, _ := w.stored(t, w.a, farmKeyOld)
	if v["email"] != "" || v["telefon"] != nil && v["telefon"] != "" {
		t.Errorf("e-mail or phone survived the erasure: %v", v)
	}
	if v["name"] != "Ada Lovelace" || v["adresse"] != "Bahnhofstrasse 1, 8001 Zürich" ||
		v["bemerkung"] != "Bitte vor 10 Uhr" || v["summe"] != 25.5 {
		t.Errorf("the invoice basis did not survive: %v", v)
	}
	if posten, _ := v["posten"].([]any); len(posten) != 1 {
		t.Errorf("the items did not survive: %v", v["posten"])
	}
	if v["aufbewahren_bis"] != "2035-01-01T00:00:00Z" || v["geloescht"] == nil || v["geloescht"] == "" {
		t.Errorf("the reduction is not recorded on the order: %v", v)
	}
	if v, _ := w.stored(t, w.a, farmKeyStranger); v["email"] != "bob@example.com" {
		t.Error("the stranger's order was touched")
	}
}

func TestTheFarmShopRefusesToCorrectAnErasedOrder(t *testing.T) {
	w := privacyInstall(t, "bestellung")
	nowKey := seedFarmShop(t, w)
	ctx := context.Background()
	w.privacy(t, w.a, PrivacyIn{Op: PrivacyErase, Keys: []string{farmKeyOld}, RetainUntil: "2035-01-01T00:00:00Z"})

	if _, err := w.m.Privacy(ctx, w.a, w.id, PrivacyIn{Op: PrivacyCorrect, Key: farmKeyOld, Field: "name", Value: "Ada King"}); err == nil {
		t.Error("a correction of a reduced order was accepted")
	}
	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyCorrect, Key: nowKey, Field: "phone", Value: "+41 79 000 00 00"})
	if len(out.Records) != 1 {
		t.Errorf("correct answered %+v", out)
	}
	if v, _ := w.stored(t, w.a, nowKey); v["telefon"] != "+41 79 000 00 00" {
		t.Errorf("the phone number was not stored: %v", v["telefon"])
	}
	for _, bad := range []PrivacyIn{
		{Op: PrivacyCorrect, Key: nowKey, Field: "email", Value: "Ada <ada@example.com>"},
		{Op: PrivacyCorrect, Key: nowKey, Field: "note", Value: "x"},
		{Op: PrivacyCorrect, Key: nowKey, Field: "name", Value: " "},
		{Op: PrivacyCorrect, Key: "einstellungen", Field: "name", Value: "x"},
	} {
		if _, err := w.m.Privacy(ctx, w.a, w.id, bad); err == nil {
			t.Errorf("correct %s=%q on %s was accepted", bad.Field, bad.Value, bad.Key)
		}
	}
}

func TestTheFarmShopExpiresOnlyAfterTheRetentionDate(t *testing.T) {
	w := privacyInstall(t, "bestellung")
	const (
		never    = "bestellung:20140501T100000-aaaaaaaa"
		retained = "bestellung:20140601T100000-bbbbbbbb"
		recent   = "bestellung:20240101T100000-cccccccc"
	)
	w.seed(t, w.a, never, farmOrder(never[len("bestellung:"):], "2014-05-01T10:00:00Z", "a@example.com", true))
	w.seed(t, w.a, retained, farmOrder(retained[len("bestellung:"):], "2014-06-01T10:00:00Z", "b@example.com", true))
	w.seed(t, w.a, recent, farmOrder(recent[len("bestellung:"):], "2024-01-01T10:00:00Z", "c@example.com", false))
	w.privacy(t, w.a, PrivacyIn{Op: PrivacyErase, Keys: []string{retained},
		RetainUntil: time.Now().UTC().AddDate(2, 0, 0).Format(time.RFC3339)})

	if out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyExpire, Kind: PrivacyKindFormSubmission, Before: "2016-01-01T00:00:00Z"}); out.Deleted != 0 {
		t.Errorf("expire of form submissions deleted %d orders", out.Deleted)
	}
	out := w.privacy(t, w.a, PrivacyIn{Op: PrivacyExpire, Kind: PrivacyKindOrder, Before: "2016-01-01T00:00:00Z"})
	if out.Deleted != 1 || out.More {
		t.Errorf("expire answered %+v, want the 2014-05 order alone", out)
	}
	if _, ok := w.stored(t, w.a, never); ok {
		t.Error("the order past its period is still there")
	}
	if _, ok := w.stored(t, w.a, retained); !ok {
		t.Error("an order kept until a date in the future was expired before it")
	}
	if _, ok := w.stored(t, w.a, recent); !ok {
		t.Error("an order inside the period was expired")
	}
}
