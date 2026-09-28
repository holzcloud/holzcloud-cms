package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// writeThumbnailFixture writes a 1000x600 PNG and returns its size.
//
// The opaque one carries seeded noise over a gradient, like a photo saved as
// PNG: noisy enough that its 800px JPEG copy is far smaller than the PNG and is
// kept. The transparent one has a fully transparent band over its top third.
func writeThumbnailFixture(t *testing.T, path string, transparent bool) int64 {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	img := image.NewNRGBA(image.Rect(0, 0, 1000, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 1000; x++ {
			n := rng.Intn(41) - 20
			c := func(v int) uint8 { return uint8(max(0, min(255, v+n))) }
			a := uint8(255)
			if transparent && y < 200 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{c(x / 4), c(y / 3), c(120), a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return int64(buf.Len())
}

// assertNoDanglingCopies checks the two promises `thumbnails -force` has to
// keep: every stored row names a file that is on disk, and every file in the
// website directory is an original or named by a row.
func assertNoDanglingCopies(t *testing.T, database *db.DB, websiteID int64, dir string) {
	t.Helper()
	rows, err := database.Read.Query(
		`SELECT m.filename, COALESCE(v.filename, '') FROM media m
		 LEFT JOIN media_variants v ON v.media_id = m.id WHERE m.website_id = $1`, websiteID)
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	defer rows.Close()
	known := map[string]bool{}
	var named []string
	for rows.Next() {
		var original, variant string
		if err := rows.Scan(&original, &variant); err != nil {
			t.Fatalf("scan: %v", err)
		}
		known[original] = true
		if variant != "" {
			known[variant] = true
			named = append(named, variant)
		}
	}
	for _, name := range named {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("a row names %s, which is not on disk", name)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !known[e.Name()] {
			t.Errorf("%s is on disk, but no row names it", e.Name())
		}
	}
}

// The command an operator runs once after the update: an opaque PNG's old
// .png copies become .jpg copies and the old files go, a transparent PNG keeps
// its .png copies, and nothing points at a missing file — after one run and
// after a second.
func TestThumbnailsForceSwapsOldPNGCopies(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOLZCLOUD_DATA_DIR", dataDir)

	database, err := db.Open(filepath.Join(dataDir, "holzcloud.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	ctx := context.Background()
	ws, err := domain.NewStore(database).CreateWebsite(ctx, "Foto", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	dir := media.WebsiteDir(dataDir, ws.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	store := media.NewStore(database)
	seed := func(name string, transparent bool, old []media.Variant) int64 {
		size := writeThumbnailFixture(t, filepath.Join(dir, name), transparent)
		m, err := store.Create(ctx, ws.ID, name, name, "image/png", size, "hash-"+name)
		if err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
		for _, v := range old {
			if err := os.WriteFile(filepath.Join(dir, v.Filename), []byte("old"), 0o644); err != nil {
				t.Fatalf("write %s: %v", v.Filename, err)
			}
		}
		if err := store.SaveVariants(ctx, dir, m.ID, 1000, 600, old); err != nil {
			t.Fatalf("SaveVariants %s: %v", name, err)
		}
		return m.ID
	}
	foto := seed("foto.png", false, []media.Variant{
		{Label: "thumb", Filename: "foto-thumb.png", Width: 400, Height: 240},
		{Label: "medium", Filename: "foto-medium.png", Width: 800, Height: 480},
	})
	logo := seed("logo.png", true, []media.Variant{
		{Label: "thumb", Filename: "logo-thumb.png", Width: 400, Height: 240},
	})

	for run := 1; run <= 2; run++ {
		if err := cmdThumbnails([]string{"-force"}); err != nil {
			t.Fatalf("run %d: thumbnails -force: %v", run, err)
		}

		fotoRows, _ := store.VariantsFor(ctx, foto)
		thumb := ""
		for _, v := range fotoRows {
			if strings.HasSuffix(v.Filename, ".png") {
				t.Errorf("run %d: foto still has a PNG copy %s", run, v.Filename)
			}
			if v.Label == "thumb" {
				thumb = v.Filename
			}
		}
		if thumb != "foto-thumb.jpg" {
			t.Errorf("run %d: foto's thumbnail is %q, want foto-thumb.jpg", run, thumb)
		}
		for _, gone := range []string{"foto-thumb.png", "foto-medium.png"} {
			if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
				t.Errorf("run %d: %s is still on disk", run, gone)
			}
		}

		logoRows, _ := store.VariantsFor(ctx, logo)
		logoThumb := ""
		for _, v := range logoRows {
			if v.Label == "thumb" {
				logoThumb = v.Filename
			}
		}
		if logoThumb != "logo-thumb.png" {
			t.Errorf("run %d: the transparent logo's thumbnail is %q, want logo-thumb.png", run, logoThumb)
		}
		if _, err := os.Stat(filepath.Join(dir, "logo-thumb.png")); err != nil {
			t.Errorf("run %d: logo-thumb.png is gone: %v", run, err)
		}

		assertNoDanglingCopies(t, database, ws.ID, dir)
	}
}
