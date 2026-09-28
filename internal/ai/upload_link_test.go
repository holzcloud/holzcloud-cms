package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// Upload links, at every boundary the handler draws: who may open one, what
// the answer says, that it works once and for a while, that the key is asked
// again when the file arrives, and that a file too large never reaches the
// intake. The intake itself is a stand-in here; the router test in
// cmd/holzcloud drives the admin's real one.

// uploadingOps is the admin handler as far as an upload needs it. It records
// every file it receives, so a test can say that a refused request never got
// that far. fakeMediaOps stays without an upload on purpose: the test for a
// build without one depends on it.
type uploadingOps struct {
	*fakeMediaOps
	mu       sync.Mutex
	received [][]byte
}

func (u *uploadingOps) OpUploadMedia(ctx context.Context, websiteID int64, name string, content []byte) (*media.Media, bool, error, error) {
	u.mu.Lock()
	u.received = append(u.received, append([]byte(nil), content...))
	u.mu.Unlock()

	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	m, err := u.media.Create(ctx, websiteID, hash+filepath.Ext(name), name, "image/png", int64(len(content)), hash)
	if err != nil {
		return nil, false, nil, err
	}
	return m, false, nil, nil
}

func (u *uploadingOps) OpCropMedia(context.Context, int64, int64, media.Crop) (*media.Media, error) {
	return nil, errors.New("cropping is not part of this test")
}

func (u *uploadingOps) OpRestoreMedia(context.Context, int64, int64) (*media.Media, error) {
	return nil, errors.New("restoring is not part of this test")
}

// files hands back what arrived so far.
func (u *uploadingOps) files() [][]byte {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([][]byte(nil), u.received...)
}

type uploadFixture struct {
	ts                 *httptest.Server
	database           *db.DB
	tokens             *Store
	uploads            *Uploads
	ops                *uploadingOps
	eins, zwei         int64
	write, read, fremd string
}

// uploadLimit is the fixture's media and video limit.
const uploadLimit = 1 << 10

func setUpUploadLinks(t *testing.T, secure bool) uploadFixture {
	t.Helper()
	database := newTestDB(t)
	ctx := context.Background()
	domains := domain.NewStore(database)
	eins, _ := domains.CreateWebsite(ctx, "Eins", "")
	zwei, _ := domains.CreateWebsite(ctx, "Zwei", "")

	tokens := NewStore(database)
	write, _, err := tokens.Issue(ctx, "schreibend", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	read, _, _ := tokens.Issue(ctx, "lesend", 0, false, 0)
	fremd, _, _ := tokens.Issue(ctx, "nur zwei", zwei.ID, true, 0)

	mediaStore := media.NewStore(database)
	ops := &uploadingOps{fakeMediaOps: &fakeMediaOps{media: mediaStore, albums: album.NewStore(database)}}
	uploads := NewUploads(secure)
	deps := Deps{
		Domains: domains, Pages: page.NewStore(database), Media: mediaStore,
		Ops: ops,
		Limits: Limits{DataDir: t.TempDir(), MaxMediaSize: uploadLimit, MaxVideoSize: uploadLimit,
			MaxMegapixels: 24},
		Tokens:  tokens,
		Uploads: uploads,
	}
	discard := slog.New(slog.DiscardHandler)
	mux := http.NewServeMux()
	mux.Handle(UploadPath, UploadHandler(deps, discard))
	mux.Handle("/", NewServer(tokens, "Test", discard, Tools(deps)))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return uploadFixture{
		ts: ts, database: database, tokens: tokens, uploads: uploads, ops: ops,
		eins: eins.ID, zwei: zwei.ID, write: write, read: read, fremd: fremd,
	}
}

// openLink asks for an upload link and hands back its address.
func openLink(t *testing.T, f uploadFixture, key string, args map[string]any) string {
	t.Helper()
	out, failed := callTool(t, f.ts, key, "create_upload_link", args)
	if failed {
		t.Fatalf("create_upload_link: %v", out["text"])
	}
	link, _ := out["upload_url"].(string)
	if link == "" {
		t.Fatalf("create_upload_link without upload_url: %v", out)
	}
	return link
}

// send delivers a body to an upload link and reads the JSON answer.
func send(t *testing.T, f uploadFixture, method, link string, body io.Reader) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, link, body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, link, err)
	}
	defer res.Body.Close()
	var out map[string]any
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	return res, out
}

