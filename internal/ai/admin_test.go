package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/mail"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// fakeInstallation stands in for the admin handler. The real one is tested
// through the same tools in internal/admin; here the question is only whether
// the tools are wired, guarded and hand over what they were given.
type fakeInstallation struct {
	calls []string
	logs  []activity.Entry

	host, lang string
	rights     UserRights
	change     UserChange
	archive    []byte
	logo       []byte
	logoName   string
	filter     activity.Filter
}

func (f *fakeInstallation) did(s string) { f.calls = append(f.calls, s) }

func (f *fakeInstallation) OpLog(_ context.Context, actor string, e activity.Entry) {
	e.ActorEmail = actor
	f.logs = append(f.logs, e)
}
func (f *fakeInstallation) OpChanged(int64) {}

func (f *fakeInstallation) OpUsers(context.Context) ([]AdminUser, error) {
	return []AdminUser{{ID: 1, Email: "a@example.org", Role: "admin", AllWebsites: true}}, nil
}
func (f *fakeInstallation) OpUser(_ context.Context, id int64) (*AdminUser, error) {
	if id != 1 && id != 2 {
		return nil, errors.New("there is no such user")
	}
	return &AdminUser{ID: id, Email: "a@example.org", Role: "editor", MayPublish: true, AllWebsites: true}, nil
}
func (f *fakeInstallation) OpInviteUser(_ context.Context, host, lang, name, email, role string, r UserRights) (*AdminUser, AccessLink, error) {
	f.did("invite")
	f.host, f.lang, f.rights = host, lang, r
	return &AdminUser{ID: 2, Name: name, Email: email, Role: role},
		AccessLink{URL: "http://" + host + "/admin/activate/geheim", Expires: time.Now()}, nil
}
func (f *fakeInstallation) OpUpdateUser(_ context.Context, id int64, ch UserChange) (*AdminUser, error) {
	f.did("update")
	f.change = ch
	return &AdminUser{ID: id}, nil
}
func (f *fakeInstallation) OpUserLink(_ context.Context, host, lang string, id int64, purpose string) (AccessLink, error) {
	f.did("link:" + purpose)
	f.host, f.lang = host, lang
	return AccessLink{URL: "http://" + host + "/admin/reset/x"}, nil
}
func (f *fakeInstallation) OpEndUserSessions(context.Context, int64) error {
	f.did("sessions")
	return nil
}
func (f *fakeInstallation) OpDisableUserTwoFactor(context.Context, int64) error {
	f.did("2fa")
	return nil
}
func (f *fakeInstallation) OpDeleteUser(context.Context, int64) error {
	f.did("delete")
	return nil
}

func (f *fakeInstallation) OpPlugins(context.Context) ([]plugin.Status, error) {
	return []plugin.Status{{Installed: plugin.Installed{ID: "suche", Name: "Suche", Enabled: true}, Running: true}}, nil
}
func (f *fakeInstallation) OpPluginInstall(_ context.Context, archive []byte) (*plugin.Manifest, error) {
	f.did("plugin:install")
	f.archive = archive
	return &plugin.Manifest{ID: "suche", Name: "Suche", Version: "1.0"}, nil
}
func (f *fakeInstallation) OpPluginSwitch(_ context.Context, id string, on bool) error {
	if on {
		f.did("plugin:on")
	} else {
		f.did("plugin:off")
	}
	return nil
}
func (f *fakeInstallation) OpPluginWebsites(context.Context, string, []int64) error {
	f.did("plugin:websites")
	return nil
}
func (f *fakeInstallation) OpPluginRemove(context.Context, string) error {
	f.did("plugin:remove")
	return nil
}

func (f *fakeInstallation) OpMailStatus(context.Context) (mail.Status, error) {
	return mail.Status{Configured: true, Host: "smtp.example.org", Failed: 2}, nil
}
func (f *fakeInstallation) OpMailRetry(context.Context) (int64, error) {
	f.did("mail:retry")
	return 2, nil
}
func (f *fakeInstallation) OpMailTest(context.Context, string, string) error {
	f.did("mail:test")
	return nil
}

func (f *fakeInstallation) OpInstallLanguage(_ context.Context, code string, _ []byte) (int, error) {
	f.did("language:" + code)
	return 3, nil
}
func (f *fakeInstallation) OpRemoveLanguage(_ context.Context, code string) error {
	f.did("language-remove:" + code)
	return nil
}

