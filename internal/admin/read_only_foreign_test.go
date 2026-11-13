package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

// The read-only guard stands after the scope check, so an id that is not one
// of this website's answers 404 for somebody without the publishing right as
// for everybody else — not the read-only sentence, which would say the label
// or the redirect exists here (rights.go, refuseReadOnly). Labels and
// redirects had no scope lookup of their own before the guard.
func TestAForeignLabelOrRedirectAnswers404WithoutPublishing(t *testing.T) {
	f := seedLabelsAndRedirects(t)
	ctx := context.Background()

	res, err := f.database.Write.Exec(`INSERT INTO websites (name, description) VALUES ('Andere', '')`)
	if err != nil {
		t.Fatalf("insert website: %v", err)
	}
	other, _ := res.LastInsertId()
	if _, err := f.h.terms.EnsureNames(ctx, other, []string{"Fremd"}); err != nil {
		t.Fatalf("EnsureNames: %v", err)
	}
	foreignLabels, err := f.h.terms.ListAll(ctx, other)
	if err != nil || len(foreignLabels) != 1 {
		t.Fatalf("ListAll: %v %v", foreignLabels, err)
	}
	if err := f.h.pages.AddRedirect(ctx, other, "/fremd", "/start", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}
	foreignRedirects, err := f.h.pages.ListRedirects(ctx, other)
	if err != nil || len(foreignRedirects) != 1 {
		t.Fatalf("ListRedirects: %v %v", foreignRedirects, err)
	}

	labelAt := func(id int64) string { return fmt.Sprintf("/admin/websites/%d/tags/%d", f.ws, id) }
	redirectAt := func(id int64) string { return fmt.Sprintf("/admin/websites/%d/redirects/%d/delete", f.ws, id) }
	foreignLabel := foreignLabels[0].ID
	termRoute := websiteRoute(f.ws, "termID", strconv.FormatInt(foreignLabel, 10))

	for _, c := range []struct {
		what string
		fn   func(http.ResponseWriter, *http.Request) error
		req  *http.Request
	}{
		{"rename another website's label", f.h.HandleTermRename,
			postForm(labelAt(foreignLabel)+"/rename", url.Values{"name": {"X"}}, termRoute)},
		{"delete another website's label", f.h.HandleTermDelete,
			postForm(labelAt(foreignLabel)+"/delete", nil, termRoute)},
		{"delete another website's redirect", f.h.HandleRedirectDelete,
			postForm(redirectAt(foreignRedirects[0].ID), nil,
				websiteRoute(f.ws, "redirectID", strconv.FormatInt(foreignRedirects[0].ID, 10)))},
		{"delete a redirect that does not exist", f.h.HandleRedirectDelete,
			postForm(redirectAt(987654), nil, websiteRoute(f.ws, "redirectID", "987654"))},
	} {
		rec, flash := serveAs(t, f.h, f.sm, f.editor, c.fn, c.req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, flash %q; want 404", c.what, rec.Code, flash)
		}
	}

	if got, _ := f.h.terms.ListAll(ctx, other); len(got) != 1 || got[0].Name != "Fremd" {
		t.Errorf("the other website's label changed: %v", got)
	}
	if got, _ := f.h.pages.ListRedirects(ctx, other); len(got) != 1 {
		t.Errorf("the other website's redirect changed: %v", got)
	}
}
