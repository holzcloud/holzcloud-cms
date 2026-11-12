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
	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// The rule of read_only_test.go, applied to the rest of what a website shows
// at once: its settings, its theme, its wording and its published products.
//
// Products follow the page rule rather than the menu rule (D-14): a draft
// product is preparation, so an editor without the publishing right creates
// and changes drafts as they do draft pages, and whatever they save stays a
// draft; a published product is online, and its text, price, stock, picture
// and its removal are a publisher's business.
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

// draftHint stands on the product form of somebody without the publishing
// right, in place of the status choice they no longer have.
const draftHint = "Publishing is not part of your account: the product stays a draft until somebody who may publish puts it online."

// productSaveButton is the form's Save; a bare "Save" would match the hint.
const productSaveButton = `hx-disabled-elt="this">Save</button>`

func productListAddress(websiteID int64) string {
	return fmt.Sprintf("/admin/websites/%d/produkte", websiteID)
}

func productAddress(websiteID, productID int64) string {
	return fmt.Sprintf("/admin/websites/%d/produkte/%d", websiteID, productID)
}

// withShop is newTestAdmin with the catalogue switched on, which most admin
// tests do not need and these do.
func withShop(h *Handler) *shop.Store {
	products := shop.NewStore(h.db)
	h.SetProductStore(products)
	return products
}

// seedShopProduct stores a product with everything a visitor sees of it: a
// text, a price, a stock and a picture.
func seedShopProduct(t *testing.T, h *Handler, products *shop.Store, websiteID int64, slug, status string) *shop.Product {
	t.Helper()
	ctx := context.Background()
	picture := seedImage(t, h, websiteID, slug+".jpg", 40, 30)
	stock := 7
	id, err := products.Create(ctx, &shop.Product{
		WebsiteID:           websiteID,
		Slug:                slug,
		Title:               "Esstisch " + slug,
		DescriptionMarkdown: "Eiche massiv.",
		DescriptionHTML:     "<p>Eiche massiv.</p>",
		PriceGross:          129000,
		TaxRate:             money.RateStandard,
		Stock:               &stock,
		Status:              status,
		FeaturedMediaID:     &picture.ID,
	})
	if err != nil {
		t.Fatalf("Create product %s: %v", slug, err)
	}
	p, err := products.Get(ctx, id)
	if err != nil || p == nil {
		t.Fatalf("Get product %s: %v", slug, err)
	}
	return p
}

// productInputOf is the product as its form shows it, so a pointer field can
// be named in a failure message.
func productInputOf(p *shop.Product) shop.ProductInput { return productToValues(p).ProductInput }

func productRoute(websiteID int64, productID string) map[string]string {
	return websiteRoute(websiteID, "productID", productID)
}

func reloadProduct(t *testing.T, products *shop.Store, id int64) *shop.Product {
	t.Helper()
	p, err := products.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get product: %v", err)
	}
	return p
}

// productForm is a full product form that changes everything a visitor sees
// and asks for the product to be published.
func productForm(title string, featured int64) url.Values {
	return url.Values{
		"title":                {title},
		"description_markdown": {"Ein ganz neuer Text."},
		"price":                {"0.05"},
		"tax_bp":               {"810"},
		"stock":                {"99"},
		"weight_grams":         {"0"},
		"featured_media_id":    {strconv.FormatInt(featured, 10)},
		"status":               {shop.StatusPublished},
	}
}

