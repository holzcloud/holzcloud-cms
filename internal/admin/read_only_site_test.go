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
)

// The rule of read_only_test.go, applied to the rest of what a website shows
// at once: its settings, its theme and its wording.
//
// Each of these is stored and served in the same moment — a new name is in the
// title of every page, a theme is the look of every page, a word is what a
// visitor reads — so an editor without the publishing right may open these
// screens and not change them. As in the other files, every door is driven
// with a hand-built request: the screen leaving a button out is a courtesy,
// and the handler refusing is the border.

func websiteSettingsAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d", websiteID)
}

func websiteDesignAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/design", websiteID)
}

// insideDisabledFieldset reports whether mark stands inside a fieldset that
// carries the disabled attribute: after such an opening tag, with no closing
// tag between them. Nested fieldsets further in are allowed, because the
// settings form keeps its own business fieldset inside the outer one.
func insideDisabledFieldset(body, mark string) bool {
	at := strings.Index(body, mark)
	if at < 0 {
		return false
	}
	before := body[:at]
	opens := disabledFieldset.FindAllStringIndex(before, -1)
	for _, open := range opens {
		if !strings.Contains(before[open[1]:], "</fieldset>") {
			return true
		}
	}
	return false
}

func storedWebsite(t *testing.T, h *Handler, id int64) *domain.Website {
	t.Helper()
	ws, err := h.domains.GetWebsite(context.Background(), id)
	if err != nil || ws == nil {
		t.Fatalf("GetWebsite: %v", err)
	}
	return ws
}

func activeSlug(t *testing.T, h *Handler, websiteID int64) string {
	t.Helper()
	slug, err := h.tmplStore.ActiveTemplateSlug(context.Background(), websiteID)
	if err != nil {
		t.Fatalf("ActiveTemplateSlug: %v", err)
	}
	return slug
}

func TestAnEditorWithoutPublishingCannotChangeTheWebsiteSettingsOrTheme(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	route := websiteRoute(ws.ID)

	standard, err := h.tmplStore.CreateBuiltin(ctx, "Standard", "default")
	if err != nil {
		t.Fatalf("CreateBuiltin: %v", err)
	}
	other, err := h.tmplStore.Create(ctx, "Anders", "anders")
	if err != nil {
		t.Fatalf("Create template: %v", err)
	}
	if err := h.tmplStore.ActivateForWebsite(ctx, ws.ID, standard.ID); err != nil {
		t.Fatalf("ActivateForWebsite: %v", err)
	}

	settings := url.Values{
		"name": {"Umbenannt"}, "locale": {"de"}, "timezone": {"Europe/Zurich"},
		"offline_mode": {"notfound"}, "city": {"Pforzheim"},
	}
	activate := url.Values{"template_id": {strconv.FormatInt(other.ID, 10)}}

	t.Run("settings", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleWebsiteEdit,
			postForm(websiteSettingsAddress(ws.ID), settings, route))
		wantRefused(t, "settings save", rec, flash, websiteSettingsAddress(ws.ID))
		rec, flash = serveAs(t, h, sm, editor, h.HandleWebsiteEdit,
			htmx(postForm(websiteSettingsAddress(ws.ID), settings, route)))
		wantRefusedHtmx(t, "settings save", rec, flash, websiteSettingsAddress(ws.ID))
		after := storedWebsite(t, h, ws.ID)
		if after.Name != ws.Name || after.City != ws.City {
			t.Errorf("the settings were changed: name %q, town %q", after.Name, after.City)
		}
	})

	t.Run("theme", func(t *testing.T) {
		target := websiteDesignAddress(ws.ID) + "/activate"
		rec, flash := serveAs(t, h, sm, editor, h.HandleWebsiteDesignActivate, postForm(target, activate, route))
		wantRefused(t, "theme activation", rec, flash, websiteDesignAddress(ws.ID))
		rec, flash = serveAs(t, h, sm, editor, h.HandleWebsiteDesignActivate, htmx(postForm(target, activate, route)))
		wantRefusedHtmx(t, "theme activation", rec, flash, websiteDesignAddress(ws.ID))
		if got := activeSlug(t, h, ws.ID); got != "default" {
			t.Errorf("the active theme became %q", got)
		}
	})

	// The control: somebody who may publish does both.
	t.Run("a person who may publish", func(t *testing.T) {
		publisher := editorWhoMayPublish(t, database, ws.ID)
		rec, flash := serveAs(t, h, sm, publisher, h.HandleWebsiteEdit,
			postForm(websiteSettingsAddress(ws.ID), settings, route))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("settings: status %d, flash %q", rec.Code, flash)
		}
		if got := storedWebsite(t, h, ws.ID).Name; got != "Umbenannt" {
			t.Errorf("a person who may publish could not rename the website: %q", got)
		}
		rec, flash = serveAs(t, h, sm, publisher, h.HandleWebsiteDesignActivate,
			postForm(websiteDesignAddress(ws.ID)+"/activate", activate, route))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("theme: status %d, flash %q", rec.Code, flash)
		}
		if got := activeSlug(t, h, ws.ID); got != "anders" {
			t.Errorf("a person who may publish could not activate the theme: %q", got)
		}
	})
}