// setClock moves the links' clock, under the lock the handler takes.
func setClock(u *Uploads, now func() time.Time) {
	u.mu.Lock()
	u.now = now
	u.mu.Unlock()
}

func hofArgs(f uploadFixture) map[string]any {
	return map[string]any{"website": f.eins, "file_name": "hof.png", "alt_text": "Der Hof"}
}

func TestAnUploadLinkSaysWhereAndHow(t *testing.T) {
	f := setUpUploadLinks(t, false)
	host := strings.TrimPrefix(f.ts.URL, "http://")

	out, failed := callTool(t, f.ts, f.write, "create_upload_link", hofArgs(f))
	if failed {
		t.Fatalf("refused: %v", out["text"])
	}
	link, _ := out["upload_url"].(string)
	prefix := "http://" + host + UploadPath
	if !strings.HasPrefix(link, prefix) {
		t.Fatalf("upload_url = %q, want it under %q", link, prefix)
	}
	if secret := strings.TrimPrefix(link, prefix); len(secret) != 43 {
		t.Errorf("secret %q has %d characters, want 43", secret, len(secret))
	}
	if out["method"] != "PUT" {
		t.Errorf("method = %v", out["method"])
	}
	expires, err := time.Parse(time.RFC3339, out["expires_at"].(string))
	if err != nil {
		t.Fatalf("expires_at: %v", err)
	}
	if d := time.Until(expires); d <= 0 || d > UploadLinkLifetime+time.Minute {
		t.Errorf("expires_at is %v away, want within %v", d, UploadLinkLifetime)
	}
	if n, _ := out["max_bytes"].(float64); n != uploadLimit {
		t.Errorf("max_bytes = %v, want %d", out["max_bytes"], uploadLimit)
	}
	if want := "curl --fail --upload-file 'hof.png' '" + link + "'"; out["example"] != want {
		t.Errorf("example = %q, want %q", out["example"], want)
	}

	secure := setUpUploadLinks(t, true)
	link = openLink(t, secure, secure.write, hofArgs(secure))
	if want := "https://" + strings.TrimPrefix(secure.ts.URL, "http://") + UploadPath; !strings.HasPrefix(link, want) {
		t.Errorf("secure upload_url = %q, want it under %q", link, want)
	}
}

func TestOnlyAWritingKeySeesTheUploadLinkTool(t *testing.T) {
	f := setUpUploadLinks(t, false)
	listed := func(key string) bool {
		res := call(t, f.ts, key, "tools/list", nil)
		tools, _ := res["result"].(map[string]any)["tools"].([]any)
		for _, tool := range tools {
			if tool.(map[string]any)["name"] == "create_upload_link" {
				return true
			}
		}
		return false
	}
	if !listed(f.write) {
		t.Error("the writing key does not see create_upload_link")
	}
	if listed(f.read) {
		t.Error("the reading key sees create_upload_link")
	}
}