func (f *fakeInstallation) OpSetBrand(context.Context, string, string) error {
	f.did("brand")
	return nil
}
func (f *fakeInstallation) OpSetLogo(_ context.Context, name string, data []byte) error {
	f.did("logo")
	f.logoName, f.logo = name, data
	return nil
}
func (f *fakeInstallation) OpRemoveLogo(context.Context) error {
	f.did("logo:remove")
	return nil
}

func (f *fakeInstallation) OpActivity(_ context.Context, flt activity.Filter, limit, offset int) ([]activity.Entry, int, error) {
	f.filter = flt
	return []activity.Entry{{ID: 1, ActorEmail: "KI: x", Action: "page.publish"}}, 1, nil
}

// installation is a server with the fake behind it and one key of each level.
type installation struct {
	ts                          *httptest.Server
	db                          *db.DB
	fake                        *fakeInstallation
	tokens                      *Store
	websiteID                   int64
	admin, content, read, other string
	adminID                     int64
}

func setUpInstallation(t *testing.T) installation {
	t.Helper()
	database := newTestDB(t)
	domains := domain.NewStore(database)
	ws, err := domains.CreateWebsite(context.Background(), "Testhof", "")
	if err != nil {
		t.Fatal(err)
	}
	tokens := NewStore(database)
	issue := func(name string, level Level, site int64) (string, int64) {
		secret, tok, err := tokens.IssueLevel(context.Background(), name, site, level, 0)
		if err != nil {
			t.Fatalf("IssueLevel %s: %v", name, err)
		}
		return secret, tok.ID
	}
	in := installation{db: database, fake: &fakeInstallation{}, tokens: tokens, websiteID: ws.ID}
	in.admin, in.adminID = issue("verwaltung", LevelAdmin, 0)
	in.content, _ = issue("inhalt", LevelContent, 0)
	in.read, _ = issue("lesen", LevelRead, 0)
	in.other, _ = issue("nur eine", LevelContent, ws.ID)

	srv := NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domains, Pages: page.NewStore(database), Media: media.NewStore(database),
		Ops: in.fake, Tokens: tokens,
	}))
	in.ts = httptest.NewServer(srv)
	t.Cleanup(in.ts.Close)
	return in
}

func toolNames(t *testing.T, ts *httptest.Server, key string) map[string]bool {
	t.Helper()
	res := call(t, ts, key, "tools/list", nil)
	out := map[string]bool{}
	for _, raw := range res["result"].(map[string]any)["tools"].([]any) {
		out[raw.(map[string]any)["name"].(string)] = true
	}
	return out
}

// A content key neither sees nor may call what manages the installation; an
// admin key sees all of it. key_info is the one tool every key has.
func TestOnlyAnAdminKeyManagesTheInstallation(t *testing.T) {
	in := setUpInstallation(t)

	var admin []string
	for _, tool := range adminTools(Deps{}) {
		if tool.Name == "key_info" {
			if tool.Admin || tool.Writes {
				t.Error("key_info must be open to every key")
			}
			continue
		}
		if !tool.Admin {
			t.Errorf("%s manages the installation and is not marked Admin", tool.Name)
		}
		admin = append(admin, tool.Name)
	}

	for _, key := range []string{in.content, in.read, in.other} {
		seen := toolNames(t, in.ts, key)
		if !seen["key_info"] {
			t.Error("key_info is missing for a non-admin key")
		}
		for _, name := range admin {
			if seen[name] {
				t.Errorf("a non-admin key is offered %s", name)
			}
		}
	}
	seen := toolNames(t, in.ts, in.admin)
	for _, name := range admin {
		if !seen[name] {
			t.Errorf("the admin key is not offered %s", name)
		}
	}

	// Called anyway: refused, and the handler never reached.
	for _, name := range []string{"list_users", "invite_user", "create_ai_key", "delete_user", "read_activity_log"} {
		res, failed := callTool(t, in.ts, in.content, name, map[string]any{
			"email": "x@example.org", "name": "x", "id": 1, "confirm": true,
		})
		if !failed {
			t.Errorf("a content key called %s", name)
			continue
		}
		if name != "invite_user" && !strings.Contains(res["text"].(string), "admin key") {
			t.Errorf("%s: refusal does not say why: %v", name, res["text"])
		}
	}
	if len(in.fake.calls) != 0 {
		t.Errorf("the handler was reached: %v", in.fake.calls)
	}
}

