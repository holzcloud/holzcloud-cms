package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/branding"
	"github.com/holzcloud/holzcloud-cms/internal/field"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// The installation tools of internal/ai, run against the real handler. The ai
// package tests the guards with a stand-in; this is where the Op methods meet
// the database, the one-time links, the brand and the language folder.

type installationRig struct {
	h         *Handler
	ts        *httptest.Server
	key       string
	websiteID int64
	adminID   int64
}

func newInstallationRig(t *testing.T) installationRig {
	t.Helper()
	h, _, database, ws := newTestAdmin(t)
	h.SetActivityStore(activity.NewStore(database))

	adminID, err := h.users.Create(context.Background(), "Chefin", "chefin@example.org", "einlangespasswort", user.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	tokens := ai.NewStore(database)
	secret, _, err := tokens.IssueLevel(context.Background(), "verwaltung", 0, ai.LevelAdmin, 0)
	if err != nil {
		t.Fatal(err)
	}
	srv := ai.NewServer(tokens, "Test", slog.New(slog.DiscardHandler), ai.Tools(ai.Deps{
		Domains: h.domains, Pages: page.NewStore(database), Media: media.NewStore(database),
		Fields: field.NewStore(database), Ops: h, Tokens: tokens,
	}))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return installationRig{h: h, ts: ts, key: secret, websiteID: ws.ID, adminID: adminID}
}

// tool calls one tool and hands back its result, or the refusal as "text".
func (rig installationRig) tool(t *testing.T, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args}})
	req, _ := http.NewRequest(http.MethodPost, rig.ts.URL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+rig.key)
	res, err := rig.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || len(out.Result.Content) == 0 {
		t.Fatalf("%s: no answer (%v)", name, err)
	}
	text := out.Result.Content[0].Text
	if out.Result.IsError {
		return map[string]any{"text": text}, true
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("%s: %q is not JSON", name, text)
	}
	return m, false
}

func (rig installationRig) must(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	res, failed := rig.tool(t, name, args)
	if failed {
		t.Fatalf("%s: %v", name, res["text"])
	}
	return res
}

func (rig installationRig) refused(t *testing.T, name string, args map[string]any, want string) {
	t.Helper()
	res, failed := rig.tool(t, name, args)
	if !failed {
		t.Errorf("%s was not refused: %v", name, res)
		return
	}
	if !strings.Contains(res["text"].(string), want) {
		t.Errorf("%s: refusal %q does not say %q", name, res["text"], want)
	}
}

// secretOf is the last path segment of a one-time link.
func secretOf(link string) string { return link[strings.LastIndex(link, "/")+1:] }

