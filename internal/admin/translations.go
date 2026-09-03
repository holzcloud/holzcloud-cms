package admin

import (
	"net/http"
	"strconv"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// TranslationColumn is one language of the overview.
type TranslationColumn struct {
	// Code is the stored tag. Empty for the main language, which is how the
	// column is written in the database.
	Code string
	// Tag is the same thing but always filled, for lang and hreflang.
	Tag     string
	Name    string
	Primary bool
	// Missing counts the pages that do not exist in this language. It is the
	// number the screen is actually for.
	Missing int
}

// TranslationCellView is one square of the grid.
type TranslationCellView struct {
	Exists bool
	ID     int64
	Title  string
	Status string
	// NewURL is where the button goes when the square is empty.
	NewURL string
	// EditURL is where the link goes when it is not.
	EditURL string
}

// TranslationRowView is one page across all languages.
type TranslationRowView struct {
	Title string
	Slug  string
	Cells []TranslationCellView
}

// TranslationsData is the overview of what is translated and what is not.
type TranslationsData struct {
	web.LayoutData
	WebsiteID int64
	Columns   []TranslationColumn
	Rows      []TranslationRowView
	// Complete is true when nothing is missing. Worth saying plainly rather
	// than leaving the reader to scan a grid for gaps that are not there.
	Complete bool
}

// HandleTranslations shows which pages exist in which language.
//
// The languages are the website's own list, so a site with one language has
// one column and the screen says so instead of pretending to be a matrix.
func (h *Handler) HandleTranslations(w http.ResponseWriter, r *http.Request) error {
	websiteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	ws, err := h.domains.GetWebsite(r.Context(), websiteID)
	if err != nil {
		return err
	}
	if ws == nil {
		http.NotFound(w, r)
		return nil
	}

	rows, err := h.pages.TranslationMatrix(r.Context(), websiteID)
	if err != nil {
		return err
	}

	columns := translationColumns(*ws)
	view := make([]TranslationRowView, 0, len(rows))
	for _, row := range rows {
		cells := make([]TranslationCellView, 0, len(columns))
		for i, col := range columns {
			cell, exists := row.ByLocale[col.Code]
			if !exists {
				columns[i].Missing++
				cells = append(cells, TranslationCellView{
					NewURL: newTranslationURL(websiteID, row.ID, col.Tag),
				})
				continue
			}
			cells = append(cells, TranslationCellView{
				Exists:  true,
				ID:      cell.ID,
				Title:   cell.Title,
				Status:  cell.Status,
				EditURL: pageEditURL(websiteID, cell.ID),
			})
		}
		view = append(view, TranslationRowView{Title: row.Title, Slug: row.Slug, Cells: cells})
	}

	complete := true
	for _, col := range columns {
		if col.Missing > 0 {
			complete = false
			break
		}
	}

	data := TranslationsData{
		LayoutData: web.NewLayoutData(r, h.sm, "Übersetzungen"),
		WebsiteID:  websiteID,
		Columns:    columns,
		Rows:       view,
		Complete:   complete && len(view) > 0,
	}
	data.ActiveNav = "translations"
	data.CurrentWebsite = ws
	return web.RenderAdmin(w, h.templates, r, "translation_matrix", data)
}

// translationColumns are the website's languages, main one first.
//
// Website.Locales returns the *additional* languages only — the main one is
// Website.Locale and is stored on a page as the empty string. So the first
// column is built separately, and it is the one column where Code and Tag
// differ: Code is what to look up, Tag is what to print.
func translationColumns(ws domain.Website) []TranslationColumn {
	extras := ws.Locales()
	columns := make([]TranslationColumn, 0, len(extras)+1)
	columns = append(columns, TranslationColumn{
		Code:    "",
		Tag:     ws.Locale,
		Name:    locale.Native(ws.Locale),
		Primary: true,
	})
	for _, tag := range extras {
		columns = append(columns, TranslationColumn{
			Code: tag,
			Tag:  tag,
			Name: locale.Native(tag),
		})
	}
	return columns
}

func newTranslationURL(websiteID, sourceID int64, tag string) string {
	return pagesPath(websiteID) + "/new?uebersetzung_von=" +
		strconv.FormatInt(sourceID, 10) + "&sprache=" + tag
}

func pageEditURL(websiteID, pageID int64) string {
	return pagesPath(websiteID) + "/" + strconv.FormatInt(pageID, 10) + "/edit"
}

func pagesPath(websiteID int64) string {
	return "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/pages"
}
