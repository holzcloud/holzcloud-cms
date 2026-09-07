package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/snippet"
)

// .planning/debug/knowledge-base.md states the rule this file exists for:
// where the second resource's table carries no website_id — or where the
// store's lookup does not take one — the check lives in the handler and NEEDS
// a *_scope_test.go, because nothing else will hold it.
//
// The snippet surface is correct today. Four call sites each compare
// sn.WebsiteID against the website in the address, and internal/admin/field.go
// went as far as centralising its own comparison into snippetOf, with a comment
// naming exactly why: "snippets.Get takes only an id ... that asymmetry is the
// whole reason this is one function rather than two inline comparisons that can
// drift apart."
//
// Correct because four people remembered is not the same as correct. These
// tests hold the surface to the promise, and the store now takes the website id
// so that a fifth caller cannot forget — which is the fix the comment above was
// asking for.

type snippetScopeFixture struct {
	handler *Handler
	sm      interface {
		Put(context.Context, string, any)
	}

	siteA, siteB *domain.Website
	mine, theirs *snippet.Snippet
}

func newSnippetScopeFixture(t *testing.T) *snippetScopeFixture {
	t.Helper()
	ctx := context.Background()
	h, _, database, siteA := newTestAdmin(t)

	siteB, err := domain.NewStore(database).CreateWebsite(ctx, "Fremde Seite", "")
	if err != nil {
		t.Fatalf("CreateWebsite B: %v", err)
	}
	store := snippet.NewStore(database)
	mine, err := store.Create(ctx, siteA.ID, "kontakt", "Kontakt", "Bei uns", "<p>Bei uns</p>")
	if err != nil {
		t.Fatalf("create A's snippet: %v", err)
	}
	theirs, err := store.Create(ctx, siteB.ID, "kontakt", "Fremder Kontakt", "Nicht deins", "<p>Nicht deins</p>")
	if err != nil {
		t.Fatalf("create B's snippet: %v", err)
	}
	return &snippetScopeFixture{handler: h, siteA: siteA, siteB: siteB, mine: mine, theirs: theirs}
}

// A snippet of website B, named behind website A in the address, must not be
// deleted — and the assertion is that it is still there, not that the status
// was 404.
func TestSnippetDeleteRefusesAForeignWebsitesSnippet(t *testing.T) {
	f := newSnippetScopeFixture(t)
	h := f.handler

	req := postForm(fmt.Sprintf("/admin/websites/%d/snippets/%d/delete", f.siteA.ID, f.theirs.ID),
		url.Values{}, map[string]string{
			"id":        strconv.FormatInt(f.siteA.ID, 10),
			"snippetID": strconv.FormatInt(f.theirs.ID, 10),
		})
	rec := serve(t, h, h.sm, h.HandleSnippetDelete, req)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("status %d, want 404 or 403", rec.Code)
	}

	still, err := snippet.NewStore(h.db).Get(context.Background(), f.siteB.ID, f.theirs.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if still == nil {
		t.Error("website B's snippet was deleted through website A's address")
	}
}

// The save arm, which is the one that carries a body: a foreign snippet id in
// the form must not have its text, its name or its key rewritten.
func TestSnippetSaveRefusesAForeignWebsitesSnippet(t *testing.T) {
	f := newSnippetScopeFixture(t)
	h := f.handler

	req := postForm(fmt.Sprintf("/admin/websites/%d/snippets", f.siteA.ID), url.Values{
		"id":               {strconv.FormatInt(f.theirs.ID, 10)},
		"key":              {"uebernommen"},
		"name":             {"Übernommen"},
		"content_markdown": {"Jetzt meins."},
	}, map[string]string{"id": strconv.FormatInt(f.siteA.ID, 10)})
	rec := serve(t, h, h.sm, h.HandleSnippetList, req)
	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "Jetzt meins") {
		t.Log("the save was rendered back; the assertion below is what decides")
	}

	after, err := snippet.NewStore(h.db).Get(context.Background(), f.siteB.ID, f.theirs.ID)
	if err != nil || after == nil {
		t.Fatalf("Get: %v", err)
	}
	if after.Name != f.theirs.Name {
		t.Errorf("website B's snippet was renamed from %q to %q", f.theirs.Name, after.Name)
	}
	if after.ContentMarkdown != f.theirs.ContentMarkdown {
		t.Errorf("website B's snippet text was overwritten with %q", after.ContentMarkdown)
	}
	if after.Key != f.theirs.Key {
		t.Errorf("website B's snippet key was changed from %q to %q", f.theirs.Key, after.Key)
	}
}

// The store itself, which is where the guard now lives: a lookup with the wrong
// website finds nothing, whatever the caller does or forgets to do.
func TestTheSnippetStoreDoesNotFindAnotherWebsitesSnippet(t *testing.T) {
	f := newSnippetScopeFixture(t)
	store := snippet.NewStore(f.handler.db)
	ctx := context.Background()

	if sn, err := store.Get(ctx, f.siteA.ID, f.theirs.ID); err != nil || sn != nil {
		t.Errorf("Get with website A found website B's snippet: %+v (err %v)", sn, err)
	}
	if err := store.Update(ctx, f.siteA.ID, f.theirs.ID, "x", "X", "x", "<p>x</p>"); err == nil {
		t.Error("Update with website A rewrote website B's snippet")
	}
	if err := store.SetFields(ctx, f.siteA.ID, f.theirs.ID, `{"werte":{"a":"b"}}`); err == nil {
		t.Error("SetFields with website A wrote into website B's snippet")
	}
	if err := store.Delete(ctx, f.siteA.ID, f.theirs.ID); err == nil {
		t.Error("Delete with website A removed website B's snippet")
	}
	// And the honest half: the right website still works.
	if sn, err := store.Get(ctx, f.siteA.ID, f.mine.ID); err != nil || sn == nil {
		t.Errorf("Get with website A did not find website A's own snippet (err %v)", err)
	}
}