// create_ai_key never makes an admin key, however it is asked.
func TestCreateAIKeyRefusesAdmin(t *testing.T) {
	in := setUpInstallation(t)

	for _, level := range []string{"admin", "ADMIN", " admin "} {
		res, failed := callTool(t, in.ts, in.admin, "create_ai_key", map[string]any{
			"name": "neu", "level": level,
		})
		if !failed {
			t.Fatalf("level %q: an admin key was created through MCP", level)
		}
		if !strings.Contains(res["text"].(string), "holzcloud ai key create") {
			t.Errorf("the refusal does not say where an admin key comes from: %v", res["text"])
		}
	}
	if _, failed := callTool(t, in.ts, in.admin, "create_ai_key", map[string]any{
		"name": "neu", "level": "root",
	}); !failed {
		t.Error("an unknown level was accepted")
	}
	keys, _ := in.tokens.List(context.Background())
	for _, k := range keys {
		if k.Admin && k.ID != in.adminID {
			t.Errorf("an admin key %q exists that the test did not make", k.Name)
		}
	}
	if _, err := creatableLevel("admin"); !errors.Is(err, ErrNoAdminKeyHere) {
		t.Errorf("creatableLevel(admin) = %v", err)
	}
}

// Read and content keys are made, with the secret shown once and working.
func TestCreateAIKeyIssuesReadAndContentKeys(t *testing.T) {
	in := setUpInstallation(t)

	res, failed := callTool(t, in.ts, in.admin, "create_ai_key", map[string]any{
		"name": "Redaktion", "level": "read", "website": in.websiteID, "days": 7,
	})
	if failed {
		t.Fatalf("create_ai_key: %v", res["text"])
	}
	if res["level"] != "read" || res["expires_at"] == nil {
		t.Errorf("key = %v", res)
	}
	scope, err := in.tokens.Verify(context.Background(), res["secret"].(string))
	if err != nil {
		t.Fatalf("the new secret does not work: %v", err)
	}
	if scope.CanWrite || scope.Admin || scope.WebsiteID != in.websiteID {
		t.Errorf("scope = %+v", scope)
	}

	res, failed = callTool(t, in.ts, in.admin, "create_ai_key", map[string]any{"name": "Schreiber"})
	if failed || res["level"] != "content" {
		t.Fatalf("default level: %v", res)
	}

	if _, failed := callTool(t, in.ts, in.admin, "create_ai_key", map[string]any{
		"name": "x", "website": 999,
	}); !failed {
		t.Error("a key for a website that does not exist was made")
	}
	if _, failed := callTool(t, in.ts, in.admin, "create_ai_key", map[string]any{"name": " "}); !failed {
		t.Error("a key without a name was made")
	}

	list, _ := callTool(t, in.ts, in.admin, "list_ai_keys", nil)
	own := 0
	for _, raw := range list["keys"].([]any) {
		k := raw.(map[string]any)
		if _, leaked := k["secret"]; leaked {
			t.Error("list_ai_keys shows a secret")
		}
		if k["this_key"] == true {
			own++
		}
	}
	if own != 1 {
		t.Errorf("%d keys marked as this key, want 1", own)
	}
	if n := len(in.fake.logs); n != 2 || in.fake.logs[0].Action != activity.ActionAIKeyCreate ||
		in.fake.logs[0].ActorEmail != "KI: verwaltung" {
		t.Errorf("activity = %+v", in.fake.logs)
	}
}

// Revoking needs confirmation and never cuts the calling key off.
func TestRevokeAIKey(t *testing.T) {
	in := setUpInstallation(t)
	_, victim, _ := in.tokens.Issue(context.Background(), "alt", 0, true, 0)

	if _, failed := callTool(t, in.ts, in.admin, "revoke_ai_key", map[string]any{"id": victim.ID}); !failed {
		t.Error("revoked without confirm")
	}
	if res, failed := callTool(t, in.ts, in.admin, "revoke_ai_key", map[string]any{
		"id": in.adminID, "confirm": true,
	}); !failed || !strings.Contains(res["text"].(string), "itself") {
		t.Errorf("the key revoked itself: %v", res)
	}
	if _, failed := callTool(t, in.ts, in.admin, "revoke_ai_key", map[string]any{
		"id": 9999, "confirm": true,
	}); !failed {
		t.Error("revoking a key that does not exist succeeded")
	}
	if res, failed := callTool(t, in.ts, in.admin, "revoke_ai_key", map[string]any{
		"id": victim.ID, "confirm": true,
	}); failed {
		t.Fatalf("revoke: %v", res["text"])
	}
	if _, err := in.tokens.Get(context.Background(), victim.ID); err == nil {
		t.Error("the key is still there")
	}
	if _, failed := callTool(t, in.ts, in.admin, "key_info", nil); failed {
		t.Error("the admin key stopped working")
	}
}

