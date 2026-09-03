package page

import (
	"context"
	"fmt"
	"html/template"
	"strings"
)

// SearchResult is one hit, with the snippet that shows why it matched.
type SearchResult struct {
	Page Page
	// Snippet is the matching passage with the search terms wrapped in <mark>.
	// It is built from escaped text, never from stored HTML.
	Snippet template.HTML
}

// Sentinels mark the highlighted range inside the snippet FTS5 returns.
//
// FTS5 inserts these literally into the text, so they must be characters no
// document can contain: STX and ETX are control codes that cannot appear in
// Markdown a person typed. Using "<mark>" directly would mean escaping the
// snippet afterwards would destroy the tags — and not escaping it would let a
// page title become script in the admin's browser.
const (
	markStart = "\x02"
	markEnd   = "\x03"
)

// SearchPages runs a full-text query over the live pages of one website.
//
// admin decides whether drafts are included: the admin list searches
// everything, the public search only what a visitor may see.
func (s *Store) SearchPages(ctx context.Context, websiteID int64, query string, includeDrafts bool, limit int) ([]SearchResult, error) {
	match := FTSQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	visibility := ListablePredicateFor("p")
	if includeDrafts {
		visibility = LivePredicateFor("p")
	}

	// bm25 weights the title ten times the body, so a page called "Impressum"
	// outranks one that merely mentions the word.
	//
	// snippet() gets column -1, which lets FTS5 pick the column the match is
	// actually in. Pinning it to the body meant a title match produced a
	// snippet with nothing highlighted in it.
	rows, err := s.DB.Read.QueryContext(ctx,
		`SELECT `+prefixColumns("p")+`,
		        snippet(pages_fts, -1, '`+markStart+`', '`+markEnd+`', '…', 20)
		 FROM pages_fts
		 JOIN pages p ON p.id = pages_fts.rowid
		 WHERE pages_fts MATCH $1 AND p.website_id = $2`+visibility+`
		 ORDER BY bm25(pages_fts, 10.0, 1.0)
		 LIMIT $3`,
		match, websiteID, limit)
	if err != nil {
		return nil, fmt.Errorf("search pages: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var raw string
		p, err := scanPage(rows, &raw)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, SearchResult{Page: *p, Snippet: highlight(raw)})
	}
	return results, rows.Err()
}

// highlight escapes the snippet and only then turns the sentinels into <mark>.
//
// The order is the whole point: escaping first means a page whose text contains
// "<script>" cannot become script in the admin's browser, and the only tags in
// the result are the two this function put there.
func highlight(snippet string) template.HTML {
	escaped := template.HTMLEscapeString(snippet)
	escaped = strings.ReplaceAll(escaped, markStart, "<mark>")
	escaped = strings.ReplaceAll(escaped, markEnd, "</mark>")
	return template.HTML(escaped)
}

// FTSQuery turns what a person typed into a safe FTS5 query.
//
// Passing user text to MATCH unquoted is a syntax-error vector at best: a stray
// quote, a bare "AND" or a "*" makes SQLite reject the whole statement, so a
// search for O"Brien would return a 500 rather than no results. Every term is
// wrapped in double quotes (with embedded quotes doubled), which makes it a
// literal phrase, and a trailing * on the last term gives prefix matching while
// someone is still typing.
func FTSQuery(input string) string {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(fields))
	for i, f := range fields {
		f = strings.ReplaceAll(f, `"`, `""`)
		term := `"` + f + `"`
		if i == len(fields)-1 {
			term += "*"
		}
		quoted = append(quoted, term)
	}
	// Terms are ANDed: typing more words should narrow, not widen.
	return strings.Join(quoted, " AND ")
}

// prefixColumns qualifies the page projection with a table alias.
func prefixColumns(alias string) string {
	parts := strings.Split(pageColumns, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}