func TestAnEditorWithoutPublishingWorksOnDraftProducts(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	products := withShop(h)
	editor := editorWithoutPublishing(t, database, ws.ID)
	draft := seedShopProduct(t, h, products, ws.ID, "entwurf", shop.StatusDraft)

	t.Run("a new product is stored as a draft", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleProductForm,
			postForm(productListAddress(ws.ID)+"/neu", productForm("Neuer Stuhl", 0), productRoute(ws.ID, "neu")))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Fatalf("create: status %d, flash %q", rec.Code, flash)
		}
		all, err := products.List(ctx, ws.ID)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		var created *shop.Product
		for _, p := range all {
			if p.Title == "Neuer Stuhl" {
				created = p
			}
		}
		if created == nil {
			t.Fatal("the new product was not stored")
		}
		if created.Status != shop.StatusDraft {
			t.Errorf("the new product was stored %q, want draft", created.Status)
		}
	})

	t.Run("a draft is changed and stays a draft", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleProductForm,
			postForm(productAddress(ws.ID, draft.ID), productForm("Umbenannt", 0),
				productRoute(ws.ID, strconv.FormatInt(draft.ID, 10))))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Fatalf("draft save: status %d, flash %q", rec.Code, flash)
		}
		after := reloadProduct(t, products, draft.ID)
		if after == nil {
			t.Fatal("the draft disappeared")
		}
		if after.Title != "Umbenannt" || after.PriceGross != 5 {
			t.Errorf("the draft was not changed: %q, %s", after.Title, money.Input(after.PriceGross))
		}
		if after.Status != shop.StatusDraft {
			t.Errorf("the draft was stored %q, want draft", after.Status)
		}
	})

	t.Run("the screens offer saving and no status", func(t *testing.T) {
		for target, id := range map[string]string{
			productListAddress(ws.ID) + "/neu": "neu",
			productAddress(ws.ID, draft.ID):    strconv.FormatInt(draft.ID, 10),
		} {
			body := getAs(t, sm, putUser(sm, editor), h.HandleProductForm, target, productRoute(ws.ID, id))
			if !strings.Contains(body, productSaveButton) {
				t.Errorf("%s: no Save", target)
			}
			if strings.Contains(body, `name="status"`) {
				t.Errorf("%s: the status choice is offered", target)
			}
			if !strings.Contains(body, draftHint) {
				t.Errorf("%s: the draft hint is missing", target)
			}
			if strings.Contains(body, readOnlyText) {
				t.Errorf("%s: a draft reads the read-only sentence", target)
			}
		}
	})

	t.Run("a draft is deleted", func(t *testing.T) {
		rec, flash := serveAs(t, h, sm, editor, h.HandleProductDelete,
			postForm(productAddress(ws.ID, draft.ID)+"/delete", nil, productRoute(ws.ID, strconv.FormatInt(draft.ID, 10))))
		if rec.Code != http.StatusSeeOther || flash != "" {
			t.Errorf("draft delete: status %d, flash %q", rec.Code, flash)
		}
		if reloadProduct(t, products, draft.ID) != nil {
			t.Error("the draft was not deleted")
		}
	})
}

func TestAnEditorWithoutPublishingCannotChangeAPublishedProduct(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	ctx := context.Background()
	products := withShop(h)
	editor := editorWithoutPublishing(t, database, ws.ID)
	live := seedShopProduct(t, h, products, ws.ID, "online", shop.StatusPublished)
	other := seedImage(t, h, ws.ID, "anderes.jpg", 40, 30)
	id := strconv.FormatInt(live.ID, 10)
	form := productForm("Übernommen", other.ID)

	unchanged := func(t *testing.T, what string) {
		t.Helper()
		after := reloadProduct(t, products, live.ID)
		if after == nil {
			t.Fatalf("%s: the published product was deleted", what)
		}
		if after.Title != live.Title || after.DescriptionMarkdown != live.DescriptionMarkdown ||
			after.PriceGross != live.PriceGross || after.Status != live.Status {
			t.Errorf("%s: the published product was changed: %q %q %s %q", what,
				after.Title, after.DescriptionMarkdown, money.Input(after.PriceGross), after.Status)
		}
		if after.Stock == nil || *after.Stock != *live.Stock {
			t.Errorf("%s: the stock was changed to %s", what, productInputOf(after).StockText)
		}
		if after.FeaturedMediaID == nil || *after.FeaturedMediaID != *live.FeaturedMediaID {
			t.Errorf("%s: the picture was changed to %d", what, productInputOf(after).FeaturedID)
		}
	}

	rec, flash := serveAs(t, h, sm, editor, h.HandleProductForm,
		postForm(productAddress(ws.ID, live.ID), form, productRoute(ws.ID, id)))
	wantRefused(t, "published product save", rec, flash, productAddress(ws.ID, live.ID))
	unchanged(t, "save")

	rec, flash = serveAs(t, h, sm, editor, h.HandleProductForm,
		htmx(postForm(productAddress(ws.ID, live.ID), form, productRoute(ws.ID, id))))
	wantRefusedHtmx(t, "published product save", rec, flash, productAddress(ws.ID, live.ID))
	unchanged(t, "htmx save")

	rec, flash = serveAs(t, h, sm, editor, h.HandleProductDelete,
		postForm(productAddress(ws.ID, live.ID)+"/delete", nil, productRoute(ws.ID, id)))
	wantRefused(t, "published product delete", rec, flash, productListAddress(ws.ID))
	unchanged(t, "delete")

	// Another website's product is not this person's business at all: the
	// scope answers first, and it answers 404, not the sentence.
	site, err := h.domains.CreateWebsite(ctx, "Andere", "")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}
	foreign := seedShopProduct(t, h, products, site.ID, "fremd", shop.StatusPublished)
	foreignID := strconv.FormatInt(foreign.ID, 10)
	rec, flash = serveAs(t, h, sm, editor, h.HandleProductForm,
		postForm(productAddress(ws.ID, foreign.ID), form, productRoute(ws.ID, foreignID)))
	if rec.Code != http.StatusNotFound || flash == readOnlyText {
		t.Errorf("foreign product save: status %d, flash %q, want 404 without the sentence", rec.Code, flash)
	}
	rec, flash = serveAs(t, h, sm, editor, h.HandleProductDelete,
		postForm(productAddress(ws.ID, foreign.ID)+"/delete", nil, productRoute(ws.ID, foreignID)))
	if rec.Code != http.StatusNotFound || flash == readOnlyText {
		t.Errorf("foreign product delete: status %d, flash %q, want 404 without the sentence", rec.Code, flash)
	}
	if after := reloadProduct(t, products, foreign.ID); after == nil || after.Title != foreign.Title {
		t.Error("the foreign product was changed")
	}
}

