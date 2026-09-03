package admin

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The page list as a particular person needs it: which columns it has, and
// which filter combinations are worth a name.
//
// Both are settings of a person and not of a website. What somebody wants to
// see while paging through is a property of their work — one person is looking
// after translations and wants the language column, the next one never touches
// them.

// Column is one optional column of the page list.
type Column struct {
	Key  string
	Name string
	// On is whether this person has it switched on.
	On bool
}

// columnNames are the columns that can be switched, in the order they appear.
//
// Title and the actions are not among them: a list without titles is not a
// list, and a row you cannot open is a dead end.
var columnNames = []struct{ Key, Name string }{
	{"status", "Status"},
	{"art", "Art"},
	{"sprache", "Sprache"},
	{"adresse", "Adresse"},
	{"veroeffentlicht", "Veröffentlicht"},
	{"geaendert", "Geändert"},
}

// defaultColumns is what the list looked like before this existed. A person who
// never opens the chooser must not notice that it is there.
var defaultColumns = []string{"status", "art", "sprache", "veroeffentlicht"}

// ColumnSet is the chosen columns, in the fixed order above.
type ColumnSet []string

// Has reports whether a column is shown. Used by the row template, which gets
// the set with every row so an htmx swap draws the same cells as the page.
func (c ColumnSet) Has(key string) bool {
	for _, k := range c {
		if k == key {
			return true
		}
	}
	return false
}

// parseColumns reads the stored setting. Empty means the default, which is what
// keeps this invisible for everybody who does not want it.
func parseColumns(stored string, multilingual bool) ColumnSet {
	chosen := map[string]bool{}
	if strings.TrimSpace(stored) == "" {
		for _, key := range defaultColumns {
			chosen[key] = true
		}
	} else {
		for _, key := range strings.Split(stored, ",") {
			chosen[strings.TrimSpace(key)] = true
		}
	}

	out := make(ColumnSet, 0, len(columnNames))
	for _, c := range columnNames {
		// A language column on a website with one language is a column of empty
		// cells: it is not offered and not drawn.
		if c.Key == "sprache" && !multilingual {
			continue
		}
		if chosen[c.Key] {
			out = append(out, c.Key)
		}
	}
	return out
}

// columnChoices is the chooser: every switchable column with its tick.
func columnChoices(set ColumnSet, multilingual bool) []Column {
	out := make([]Column, 0, len(columnNames))
	for _, c := range columnNames {
		if c.Key == "sprache" && !multilingual {
			continue
		}
		out = append(out, Column{Key: c.Key, Name: c.Name, On: set.Has(c.Key)})
	}
	return out
}

// pageColumns loads this person's column setting.
func (h *Handler) pageColumns(r *http.Request, multilingual bool) ColumnSet {
	id := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if id == 0 {
		return parseColumns("", multilingual)
	}
	var stored string
	if err := h.db.Read.QueryRowContext(r.Context(),
		`SELECT page_columns FROM users WHERE id = $1`, id).Scan(&stored); err != nil {
		return parseColumns("", multilingual)
	}
	return parseColumns(stored, multilingual)
}

// HandlePageColumns stores which columns this person wants to see.
func (h *Handler) HandlePageColumns(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	id := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if id == 0 {
		http.NotFound(w, r)
		return nil
	}

	// Only known keys, in the fixed order: what comes back is a form and
	// therefore anything at all.
	wanted := map[string]bool{}
	for _, key := range r.PostForm["spalten"] {
		wanted[key] = true
	}
	var keep []string
	for _, c := range columnNames {
		if wanted[c.Key] {
			keep = append(keep, c.Key)
		}
	}
	// Nothing ticked is a legitimate answer — a list of titles and nothing
	// else. It is stored as a marker so it does not read as "never chose".
	stored := strings.Join(keep, ",")
	if stored == "" {
		stored = "-"
	}
	if _, err := h.db.Write.ExecContext(r.Context(),
		`UPDATE users SET page_columns = $1 WHERE id = $2`, stored, id); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Spalten gespeichert")
	return h.redirect(w, r, listURL(websiteID, r.FormValue("zurueck")))
}

