package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// shippedZip reads a committed plugin archive as it stands. It is never
// rebuilt or repacked here: the point is that what ships installs.
func shippedZip(t *testing.T, id string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "plugins", id, id+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEveryShippedZipInstallsOnAnEmptyDatabase(t *testing.T) {
	m, _, site := neuerManager(t)
	ctx := context.Background()

	ids := []string{"bestellung", "jahreszahl", "kontaktformular", "nicht-gefunden", "suche"}
	for _, id := range ids {
		if man := einspielen(t, m, shippedZip(t, id)); man.ID != id {
			t.Fatalf("%s.zip carries plugin %q", id, man.ID)
		}
		if err := m.SetWebsites(ctx, id, []int64{site}); err != nil {
			t.Fatalf("%s: SetWebsites: %v", id, err)
		}
		if err := m.Enable(ctx, id); err != nil {
			t.Fatalf("%s: Enable: %v", id, err)
		}
	}

	all, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, st := range all {
		seen[st.ID] = true
		if !st.Running || st.LastError != "" {
			t.Errorf("%s: running=%v, last error %q", st.ID, st.Running, st.LastError)
		}
	}
	for _, id := range ids {
		if !seen[id] {
			t.Errorf("%s is not in the list", id)
		}
	}
}

func TestTheContactFormMigrationCopiesTheOldMessages(t *testing.T) {
	m, store, site := neuerManager(t)
	ctx := context.Background()

	seedLegacyMessages(t, store, site)
	// A second message, already read.
	if _, err := store.DB.Write.ExecContext(ctx, `
		INSERT INTO form_messages (website_id, page_id, name, email, subject, body, created_at, read_at)
		VALUES ($1, NULL, 'Bruno', 'bruno@example.org', 'Frage', 'Noch da?', '2025-02-03T04:05:06Z', '2025-02-04T00:00:00Z')`, site); err != nil {
		t.Fatal(err)
	}

	zip := shippedZip(t, "kontaktformular")
	einspielen(t, m, zip)

	read := func() map[string]string {
		rows, err := store.DB.Read.QueryContext(ctx,
			`SELECT key, value FROM plugin_store WHERE plugin_id = 'kontaktformular' ORDER BY key`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				t.Fatal(err)
			}
			out[k] = v
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}

	first := read()
	if len(first) != 2 {
		t.Fatalf("%d rows copied, want 2: %v", len(first), first)
	}
	var withPage, withoutPage int
	for k, v := range first {
		var msg struct {
			Seite   string `json:"seite"`
			Gelesen bool   `json:"gelesen"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		switch msg.Name {
		case "Anna":
			if msg.Seite != "kontakt" || msg.Gelesen {
				t.Errorf("Anna: %+v", msg)
			}
			withPage++
		case "Bruno":
			if msg.Seite != "" || !msg.Gelesen {
				t.Errorf("Bruno: %+v", msg)
			}
			withoutPage++
		}
	}
	if withPage != 1 || withoutPage != 1 {
		t.Errorf("messages by name: %v", first)
	}

	// The same archive a second time applies nothing and leaves the rows alone.
	einspielen(t, m, zip)
	second := read()
	if len(second) != len(first) {
		t.Fatalf("%d rows after the second install, were %d", len(second), len(first))
	}
	for k, v := range first {
		if second[k] != v {
			t.Errorf("row %s changed on the second install", k)
		}
	}
}
