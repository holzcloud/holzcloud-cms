// What this file guards are two sentences that must not stay a habit.
//
// The first: a started staging belongs to exactly one person. Two admins
// importing at the same time hold two tokens and two files, and neither of them
// can see or resume the other's. That stands in the database — user_id with
// ON DELETE CASCADE from users — and is checked here against a second user
// rather than merely believed.
//
// The second: two tabs do not destroy each other. user/token.go:59-62 deletes
// the previous token of the same user, and there that is right. Here that line
// was deliberately not copied, and a line that was not copied is invisible —
// which is why TestTwoTabsSurviveEachOther asserts its absence instead of
// leaving it to the comment.
package csvimport_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/csvimport"
	"github.com/holzcloud/holzcloud-cms/internal/db"
)

// setup opens a migrated database in the test directory and creates a user and
// a website. Staging is the one half of this phase that is allowed a database —
// internal/csv is the other and has none.
func setup(t *testing.T) (*csvimport.Store, *db.DB, int64, int64) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)
	if err := db.RunMigrations(database.Write); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return csvimport.NewStore(database), database, user(t, database, "admin@example.com"), website(t, database, "Test Site")
}

func user(t *testing.T, database *db.DB, email string) int64 {
	t.Helper()
	res, err := database.Write.Exec(
		`INSERT INTO users (email, password, role, created_at) VALUES ($1, 'hash', 'admin', '2026-01-01T00:00:00Z')`,
		email)
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func website(t *testing.T, database *db.DB, name string) int64 {
	t.Helper()
	res, err := database.Write.Exec(
		`INSERT INTO websites (name, description) VALUES ($1, '')`, name)
	if err != nil {
		t.Fatalf("create website %s: %v", name, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func sampleUpload(userID, websiteID int64, data []byte) csvimport.Upload {
	return csvimport.Upload{
		UserID:    userID,
		WebsiteID: websiteID,
		Mode:      "bestehend",
		Collision: "uebergehen",
		Filename:  "products.csv",
		Data:      data,
	}
}

// TestUploadComesBackByteForByte: what was uploaded comes back the way it went
// in.
//
// With the BOM in front that Excel writes, and with a tail of high bytes. A
// column that carried the upload as TEXT would silently change both — which is
// why 00049 holds a BLOB.
func TestUploadComesBackByteForByte(t *testing.T) {
	ctx := context.Background()
	store, _, userID, websiteID := setup(t)

	data := []byte("\xef\xbb\xbfTitle,Text\nChair,\"oak, oiled\"\n")
	for b := 0x80; b <= 0xff; b++ {
		data = append(data, byte(b))
	}

	token, err := store.Stage(ctx, sampleUpload(userID, websiteID, data))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if len(token) != 32 {
		t.Errorf("token is %d characters long, expected 32 — 128 bits as hex", len(token))
	}

	back, err := store.Get(ctx, token, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(back.Data, data) {
		t.Errorf("data comes back changed:\n  in   = %q\n  back = %q", data, back.Data)
	}
	if !bytes.HasPrefix(back.Data, []byte("\xef\xbb\xbf")) {
		t.Error("the BOM was lost on the way — it belongs to the file and is stripped by the reader, not before")
	}
	if back.WebsiteID != websiteID {
		t.Errorf("WebsiteID = %d, expected %d", back.WebsiteID, websiteID)
	}
	if back.Mode != "bestehend" || back.Collision != "uebergehen" {
		t.Errorf("Mode/Collision = %q/%q, expected \"bestehend\"/\"uebergehen\"", back.Mode, back.Collision)
	}
	if back.Filename != "products.csv" {
		t.Errorf("Filename = %q", back.Filename)
	}
	if back.CreatedAt.IsZero() {
		t.Error("CreatedAt is empty — the sweep hangs on that column")
	}
}

// TestForeignTokenIsRefused: the second admin does not get the first one's
// file, even knowing the token. (T-09-06)
func TestForeignTokenIsRefused(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := setup(t)
	stranger := user(t, database, "second@example.com")

	token, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nChair\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	if _, err := store.Get(ctx, token, stranger); !errors.Is(err, csvimport.ErrForeign) {
		t.Fatalf("Get with a foreign token = %v, expected ErrForeign", err)
	}
	// And the owner still gets at it — the refusal of the one must not take the
	// other with it.
	if _, err := store.Get(ctx, token, userID); err != nil {
		t.Errorf("Get by the owner: %v", err)
	}
}

// TestUnknownTokenIsExpired: a token that never existed, or that the sweep has
// taken, is expired and not forbidden. The two cases are distinguishable
// because they describe two different people. (D-33)
func TestUnknownTokenIsExpired(t *testing.T) {
	ctx := context.Background()
	store, _, userID, _ := setup(t)

	_, err := store.Get(ctx, "00000000000000000000000000000000", userID)
	if !errors.Is(err, csvimport.ErrExpired) {
		t.Fatalf("Get with an unknown token = %v, expected ErrExpired", err)
	}
	if errors.Is(err, csvimport.ErrForeign) {
		t.Error("ErrExpired and ErrForeign cannot be told apart — the screen then cannot say which of the two cases applies")
	}
}

// TestTwoTabsSurviveEachOther asserts the absence of user/token.go:59-62.
//
// An admin with two tabs uploads twice. After the second time two rows stand
// there and the first token still fetches the first file. Had the line from
// token.go been copied, the first file would now be gone — and nobody would
// have noticed, because a deleted staging looks like an expired one. (D-33)
func TestTwoTabsSurviveEachOther(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := setup(t)

	firstData := []byte("Title\nFirst tab\n")
	secondData := []byte("Title\nSecond tab\n")

	firstToken, err := store.Stage(ctx, sampleUpload(userID, websiteID, firstData))
	if err != nil {
		t.Fatalf("first Stage: %v", err)
	}
	secondToken, err := store.Stage(ctx, sampleUpload(userID, websiteID, secondData))
	if err != nil {
		t.Fatalf("second Stage: %v", err)
	}
	if firstToken == secondToken {
		t.Fatal("both tokens are the same")
	}

	var rows int
	if err := database.Read.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM csv_imports WHERE user_id = $1`, userID).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 2 {
		t.Fatalf("after two uploads %d rows stand there, expected 2 — the previous staging was deleted", rows)
	}

	first, err := store.Get(ctx, firstToken, userID)
	if err != nil {
		t.Fatalf("the first token fetches nothing any more: %v", err)
	}
	if !bytes.Equal(first.Data, firstData) {
		t.Errorf("the first token fetches %q, expected %q", first.Data, firstData)
	}
	second, err := store.Get(ctx, secondToken, userID)
	if err != nil {
		t.Fatalf("Get of the second token: %v", err)
	}
	if !bytes.Equal(second.Data, secondData) {
		t.Errorf("the second token fetches %q, expected %q", second.Data, secondData)
	}
}

// TestTokenIsNotInTheDatabase: the token itself stands in no column, only its
// SHA-256. A backup copy therefore yields no resumable import. (T-09-09, the
// same discipline as 00012)
func TestTokenIsNotInTheDatabase(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := setup(t)

	token, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nChair\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	var hash, name, filename, mode, collision string
	var data []byte
	if err := database.Read.QueryRowContext(ctx,
		`SELECT token_hash, website_name, dateiname, modus, kollision, daten FROM csv_imports`).
		Scan(&hash, &name, &filename, &mode, &collision, &data); err != nil {
		t.Fatalf("read row: %v", err)
	}
	for column, value := range map[string]string{
		"token_hash":   hash,
		"website_name": name,
		"dateiname":    filename,
		"modus":        mode,
		"kollision":    collision,
		"daten":        string(data),
	} {
		if bytes.Contains([]byte(value), []byte(token)) {
			t.Errorf("the token stands in column %s — only its hash is stored", column)
		}
	}
	if len(hash) != 64 {
		t.Errorf("token_hash is %d characters long, expected 64 — SHA-256 as hex", len(hash))
	}
}

// TestDeleteTakesExactlyOne: after the write run the staging disappears, and a
// reload of the report finds no token any more — instead of importing the file
// a second time. The staging beside it stays where it is.
func TestDeleteTakesExactlyOne(t *testing.T) {
	ctx := context.Background()
	store, _, userID, websiteID := setup(t)

	one, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\none\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	other, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\ntwo\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	u, err := store.Get(ctx, one, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := store.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, one, userID); !errors.Is(err, csvimport.ErrExpired) {
		t.Errorf("after the delete = %v, expected ErrExpired", err)
	}
	if _, err := store.Get(ctx, other, userID); err != nil {
		t.Errorf("the other staging went with it: %v", err)
	}
}

// TestPruneSweepsOnlyTheOld: what is two days old goes; what is an hour old
// stays. The return value is the number of swept rows, so the job in main.go
// has the same shape as PurgeExpiredTokens.
func TestPruneSweepsOnlyTheOld(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := setup(t)

	old, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nold\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	fresh, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nfresh\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// Backdating is possible only here in the test, from outside: staging
	// itself writes its row once and never changes it.
	twoDaysAgo := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02T15:04:05Z")
	oldUpload, err := store.Get(ctx, old, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx,
		`UPDATE csv_imports SET erstellt_am = $1 WHERE id = $2`, twoDaysAgo, oldUpload.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := store.Prune(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("Prune reports %d swept rows, expected 1", n)
	}
	if _, err := store.Get(ctx, old, userID); !errors.Is(err, csvimport.ErrExpired) {
		t.Errorf("the old staging is still there: %v", err)
	}
	if _, err := store.Get(ctx, fresh, userID); err != nil {
		t.Errorf("the fresh staging was swept along: %v", err)
	}
}

// TestDeletingAUserTakesTheUploadWithIt: a deleted account leaves no orphaned
// ten megabytes behind. That is a fact of the database — ON DELETE CASCADE from
// users — and not the habit of a handler. (T-09-12)
func TestDeletingAUserTakesTheUploadWithIt(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := setup(t)

	if _, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nChair\n"))); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var rows int
	if err := database.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM csv_imports`).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("after deleting the account %d stagings stand there, expected 0", rows)
	}
}

// TestDeletingAWebsiteLeavesTheUploadStanding is the counter-check to the
// previous one: the target website disappears, the operator's file does not.
// website_id falls back to NULL and arrives as 0 — exactly the state of an
// import that has yet to create its website. The run then ends with a message
// and not with the upload vanishing under the operator's hands. (D-29)
func TestDeletingAWebsiteLeavesTheUploadStanding(t *testing.T) {
	ctx := context.Background()
	store, database, userID, websiteID := setup(t)

	token, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nChair\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if _, err := database.Write.ExecContext(ctx, `DELETE FROM websites WHERE id = $1`, websiteID); err != nil {
		t.Fatalf("delete website: %v", err)
	}

	back, err := store.Get(ctx, token, userID)
	if err != nil {
		t.Fatalf("the staging disappeared with the website: %v", err)
	}
	if back.WebsiteID != 0 {
		t.Errorf("WebsiteID = %d, expected 0 — the deleted website leaves NULL behind", back.WebsiteID)
	}
	if !bytes.Equal(back.Data, []byte("Title\nChair\n")) {
		t.Errorf("Data = %q", back.Data)
	}
}

// TestUploadWithoutAWebsiteIsAllowed: the import that creates the website in
// the first place has none yet — website_id is deliberately allowed to be
// empty.
func TestUploadWithoutAWebsiteIsAllowed(t *testing.T) {
	ctx := context.Background()
	store, _, userID, _ := setup(t)

	u := sampleUpload(userID, 0, []byte("Title\nChair\n"))
	u.Mode = "neu"
	u.WebsiteName = "New Site"

	token, err := store.Stage(ctx, u)
	if err != nil {
		t.Fatalf("Stage without a website: %v", err)
	}
	back, err := store.Get(ctx, token, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if back.WebsiteID != 0 {
		t.Errorf("WebsiteID = %d, expected 0", back.WebsiteID)
	}
	if back.WebsiteName != "New Site" || back.Mode != "neu" {
		t.Errorf("WebsiteName/Mode = %q/%q", back.WebsiteName, back.Mode)
	}
}

// TestClaimSucceedsExactlyOnce (WR-05): of two callers on one row, one gets it.
//
// This is the whole of what the write screen rests on. The staging row used to
// be deleted AFTER the loop, which closed a refresh — the row is gone by the
// time the report renders — and nothing else. Two requests that overlap both
// passed the staged() lookup, both reached the loop, and on the "new website"
// path both called CreateWebsite: two websites, each carrying the whole file,
// from one double-click on a form htmx never processes.
func TestClaimSucceedsExactlyOnce(t *testing.T) {
	ctx := context.Background()
	store, _, userID, websiteID := setup(t)

	token, err := store.Stage(ctx, sampleUpload(userID, websiteID, []byte("Title\nChair\n")))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	up, err := store.Get(ctx, token, userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	mine, err := store.Claim(ctx, up.ID)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if !mine {
		t.Fatal("the first caller did not get the row")
	}

	again, err := store.Claim(ctx, up.ID)
	if err != nil {
		t.Fatalf("second Claim: %v", err)
	}
	if again {
		t.Error("the second caller got the row as well — both would import the whole file")
	}

	// And the token is gone, so a refresh answers the expiry screen rather
	// than importing a second time.
	if _, err := store.Get(ctx, token, userID); !errors.Is(err, csvimport.ErrExpired) {
		t.Errorf("Get after a claim = %v, want ErrExpired", err)
	}
}
