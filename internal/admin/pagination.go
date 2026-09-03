package admin

import (
	"fmt"
	"net/http"
	"strconv"
)

// PagerTarget tells the shared pager template where its links point and which
// element htmx should swap.
type PagerTarget struct {
	URL      string
	Selector string
	// Query are the parameters that have to survive a page change — the
	// filters a list was narrowed by — already encoded and without "page".
	// Empty on a list that has nothing to carry.
	Query string
}

// Link is the address of one page of this list. Built here rather than in the
// template, because a template that glues "?" and "&" together gets it right
// until the day a list grows its first filter.
func (t PagerTarget) Link(page int) string {
	if t.Query == "" {
		return fmt.Sprintf("%s?page=%d", t.URL, page)
	}
	return fmt.Sprintf("%s?%s&page=%d", t.URL, t.Query, page)
}

// Pagination holds the derived numbers a list template needs to render its
// pager. Embed it in a page's data struct so the fields promote and the
// templates can use .Page, .HasNext and so on directly.
type Pagination struct {
	Page        int
	PerPage     int
	Total       int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
	PagerTarget PagerTarget
}

// WithTarget returns a copy of p pointing its pager links at url and swapping
// selector.
func (p Pagination) WithTarget(url, selector string) Pagination {
	p.PagerTarget = PagerTarget{URL: url, Selector: selector}
	return p
}

// WithFilteredTarget is WithTarget for a list whose filters must survive the
// pager. query is the encoded parameter string without "page".
func (p Pagination) WithFilteredTarget(url, selector, query string) Pagination {
	p.PagerTarget = PagerTarget{URL: url, Selector: selector, Query: query}
	return p
}

// NewPagination derives the pager state for a result set.
func NewPagination(page, perPage, total int) Pagination {
	if perPage <= 0 {
		perPage = 1
	}
	if page <= 0 {
		page = 1
	}

	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	return Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
	}
}

// pageParam reads the "page" query parameter, defaulting to 1. Values that are
// not a positive number are ignored rather than rejected — a bad page number in
// a URL should show the first page, not an error.
func pageParam(r *http.Request) int {
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			return n
		}
	}
	return 1
}