// key_info tells a key what it is.
func TestKeyInfo(t *testing.T) {
	in := setUpInstallation(t)

	res, failed := callTool(t, in.ts, in.other, "key_info", nil)
	if failed {
		t.Fatalf("key_info: %v", res["text"])
	}
	if res["level"] != "content" || res["name"] != "nur eine" ||
		int64(res["website"].(float64)) != in.websiteID || res["website_name"] != "Testhof" {
		t.Errorf("key_info = %v", res)
	}
	if !strings.Contains(strings.Join(anyStrings(res["may"]), " "), "admin key") {
		t.Errorf("a content key is not told how to manage the installation: %v", res["may"])
	}

	res, _ = callTool(t, in.ts, in.read, "key_info", nil)
	if res["level"] != "read" || res["website_name"] != "every website" {
		t.Errorf("read key_info = %v", res)
	}
	res, _ = callTool(t, in.ts, in.admin, "key_info", nil)
	if res["level"] != "admin" {
		t.Errorf("admin key_info = %v", res)
	}
}

func anyStrings(v any) []string {
	var out []string
	list, _ := v.([]any)
	for _, s := range list {
		out = append(out, s.(string))
	}
	return out
}

// Every write reaches the handler with what it was given, is logged under the
// key's name, and a destructive one waits for confirm.
func TestInstallationToolsReachTheHandler(t *testing.T) {
	in := setUpInstallation(t)
	ok := func(name string, args map[string]any) map[string]any {
		t.Helper()
		res, failed := callTool(t, in.ts, in.admin, name, args)
		if failed {
			t.Fatalf("%s: %v", name, res["text"])
		}
		return res
	}

	res := ok("invite_user", map[string]any{"email": "neu@example.org", "websites": []int64{in.websiteID},
		"may_publish": false, "language": "de"})
	inv := res["invitation"].(map[string]any)
	if !strings.Contains(inv["url"].(string), "/admin/activate/") {
		t.Errorf("invitation = %v", inv)
	}
	if in.fake.lang != "de" || in.fake.host == "" {
		t.Errorf("host %q, language %q", in.fake.host, in.fake.lang)
	}
	if in.fake.rights.AllWebsites || in.fake.rights.MayPublish || len(in.fake.rights.Websites) != 1 {
		t.Errorf("rights = %+v", in.fake.rights)
	}
	if _, failed := callTool(t, in.ts, in.admin, "invite_user", map[string]any{
		"email": "x@example.org", "language": "xx",
	}); !failed {
		t.Error("a language the admin cannot speak was accepted")
	}

	ok("update_user", map[string]any{"id": 2, "may_publish": false})
	if in.fake.change.MayPublish == nil || *in.fake.change.MayPublish || in.fake.change.Rights != nil {
		t.Errorf("change = %+v", in.fake.change)
	}
	ok("update_user", map[string]any{"id": 2, "all_websites": false, "websites": []int64{}})
	if r := in.fake.change.Rights; r == nil || r.AllWebsites || len(r.Websites) != 0 || !r.MayPublish {
		t.Errorf("rights change = %+v", in.fake.change.Rights)
	}

	ok("create_password_reset_link", map[string]any{"id": 2})
	ok("create_invitation_link", map[string]any{"id": 2})
	ok("end_user_sessions", map[string]any{"id": 2})

	for _, name := range []string{"disable_user_two_factor", "delete_user"} {
		if res, failed := callTool(t, in.ts, in.admin, name, map[string]any{"id": 2}); !failed ||
			!strings.Contains(res["text"].(string), "confirm") {
			t.Errorf("%s ran without confirm", name)
		}
		ok(name, map[string]any{"id": 2, "confirm": true})
	}

	zip := base64.StdEncoding.EncodeToString([]byte("PK\x03\x04archiv"))
	ok("install_plugin", map[string]any{"archive_base64": "data:application/zip;base64," + zip})
	if string(in.fake.archive) != "PK\x03\x04archiv" {
		t.Errorf("archive = %q", in.fake.archive)
	}
	ok("enable_plugin", map[string]any{"id": "suche"})
	ok("disable_plugin", map[string]any{"id": "suche"})
	ok("set_plugin_websites", map[string]any{"id": "suche", "websites": []int64{in.websiteID}})
	if _, failed := callTool(t, in.ts, in.admin, "remove_plugin", map[string]any{"id": "suche"}); !failed {
		t.Error("remove_plugin ran without confirm")
	}
	ok("remove_plugin", map[string]any{"id": "suche", "confirm": true})

	ok("retry_mail", nil)
	ok("send_test_mail", map[string]any{"to": "a@example.org"})
	ok("install_language", map[string]any{"code": "NL", "json": "{}"})
	if _, failed := callTool(t, in.ts, in.admin, "remove_language", map[string]any{"code": "nl"}); !failed {
		t.Error("remove_language ran without confirm")
	}
	ok("remove_language", map[string]any{"code": "nl", "confirm": true})

	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nbild"))
	ok("update_branding", map[string]any{"name": "Hof", "logo_base64": png, "logo_filename": "logo.png"})
	if in.fake.logoName != "logo.png" || !strings.HasPrefix(string(in.fake.logo), "\x89PNG") {
		t.Errorf("logo %q %q", in.fake.logoName, in.fake.logo)
	}
	if _, failed := callTool(t, in.ts, in.admin, "update_branding", map[string]any{"logo_base64": png}); !failed {
		t.Error("a logo without a file name was accepted")
	}
	if _, failed := callTool(t, in.ts, in.admin, "update_branding", map[string]any{}); !failed {
		t.Error("an empty change was accepted")
	}

	want := []string{"invite", "update", "update", "link:reset", "link:invite", "sessions", "2fa",
		"delete", "plugin:install", "plugin:on", "plugin:off", "plugin:websites", "plugin:remove",
		"mail:retry", "mail:test", "language:nl", "language-remove:nl", "brand", "logo"}
	if strings.Join(in.fake.calls, ",") != strings.Join(want, ",") {
		t.Errorf("calls\n got %v\nwant %v", in.fake.calls, want)
	}
	// One entry per tool call: update_branding changed name and logo in one.
	if len(in.fake.logs) != len(want)-1 {
		t.Errorf("%d activity entries for %d changes", len(in.fake.logs), len(want)-1)
	}
	for _, e := range in.fake.logs {
		if e.ActorEmail != "KI: verwaltung" {
			t.Errorf("entry %s logged as %q", e.Action, e.ActorEmail)
		}
	}
}

