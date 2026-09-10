package admin

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/user"
)

// Saving the user form unchanged must not change what somebody may reach.
//
// Since migration 00052 an editor can be limited to no website at all: the
// only website they were limited to was deleted, and a limited account whose
// last row goes reaches nothing. The user form could not show that state. It
// knew two answers — ticks, or no tick meaning every website — so it showed
// such an editor without ticks, and an administrator who opened the form and
// saved it without touching anything turned "no website" into "every website".
// Before 00052 the state was not reachable; the change that made a deleted
// website fail closed made it reachable, and this is the edge of that change.
func TestSavingTheUserFormUnchangedKeepsWhatAnEditorMayReach(t *testing.T) {
	h, sm, database, siteA := newTestAdmin(t)
	ctx := context.Background()
	domains := domain.NewStore(database)
	siteB, err := domains.CreateWebsite(ctx, "Seite B", "")
	if err != nil {
		t.Fatalf("create Seite B: %v", err)
	}

	limited := seedAccount(t, database, "begrenzt@example.com", user.RoleEditor)
	if err := h.users.SetRights(ctx, limited,
		user.Rights{MayPublish: true, Websites: []int64{siteA.ID}}); err != nil {
		t.Fatalf("limit the editor to %q: %v", siteA.Name, err)
	}
	if err := domains.DeleteWebsite(ctx, siteA.ID); err != nil {
		t.Fatalf("delete %q: %v", siteA.Name, err)
	}
	// The control. A fix that answers this by limiting everybody who is saved
	// through the form would lock this editor out, and that is not a fix.
	unlimited := seedAccount(t, database, "offen@example.com", user.RoleEditor)

	lookup := NewWebsiteAccessLookup(database)
	if lookup(ctx, limited, siteB.ID) {
		t.Fatal("before the form is saved, the editor whose only website was deleted already reaches " +
			"Seite B; this test would be measuring the cascade, not the form")
	}
	if !lookup(ctx, unlimited, siteB.ID) {
		t.Fatal("before the form is saved, an editor nobody limited cannot reach Seite B")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/users/{id}/edit", h.ErrHandler(h.HandleUserEdit))
	mux.HandleFunc("POST /admin/users/{id}/edit", h.ErrHandler(h.HandleUserEdit))
	handler := sm.LoadAndSave(mux)

	for _, tc := range []struct {
		name  string
		id    int64
		wantB bool
	}{
		{"an editor limited to no website stays limited to none", limited, false},
		{"an editor nobody limited stays unlimited", unlimited, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/admin/users/" + strconv.FormatInt(tc.id, 10) + "/edit"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s answered %d", path, rec.Code)
			}
			form := formAsShown(t, rec.Body.String())

			post := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
			post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("POST %s answered %d; want 303 (form sent: %v)", path, rec.Code, form)
			}

			if got := lookup(ctx, tc.id, siteB.ID); got != tc.wantB {
				t.Errorf("after the form was saved exactly as it was shown, the editor reaches Seite B = %v; "+
					"want %v (form sent: %v)", got, tc.wantB, form)
			}
		})
	}
}

var (
	userEditForm = regexp.MustCompile(`(?s)<form\b[^>]*action="/admin/users/\d+/edit"[^>]*>.*?</form>`)
	inputTag     = regexp.MustCompile(`<input\b[^>]*>`)
	tagAttr      = regexp.MustCompile(`\b(name|value|type)="([^"]*)"`)
	checkedAttr  = regexp.MustCompile(`\bchecked\b`)
	roleSelect   = regexp.MustCompile(`(?s)<select\b[^>]*name="role"[^>]*>(.*?)</select>`)
	selectedRole = regexp.MustCompile(`<option\b[^>]*value="([^"]*)"[^>]*\bselected\b`)
)

// formAsShown reads back what the user edit form would send if it were
// submitted untouched: fields with their values, checkboxes and radio buttons
// only when checked, and the selected role. It reads the template's own
// output, which is what a browser would submit, rather than rebuilding the
// request from what the handler was expected to render.
func formAsShown(t *testing.T, body string) url.Values {
	t.Helper()
	formHTML := userEditForm.FindString(body)
	if formHTML == "" {
		t.Fatal("the page carries no user edit form")
	}
	vals := url.Values{}
	for _, tag := range inputTag.FindAllString(formHTML, -1) {
		a := map[string]string{}
		for _, m := range tagAttr.FindAllStringSubmatch(tag, -1) {
			a[m[1]] = html.UnescapeString(m[2])
		}
		if a["name"] == "" {
			continue
		}
		switch a["type"] {
		case "checkbox", "radio":
			if checkedAttr.MatchString(tag) {
				vals.Add(a["name"], a["value"])
			}
		case "submit", "button":
		default:
			vals.Add(a["name"], a["value"])
		}
	}
	if sel := roleSelect.FindStringSubmatch(formHTML); sel != nil {
		if opt := selectedRole.FindStringSubmatch(sel[1]); opt != nil {
			vals.Set("role", opt[1])
		}
	}
	return vals
}