func TestTheProductScreensOfferNoChangeOfAPublishedProductWithoutPublishing(t *testing.T) {
	h, sm, database, ws := newTestAdmin(t)
	products := withShop(h)
	editor := editorWithoutPublishing(t, database, ws.ID)
	publisher := editorWhoMayPublish(t, database, ws.ID)
	draft := seedShopProduct(t, h, products, ws.ID, "entwurf", shop.StatusDraft)
	live := seedShopProduct(t, h, products, ws.ID, "online", shop.StatusPublished)

	newLink := `/produkte/neu"`
	deleteDraft := fmt.Sprintf(`/produkte/%d/delete"`, draft.ID)
	deleteLive := fmt.Sprintf(`/produkte/%d/delete"`, live.ID)
	liveRoute := productRoute(ws.ID, strconv.FormatInt(live.ID, 10))
	draftRoute := productRoute(ws.ID, strconv.FormatInt(draft.ID, 10))

	body := getAs(t, sm, putUser(sm, editor), h.HandleProductList, productListAddress(ws.ID), websiteRoute(ws.ID))
	if !strings.Contains(body, newLink) {
		t.Error("list: New product is not offered")
	}
	if !strings.Contains(body, deleteDraft) {
		t.Error("list: the draft cannot be deleted")
	}
	if strings.Contains(body, deleteLive) {
		t.Error("list: the published product can be deleted")
	}

	body = getAs(t, sm, putUser(sm, editor), h.HandleProductForm, productAddress(ws.ID, live.ID), liveRoute)
	if n := strings.Count(body, readOnlyText); n != 1 {
		t.Errorf("published product: the sentence stands %d times, want once", n)
	}
	if !insideDisabledFieldset(body, `id="title" name="title"`) {
		t.Error("published product: the title is not in a disabled fieldset")
	}
	if strings.Contains(body, productSaveButton) {
		t.Error("published product: Save is offered")
	}
	if !strings.Contains(body, live.Title) {
		t.Error("published product: its values are not shown")
	}

	body = getAs(t, sm, putUser(sm, publisher), h.HandleProductList, productListAddress(ws.ID), websiteRoute(ws.ID))
	if !strings.Contains(body, deleteDraft) || !strings.Contains(body, deleteLive) {
		t.Error("list: a person who may publish cannot delete both")
	}
	for _, page := range []struct {
		target string
		route  map[string]string
	}{
		{productAddress(ws.ID, live.ID), liveRoute},
		{productAddress(ws.ID, draft.ID), draftRoute},
	} {
		body = getAs(t, sm, putUser(sm, publisher), h.HandleProductForm, page.target, page.route)
		if strings.Contains(body, readOnlyText) || strings.Contains(body, draftHint) {
			t.Errorf("%s: a person who may publish reads a sentence meant for others", page.target)
		}
		if !strings.Contains(body, `name="status"`) || !strings.Contains(body, productSaveButton) {
			t.Errorf("%s: a person who may publish has no status choice or no Save", page.target)
		}
	}
}