// The reading tools hand the filters over as the log screen reads them.
func TestReadActivityLogFilters(t *testing.T) {
	in := setUpInstallation(t)
	res, failed := callTool(t, in.ts, in.admin, "read_activity_log", map[string]any{
		"website": in.websiteID, "action": "page.*", "from": "2026-01-01", "to": "2026-01-31",
	})
	if failed {
		t.Fatalf("read_activity_log: %v", res["text"])
	}
	f := in.fake.filter
	if f.WebsiteID == nil || *f.WebsiteID != in.websiteID || f.Action != "page.*" ||
		f.From == nil || f.To == nil || f.To.Day() != 31 || f.To.Hour() != 23 {
		t.Errorf("filter = %+v", f)
	}
	if _, failed := callTool(t, in.ts, in.admin, "read_activity_log", map[string]any{"from": "gestern"}); !failed {
		t.Error("a date that is none was accepted")
	}
	for _, name := range []string{"list_users", "list_plugins", "get_mail_status", "list_languages", "get_branding"} {
		if res, failed := callTool(t, in.ts, in.admin, name, nil); failed {
			t.Errorf("%s: %v", name, res["text"])
		}
	}
}

// Without the admin handler the tools say so rather than fail strangely.
func TestInstallationToolsWithoutTheHandler(t *testing.T) {
	database := newTestDB(t)
	tokens := NewStore(database)
	secret, _, _ := tokens.IssueLevel(context.Background(), "admin", 0, LevelAdmin, 0)
	ts := httptest.NewServer(NewServer(tokens, "Test", slog.New(slog.DiscardHandler), Tools(Deps{
		Domains: domain.NewStore(database), Pages: page.NewStore(database), Media: media.NewStore(database),
	})))
	defer ts.Close()

	for _, name := range []string{"list_users", "list_plugins", "get_mail_status", "read_activity_log", "list_ai_keys"} {
		res, failed := callTool(t, ts, secret, name, nil)
		if !failed || !strings.Contains(res["text"].(string), "not available") {
			t.Errorf("%s: %v", name, res)
		}
	}
}
