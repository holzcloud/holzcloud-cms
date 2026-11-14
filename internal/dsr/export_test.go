package dsr

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTheExportHoldsTheRecords(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "Holzwerk")
	number := findOrder(t, database, a, "2026-0003", "Ada@Example.COM")
	res, err := NewService(database).Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	now := time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC)
	raw, err := json.Marshal(BuildExport(res, a, "Holzwerk", now))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got struct {
		GeneratedAt string `json:"generated_at"`
		Website     struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"website"`
		Email   string `json:"email"`
		Records map[string][]struct {
			Key    string            `json:"key"`
			Kind   string            `json:"kind"`
			Time   string            `json:"time"`
			Fields map[string]string `json:"fields"`
		} `json:"records"`
		NotCovered json.RawMessage `json:"not_covered"`
		Incomplete json.RawMessage `json:"incomplete"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, raw)
	}
	if _, err := time.Parse(time.RFC3339, got.GeneratedAt); err != nil {
		t.Errorf("generated_at = %q is not RFC 3339", got.GeneratedAt)
	}
	if got.Website.ID != a || got.Website.Name != "Holzwerk" {
		t.Errorf("website = %+v", got.Website)
	}
	if got.Email != "ada@example.com" {
		t.Errorf("email = %q", got.Email)
	}
	orders := got.Records["orders"]
	if len(orders) != 1 || orders[0].Key != number {
		t.Fatalf("records.orders = %+v, want the order %s", orders, number)
	}
	if orders[0].Fields["name"] != "Ada Lovelace" {
		t.Errorf("fields.name = %q", orders[0].Fields["name"])
	}
	if orders[0].Kind != "order" || orders[0].Time == "" {
		t.Errorf("kind/time = %q/%q", orders[0].Kind, orders[0].Time)
	}
	if string(got.NotCovered) != "[]" {
		t.Errorf("not_covered = %s, want []", got.NotCovered)
	}
	if string(got.Incomplete) != "[]" {
		t.Errorf("incomplete = %s, want []", got.Incomplete)
	}
}

// T-33.2-03: the file holds the person's data and no credential. Every
// secret the database keeps about the account and the order is searched for
// in the JSON text, so a field added to a source later cannot carry one past
// this test.
func TestTheExportCarriesNoSecret(t *testing.T) {
	_, database := newTestStore(t)
	a := website(t, database, "A")
	userID := findUser(t, database, "ada@example.com", "Ada Lovelace", "editor", false)
	findExec(t, database, `INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, 'RECOVERY-HASH-1'), ($1, 'RECOVERY-HASH-2')`, userID)
	findExec(t, database, `INSERT INTO user_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1, 'TOKEN-HASH-RESET', 'reset', '2030-01-01T00:00:00Z')`, userID)
	number := findOrder(t, database, a, "2026-0601", "ADA@example.com")

	secrets := map[string]string{}
	var password, totp, pending, token string
	if err := database.Read.QueryRow(`SELECT password, totp_secret, totp_pending_secret FROM users WHERE id = $1`, userID).
		Scan(&password, &totp, &pending); err != nil {
		t.Fatal(err)
	}
	if err := database.Read.QueryRow(`SELECT token FROM orders WHERE number = $1`, number).Scan(&token); err != nil {
		t.Fatal(err)
	}
	secrets["password"] = password
	secrets["totp_secret"] = totp
	secrets["totp_pending_secret"] = pending
	secrets["recovery code"] = "RECOVERY-HASH-1"
	secrets["second recovery code"] = "RECOVERY-HASH-2"
	secrets["token hash"] = "TOKEN-HASH-RESET"
	secrets["order token"] = token

	res, err := NewService(database).Find(context.Background(), a, "ada@example.com")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	raw, err := json.Marshal(BuildExport(res, a, "A", time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for what, secret := range secrets {
		if secret == "" {
			t.Fatalf("the fixture stored no %s; the test would prove nothing", what)
		}
		if strings.Contains(text, secret) {
			t.Errorf("the export carries the %s", what)
		}
	}
	for _, want := range []string{"Ada Lovelace", "ada@example.com", number} {
		if !strings.Contains(text, want) {
			t.Errorf("the export lacks %q", want)
		}
	}
	if len(res.BySource()["accounts"]) != 1 || len(res.BySource()["orders"]) != 1 {
		t.Errorf("the export should hold the account and the order: %s", text)
	}
}