func TestAnUploadLinkStoresTheFile(t *testing.T) {
	f := setUpUploadLinks(t, false)
	link := openLink(t, f, f.write, hofArgs(f))

	body := []byte("die Bytes des Hofs")
	res, out := send(t, f, http.MethodPut, link, bytes.NewReader(body))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("PUT: %d %v", res.StatusCode, out)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if out["description"] != "Der Hof" {
		t.Errorf("description = %v", out["description"])
	}
	got := f.ops.files()
	if len(got) != 1 || !bytes.Equal(got[0], body) {
		t.Fatalf("received %q, want exactly %q", got, body)
	}
	id, _ := out["id"].(float64)
	m, err := f.ops.media.GetByID(context.Background(), int64(id))
	if err != nil || m == nil {
		t.Fatalf("media %v: %v", out["id"], err)
	}
	if m.AltText != "Der Hof" {
		t.Errorf("stored alt text %q", m.AltText)
	}
	var uploads []activity.Entry
	for _, e := range f.ops.logged {
		if e.Action == activity.ActionMediaUpload {
			uploads = append(uploads, e)
		}
	}
	if len(uploads) != 1 || uploads[0].ActorEmail != "KI: schreibend" {
		t.Errorf("activity: %+v, want one media upload by \"KI: schreibend\"", uploads)
	}

	// POST is taken the same way.
	link = openLink(t, f, f.write, map[string]any{"website": f.eins, "file_name": "scheune.png"})
	res, out = send(t, f, http.MethodPost, link, strings.NewReader("die Bytes der Scheune"))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST: %d %v", res.StatusCode, out)
	}
}

func TestAnUploadLinkWorksOnce(t *testing.T) {
	f := setUpUploadLinks(t, false)
	link := openLink(t, f, f.write, hofArgs(f))

	if res, out := send(t, f, http.MethodPut, link, strings.NewReader("einmal")); res.StatusCode != http.StatusCreated {
		t.Fatalf("first PUT: %d %v", res.StatusCode, out)
	}
	res, _ := send(t, f, http.MethodPut, link, strings.NewReader("zweimal"))
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("second PUT: %d, want 404", res.StatusCode)
	}
	if n := len(f.ops.files()); n != 1 {
		t.Errorf("%d files received, want 1", n)
	}
}

func TestAnUploadLinkExpires(t *testing.T) {
	f := setUpUploadLinks(t, false)
	link := openLink(t, f, f.write, hofArgs(f))

	later := time.Now().Add(UploadLinkLifetime + time.Second)
	setClock(f.uploads, func() time.Time { return later })

	res, _ := send(t, f, http.MethodPut, link, strings.NewReader("zu spaet"))
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("PUT after expiry: %d, want 404", res.StatusCode)
	}
	if n := len(f.ops.files()); n != 0 {
		t.Errorf("%d files received, want none", n)
	}
}

func TestAnUploadLinkIsOnlyForAWritingKeyOfTheWebsite(t *testing.T) {
	f := setUpUploadLinks(t, false)

	out, failed := callTool(t, f.ts, f.fremd, "create_upload_link", hofArgs(f))
	if !failed || !strings.Contains(out["text"].(string), "another website") {
		t.Errorf("key for website two: %v, want a refusal naming another website", out)
	}
	out, failed = callTool(t, f.ts, f.read, "create_upload_link", hofArgs(f))
	if !failed || !strings.Contains(out["text"].(string), "may only read") {
		t.Errorf("reading key: %v, want a refusal saying it may only read", out)
	}
}

func TestAnUploadLinkDiesWithItsKey(t *testing.T) {
	f := setUpUploadLinks(t, false)
	ctx := context.Background()

	// Revoked after the link was opened.
	key, tok, err := f.tokens.Issue(ctx, "widerrufen", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	link := openLink(t, f, key, hofArgs(f))
	if err := f.tokens.Revoke(ctx, tok.ID); err != nil {
		t.Fatal(err)
	}
	res, out := send(t, f, http.MethodPut, link, strings.NewReader("widerrufen"))
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("revoked key: %d, want 403", res.StatusCode)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "the access key is wrong") {
		t.Errorf("revoked key says %q", msg)
	}

	// Made read-only after the link was opened.
	key, tok, err = f.tokens.Issue(ctx, "herabgestuft", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	link = openLink(t, f, key, hofArgs(f))
	if _, err := f.database.Write.ExecContext(ctx,
		`UPDATE ai_tokens SET can_write = 0 WHERE id = $1`, tok.ID); err != nil {
		t.Fatal(err)
	}
	res, out = send(t, f, http.MethodPut, link, strings.NewReader("herabgestuft"))
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("read-only key: %d, want 403", res.StatusCode)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "may only read") {
		t.Errorf("read-only key says %q", msg)
	}

	if n := len(f.ops.files()); n != 0 {
		t.Errorf("%d files received, want none", n)
	}
}