// An account is invited, limited, reset, signed out and deleted through the
// tools, with the same guards and the same links as on the screens.
func TestUserToolsThroughTheHandler(t *testing.T) {
	rig := newInstallationRig(t)
	ctx := context.Background()
	users := rig.h.users

	res := rig.must(t, "invite_user", map[string]any{
		"email": "neu@example.org", "name": "Neu", "websites": []int64{rig.websiteID}, "may_publish": false,
	})
	invited := res["user"].(map[string]any)
	id := int64(invited["id"].(float64))
	link := res["invitation"].(map[string]any)["url"].(string)
	host := strings.TrimPrefix(rig.ts.URL, "http://")
	if !strings.HasPrefix(link, "http://"+host+"/admin/activate/") {
		t.Errorf("invitation link = %q", link)
	}
	if u, err := users.RedeemToken(ctx, secretOf(link), user.PurposeInvite); err != nil || u.ID != id {
		t.Errorf("the invitation does not open the account: %v", err)
	}
	rights, _ := users.Rights(ctx, id)
	if rights.MayPublish || !rights.Limited() || len(rights.Websites) != 1 || rights.Websites[0] != rig.websiteID {
		t.Errorf("rights after invite = %+v", rights)
	}
	if invited["role"] != "editor" || invited["all_websites"] != false {
		t.Errorf("invited = %v", invited)
	}
	rig.refused(t, "invite_user", map[string]any{"email": "NEU@example.org"}, "already exists")
	rig.refused(t, "invite_user", map[string]any{"email": "x@example.org", "all_websites": false,
		"websites": []int64{999}}, "no website 999")

	// Only the publishing right changes; the website stays.
	rig.must(t, "update_user", map[string]any{"id": id, "may_publish": true, "name": "Neu Name"})
	rights, _ = users.Rights(ctx, id)
	if !rights.MayPublish || len(rights.Websites) != 1 {
		t.Errorf("rights after may_publish = %+v", rights)
	}
	// Limited to none, then every website again.
	rig.must(t, "update_user", map[string]any{"id": id, "all_websites": false, "websites": []int64{}})
	if rights, _ = users.Rights(ctx, id); !rights.Limited() || len(rights.Websites) != 0 {
		t.Errorf("limited to none = %+v", rights)
	}
	got := rig.must(t, "update_user", map[string]any{"id": id, "all_websites": true})
	if got["all_websites"] != true || got["name"] != "Neu Name" {
		t.Errorf("after all_websites = %v", got)
	}

	// The last administrator keeps the role and the account.
	rig.refused(t, "update_user", map[string]any{"id": rig.adminID, "role": "editor"}, "last administrator")
	rig.refused(t, "delete_user", map[string]any{"id": rig.adminID, "confirm": true}, "last administrator")

	reset := rig.must(t, "create_password_reset_link", map[string]any{"id": id})
	if u, err := users.RedeemToken(ctx, secretOf(reset["url"].(string)), user.PurposeReset); err != nil || u.ID != id {
		t.Errorf("the reset link does not work: %v", err)
	}
	rig.refused(t, "create_password_reset_link", map[string]any{"id": 999}, "no such user")

	rig.must(t, "end_user_sessions", map[string]any{"id": id})
	rig.refused(t, "end_user_sessions", map[string]any{"id": 999}, "no such user")

	// A second factor, set by hand, and taken away again.
	if _, err := rig.h.db.Write.Exec(`UPDATE users SET totp_secret = 'ABC',
		totp_confirmed_at = '2026-01-01T00:00:00Z' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if u := rig.must(t, "get_user", map[string]any{"id": id}); u["two_factor"] != true {
		t.Errorf("two_factor not reported: %v", u)
	}
	rig.must(t, "disable_user_two_factor", map[string]any{"id": id, "confirm": true})
	if tf, _ := users.GetTwoFactor(ctx, id); tf.Enabled() {
		t.Error("the second factor is still on")
	}

	list := rig.must(t, "list_users", nil)
	if n := len(list["users"].([]any)); n != 2 {
		t.Errorf("%d users listed, want 2", n)
	}

	rig.must(t, "delete_user", map[string]any{"id": id, "confirm": true})
	if u, _ := users.GetByID(ctx, id); u != nil {
		t.Error("the account is still there")
	}

	// Everything above is in the log, under the key's name.
	logged := rig.must(t, "read_activity_log", map[string]any{"action": "user.*"})
	entries := logged["entries"].([]any)
	if len(entries) < 7 {
		t.Fatalf("%d user entries logged", len(entries))
	}
	for _, raw := range entries {
		if e := raw.(map[string]any); e["actor"] != "KI: verwaltung" {
			t.Errorf("entry %v not logged under the key", e["action"])
		}
	}
}

// Name, letter and logo, with the logo held to the same checks as the upload.
func TestBrandingThroughTheHandler(t *testing.T) {
	rig := newInstallationRig(t)
	branding.SetDir(t.TempDir())
	t.Cleanup(func() {
		branding.SetDir("")
		branding.Load(context.Background(), rig.h.db.Read)
	})

	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nbild"))
	got := rig.must(t, "update_branding", map[string]any{
		"name": "Hofladen", "mark": "HL", "logo_base64": png, "logo_filename": "marke.png",
	})
	if got["name"] != "Hofladen" || got["mark"] != "HL" || got["has_logo"] != true {
		t.Errorf("brand = %v", got)
	}
	if !strings.HasSuffix(branding.LogoPath(), "logo.png") {
		t.Errorf("logo stored at %q", branding.LogoPath())
	}

	script := base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`))
	rig.refused(t, "update_branding", map[string]any{"logo_base64": script, "logo_filename": "x.svg"}, "PNG, WebP and SVG")
	rig.refused(t, "update_branding", map[string]any{"logo_base64": png, "logo_filename": "x.gif"}, "PNG, WebP and SVG")

	// Only the mark: the name stays.
	if got := rig.must(t, "update_branding", map[string]any{"mark": "H"}); got["name"] != "Hofladen" {
		t.Errorf("the name changed with the mark: %v", got)
	}
	if got := rig.must(t, "update_branding", map[string]any{"remove_logo": true}); got["has_logo"] != false {
		t.Errorf("the logo is still there: %v", got)
	}
}

// A language file goes in and out through the same checks as the upload.
func TestLanguagesThroughTheHandler(t *testing.T) {
	rig := newInstallationRig(t)
	languageDir(t)

	var source string
	for _, s := range i18n.SourceStrings() {
		if !strings.Contains(s, "%") {
			source = s
			break
		}
	}
	file, _ := json.Marshal(map[string]string{source: "Vertaald"})

	got := rig.must(t, "install_language", map[string]any{"code": "nl", "json": string(file)})
	if got["translations"] != float64(1) {
		t.Errorf("install = %v", got)
	}
	if i18n.T("nl", source) != "Vertaald" {
		t.Error("the language is not in use")
	}
	listed := false
	for _, raw := range rig.must(t, "list_languages", nil)["languages"].([]any) {
		if l := raw.(map[string]any); l["code"] == "nl" && l["from_file"] == true {
			listed = true
		}
	}
	if !listed {
		t.Error("nl is not listed as a file")
	}

	rig.refused(t, "install_language", map[string]any{"code": "nicht gut", "json": "{}"}, "not a language tag")
	rig.refused(t, "install_language", map[string]any{"code": "sv", "json": "kein json"}, "refused")
	rig.refused(t, "remove_language", map[string]any{"code": "de", "confirm": true}, "belongs to the program")
	rig.must(t, "remove_language", map[string]any{"code": "nl", "confirm": true})
	if i18n.Known("nl") {
		t.Error("nl is still known")
	}
}

// Without mail and without plugins the tools answer rather than pretend.
func TestMailAndPluginsWhenNotSetUp(t *testing.T) {
	rig := newInstallationRig(t)

	st := rig.must(t, "get_mail_status", nil)
	if st["configured"] != false {
		t.Errorf("mail status = %v", st)
	}
	rig.refused(t, "send_test_mail", map[string]any{"to": "chefin@example.org"}, "no mail server")
	if got := rig.must(t, "retry_mail", nil); got["requeued"] != float64(0) {
		t.Errorf("retry = %v", got)
	}
	rig.refused(t, "list_plugins", nil, "not available")
	rig.refused(t, "enable_plugin", map[string]any{"id": "suche"}, "not available")
}
