package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/db/dbtest"
)

func TestTheStartupLineSizesTheGateOverStoredHashes(t *testing.T) {
	database := dbtest.Open(t)
	ctx := context.Background()
	big, err := auth.HashPassword("geheim-geheim", auth.Argon2Params{Memory: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`INSERT INTO users (email, password, role) VALUES ('a@example.com', ?, 'admin')`, big); err != nil {
		t.Fatal(err)
	}

	got, err := largestStoredArgon2Memory(ctx, database.Read)
	if err != nil || got != 64 {
		t.Fatalf("largestStoredArgon2Memory = %d, %v; want 64", got, err)
	}

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	logPasswordChecksBounded(ctx, database.Read, 2, 8)
	out := buf.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "largest_stored_kb=64") || !strings.Contains(out, "slots=2") {
		t.Errorf("a stored hash larger than the configured memory should warn with the sizes: %s", out)
	}

	buf.Reset()
	logPasswordChecksBounded(ctx, database.Read, 2, 128)
	if out := buf.String(); !strings.Contains(out, "level=INFO") || !strings.Contains(out, "peak_mb=1") {
		t.Errorf("expected an Info line with the peak, 2 x 128 KiB rounded up to 1 MiB: %s", out)
	}
}