func TestAnUploadLinkRefusesAFileTooLarge(t *testing.T) {
	f := setUpUploadLinks(t, false)
	big := bytes.Repeat([]byte("x"), uploadLimit+1)

	// With a Content-Length, which is refused before the body is read.
	link := openLink(t, f, f.write, hofArgs(f))
	res, _ := send(t, f, http.MethodPut, link, bytes.NewReader(big))
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("with Content-Length: %d, want 413", res.StatusCode)
	}

	// Without one: net/http cannot size a plain reader and sends it chunked.
	link = openLink(t, f, f.write, hofArgs(f))
	res, _ = send(t, f, http.MethodPut, link, struct{ io.Reader }{bytes.NewReader(big)})
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("chunked: %d, want 413", res.StatusCode)
	}

	if n := len(f.ops.files()); n != 0 {
		t.Errorf("%d files received, want none", n)
	}
}

func TestAWrongMethodLeavesTheUploadLinkOpen(t *testing.T) {
	f := setUpUploadLinks(t, false)
	link := openLink(t, f, f.write, hofArgs(f))

	res, _ := send(t, f, http.MethodGet, link, nil)
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d, want 405", res.StatusCode)
	}
	if allow := res.Header.Get("Allow"); allow != "PUT, POST" {
		t.Errorf("Allow = %q", allow)
	}
	if res, out := send(t, f, http.MethodPut, link, strings.NewReader("nach dem GET")); res.StatusCode != http.StatusCreated {
		t.Errorf("PUT after GET: %d %v, want 201", res.StatusCode, out)
	}
}

func TestAnUnknownUploadLinkIsNotFound(t *testing.T) {
	f := setUpUploadLinks(t, false)
	res, out := send(t, f, http.MethodPut, f.ts.URL+UploadPath+strings.Repeat("A", 43), strings.NewReader("x"))
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown link: %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if msg, _ := out["error"].(string); msg == "" {
		t.Errorf("no error in the answer: %v", out)
	}
}

func TestUploadLinksAreBoundedAndStoredByHash(t *testing.T) {
	u := NewUploads(false)
	start := time.Now()
	setClock(u, func() time.Time { return start })

	var secrets []string
	for i := 0; i < maxOpenUploadLinks; i++ {
		secret, _, err := u.issue(uploadTicket{name: "a.png"})
		if err != nil {
			t.Fatalf("link %d: %v", i+1, err)
		}
		secrets = append(secrets, secret)
	}
	if _, _, err := u.issue(uploadTicket{name: "a.png"}); !errors.Is(err, errTooManyUploadLinks) {
		t.Errorf("link %d: %v, want errTooManyUploadLinks", maxOpenUploadLinks+1, err)
	}

	u.mu.Lock()
	for _, s := range secrets {
		if _, plain := u.tickets[s]; plain {
			t.Errorf("secret %q is a key of the map", s)
		}
		if _, hashed := u.tickets[secretHash(s)]; !hashed {
			t.Errorf("secret %q is not stored by its hash", s)
		}
	}
	u.mu.Unlock()

	// Once they have expired, the open ones are swept and there is room again.
	setClock(u, func() time.Time { return start.Add(UploadLinkLifetime + time.Second) })
	if _, _, err := u.issue(uploadTicket{name: "a.png"}); err != nil {
		t.Errorf("after expiry: %v", err)
	}
	u.mu.Lock()
	if n := len(u.tickets); n != 1 {
		t.Errorf("%d links open after the sweep, want 1", n)
	}
	u.mu.Unlock()
}

func TestAnUploadLinkWithoutUploadsSaysSo(t *testing.T) {
	f := setUpMedia(t, nil)
	text := refusedTool(t, f, f.write, "create_upload_link",
		map[string]any{"website": f.eins, "file_name": "hof.png"})
	if !strings.Contains(text, "not available") {
		t.Errorf("refusal %q, want it to say uploading is not available", text)
	}
}
