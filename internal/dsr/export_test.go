package dsr

import (
	"context"
	"encoding/json"
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
