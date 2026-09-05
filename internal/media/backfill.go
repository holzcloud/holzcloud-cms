package media

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
)

// Backfill gives stored images the dimensions and the scaled copies they should
// have had from the beginning.
//
// The upload path measures every picture and writes its variants; the bundle
// import does not — it stores the bytes and the row and nothing else. Every
// picture on a website that came out of an archive therefore has width 0 in the
// database, and from that follows everything a visitor notices: no width and
// height in the HTML, so the page jumps as each picture arrives, and no srcset
// at all, so a phone downloads the full-size original. On one of the websites
// this server carries that is eighty-five pictures.
//
// After the fact rather than inside the import: a website with a hundred
// pictures would decode a hundred images inside one HTTP request, and an import
// that times out is worse than an import whose pictures are sharp a few minutes
// later. The work is idempotent and self-limiting — a row that has its size is
// never looked at again — so this may simply run on a timer for ever.
//
// `limit` bounds one pass. Decoding holds a whole bitmap in memory, and
// MakeVariantsThrottled already allows only two at a time; the limit is what
// keeps a first run on a large library from occupying those two slots for
// minutes on end.
func Backfill(ctx context.Context, store *Store, dataDir string, maxMegapixels, limit int) (done, failed int, err error) {
	if store == nil || store.DB == nil || limit <= 0 {
		return 0, 0, nil
	}

	type item struct {
		id        int64
		websiteID int64
		filename  string
		mimeType  string
	}

	// Read the whole batch before touching a file: the write below takes the
	// database, and holding a read cursor open across it is how a busy
	// SQLite starts returning "database is locked".
	rows, err := store.DB.Read.QueryContext(ctx,
		`SELECT id, website_id, filename, mime_type FROM media
		  WHERE width = 0 ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("list media without dimensions: %w", err)
	}
	var todo []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.websiteID, &it.filename, &it.mimeType); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan media: %w", err)
		}
		// A PDF has no dimensions and never will; leaving it in the query would
		// mean reading the same rows on every pass for ever.
		if CanMakeVariants(it.mimeType) {
			todo = append(todo, it)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, it := range todo {
		if ctx.Err() != nil {
			return done, failed, ctx.Err()
		}
		dir := filepath.Join(dataDir, "media", strconv.FormatInt(it.websiteID, 10))
		source := filepath.Join(dir, it.filename)

		width, height, err := Dimensions(source)
		if err != nil {
			// One unreadable file must not stop the pass. It stays at width 0
			// and is tried again next time, which is right: the usual cause is
			// a file that has not finished being written.
			slog.Warn("media backfill: could not read dimensions",
				"err", err, "media", it.id, "file", it.filename)
			failed++
			continue
		}

		// Variants are best effort, the dimensions are not: an image too large
		// to scale should still get its width and height, because that alone
		// stops the page from jumping.
		variants, verr := MakeVariantsThrottled(source, dir, it.filename, it.mimeType, maxMegapixels)
		if verr != nil {
			slog.Warn("media backfill: could not make variants",
				"err", verr, "media", it.id, "file", it.filename)
			variants = nil
		}
		if err := store.SaveVariants(ctx, it.id, width, height, variants); err != nil {
			slog.Warn("media backfill: could not store dimensions",
				"err", err, "media", it.id)
			failed++
			continue
		}
		done++
	}
	return done, failed, nil
}