// SavedView is one remembered filter combination.
type SavedView struct {
	ID    int64
	Name  string
	Query string
	// Active marks the view whose filter is the one currently shown, so the
	// chips say where you are.
	Active bool
}

// URL is the address of this view.
func (v SavedView) URL(websiteID int64) string {
	return listURL(websiteID, v.Query)
}

// listURL is the page list with a query part, without an empty question mark.
func listURL(websiteID int64, query string) string {
	base := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/pages"
	query = strings.TrimPrefix(strings.TrimSpace(query), "?")
	if query == "" {
		return base
	}
	return base + "?" + query
}

// savedViews loads this person's views for a website, marking the one that is
// currently shown.
func (h *Handler) savedViews(ctx context.Context, userID, websiteID int64, current string) []SavedView {
	if userID == 0 {
		return nil
	}
	rows, err := h.db.Read.QueryContext(ctx,
		`SELECT id, name, query FROM saved_views
		 WHERE user_id = $1 AND website_id = $2 ORDER BY name`, userID, websiteID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []SavedView
	for rows.Next() {
		var v SavedView
		if err := rows.Scan(&v.ID, &v.Name, &v.Query); err != nil {
			return out
		}
		v.Active = sameFilter(v.Query, current)
		out = append(out, v)
	}
	return out
}

// filterQuery is the part of the address worth remembering.
//
// Deliberately not the whole query string: the page number and the search term
// belong to a moment, not to a view somebody comes back to every morning.
func filterQuery(values url.Values) string {
	keep := url.Values{}
	for _, key := range []string{"status", "review", "kind", "sort", "sprache"} {
		if v := strings.TrimSpace(values.Get(key)); v != "" {
			keep.Set(key, v)
		}
	}
	return keep.Encode()
}

// sameFilter reports whether two query parts mean the same view. Both come out
// of url.Values.Encode, which sorts, so a string comparison is enough.
func sameFilter(a, b string) bool { return a != "" && a == b }

// HandleSavedViewCreate remembers the current filter under a name.
func (h *Handler) HandleSavedViewCreate(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	if userID == 0 {
		http.NotFound(w, r)
		return nil
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		web.SetFlashError(h.sm, r.Context(), "Bitte einen Namen für die Ansicht angeben")
		return h.redirect(w, r, listURL(websiteID, r.FormValue("filter")))
	}
	if len(name) > 40 {
		name = name[:40]
	}

	values, err := url.ParseQuery(strings.TrimPrefix(r.FormValue("filter"), "?"))
	if err != nil {
		return err
	}
	query := filterQuery(values)
	if query == "" {
		web.SetFlashError(h.sm, r.Context(),
			"Diese Ansicht ist die ganze Liste — stelle erst einen Filter ein.")
		return h.redirect(w, r, listURL(websiteID, ""))
	}

	// The same name twice replaces the old one: somebody correcting a view
	// means to correct it, and a second chip with the same label helps nobody.
	if _, err := h.db.Write.ExecContext(r.Context(),
		`INSERT INTO saved_views (user_id, website_id, name, query) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, website_id, name) DO UPDATE SET query = excluded.query`,
		userID, websiteID, name, query); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Ansicht gemerkt")
	return h.redirect(w, r, listURL(websiteID, query))
}

// HandleSavedViewDelete forgets one view.
func (h *Handler) HandleSavedViewDelete(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	viewID, err := strconv.ParseInt(r.PathValue("viewID"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	userID := h.sm.GetInt64(r.Context(), auth.SessionKeyUserID)
	// The user id in the condition is the whole access check: a view belongs to
	// one person, and a number in an address must not reach anybody else's.
	if _, err := h.db.Write.ExecContext(r.Context(),
		`DELETE FROM saved_views WHERE id = $1 AND user_id = $2 AND website_id = $3`,
		viewID, userID, websiteID); err != nil {
		return err
	}
	web.SetFlashSuccess(h.sm, r.Context(), "Ansicht entfernt")
	return h.redirect(w, r, listURL(websiteID, ""))
}