func TestTheSettingsAndDesignScreensOfferNoChangeWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	route := websiteRoute(ws.ID)

	standard, err := h.tmplStore.CreateBuiltin(ctx, "Standard", "default")
	if err != nil {
		t.Fatalf("CreateBuiltin: %v", err)
	}
	if _, err := h.tmplStore.Create(ctx, "Anders", "anders"); err != nil {
		t.Fatalf("Create template: %v", err)
	}
	if err := h.tmplStore.ActivateForWebsite(ctx, ws.ID, standard.ID); err != nil {
		t.Fatalf("ActivateForWebsite: %v", err)
	}

	const (
		nameInput   = `id="name" name="name"`
		saveButton  = "Save changes"
		privacyForm = "/datenschutz-vorlage"
		activate    = `/design/activate"`
	)

	body := getAs(t, sm, putUser(sm, editor), h.HandleWebsiteEdit, websiteSettingsAddress(ws.ID), route)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("settings: the sentence stands %d times, want once", n)
	}
	if !insideDisabledFieldset(body, nameInput) {
		t.Error("settings: the name is not in a disabled fieldset")
	}
	if !strings.Contains(body, ws.Name) {
		t.Error("settings: the values are no longer shown")
	}
	if strings.Contains(body, saveButton) {
		t.Error("settings: Save changes is offered")
	}
	if !strings.Contains(body, privacyForm) {
		t.Error("settings: the privacy template, which only makes a draft, is no longer offered")
	}

	body = getAs(t, sm, putUser(sm, editor), h.HandleWebsiteDesign, websiteDesignAddress(ws.ID), route)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("design: the sentence stands %d times, want once", n)
	}
	if strings.Contains(body, activate) {
		t.Error("design: a theme can be activated")
	}
	if !strings.Contains(body, "Anders") {
		t.Error("design: the other theme is no longer listed")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleWebsiteEdit, websiteSettingsAddress(ws.ID), route)
	if strings.Contains(body, readOnlyText) || insideDisabledFieldset(body, nameInput) || !strings.Contains(body, saveButton) {
		t.Error("settings: a person who may publish cannot save them")
	}
	body = getAs(t, sm, putUser(sm, publisher), h.HandleWebsiteDesign, websiteDesignAddress(ws.ID), route)
	if strings.Contains(body, readOnlyText) || !strings.Contains(body, activate) {
		t.Error("design: a person who may publish cannot activate a theme")
	}
}

func wordingAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/wording", websiteID)
}

func TestAnEditorWithoutPublishingCannotChangeTheWording(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	editor := editorWithoutPublishing(t, database, ws.ID)
	route := websiteRoute(ws.ID)

	keys, _ := h.themeVocabulary(ctx, ws)
	if len(keys) == 0 {
		t.Fatal("the shipped theme asks for no words")
	}
	form := url.Values{"w:" + ws.Locale + ":" + keys[0]: {"Mein Wort"}}
	stored := func(t *testing.T) int {
		t.Helper()
		all, err := h.wording.All(ctx, ws.ID)
		if err != nil {
			t.Fatalf("wording.All: %v", err)
		}
		return len(all)
	}

	rec, flash := serveAs(t, h, sm, editor, h.HandleWebsiteWordingSave, postForm(wordingAddress(ws.ID), form, route))
	wantRefused(t, "wording save", rec, flash, wordingAddress(ws.ID))
	rec, flash = serveAs(t, h, sm, editor, h.HandleWebsiteWordingSave, htmx(postForm(wordingAddress(ws.ID), form, route)))
	wantRefusedHtmx(t, "wording save", rec, flash, wordingAddress(ws.ID))
	if n := stored(t); n != 0 {
		t.Errorf("%d words stored for the website, want none", n)
	}

	// The control: somebody who may publish saves the word.
	publisher := editorWhoMayPublish(t, database, ws.ID)
	rec, flash = serveAs(t, h, sm, publisher, h.HandleWebsiteWordingSave, postForm(wordingAddress(ws.ID), form, route))
	if rec.Code != http.StatusSeeOther || flash != "" {
		t.Errorf("publisher: status %d, flash %q", rec.Code, flash)
	}
	if n := stored(t); n != 1 {
		t.Errorf("a person who may publish saved %d words, want one", n)
	}
}

func TestTheWordingScreenOffersNoChangeWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	route := websiteRoute(ws.ID)

	const (
		wordInput  = `name="w:`
		saveButton = "Save wording"
	)

	body := getAs(t, sm, putUser(sm, editor), h.HandleWebsiteWording, wordingAddress(ws.ID), route)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("wording: the sentence stands %d times, want once", n)
	}
	if !insideDisabledFieldset(body, wordInput) {
		t.Error("wording: the words are not in a disabled fieldset")
	}
	if strings.Contains(body, saveButton) {
		t.Error("wording: Save wording is offered")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleWebsiteWording, wordingAddress(ws.ID), route)
	if strings.Contains(body, readOnlyText) || insideDisabledFieldset(body, wordInput) || !strings.Contains(body, saveButton) {
		t.Error("wording: a person who may publish cannot save it")
	}
}
