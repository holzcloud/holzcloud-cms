// Full-text search as a plugin.
//
// It answers /suche itself and lets the host render the result in the theme's
// own view. The plugin never sees the theme — it hands over a list of hits, and
// the header, menu, fonts and footer come from the website.
//
// Why this is not core: a website with eight pages does not need a search, and
// whoever does not offer one also has no page where somebody searches for
// something that is not there. Whoever wants it switches it on.
package main

import (
	"net/url"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// maxHits bounds a result list.
//
// Twenty is more than anybody reads. Somebody paging to the third screen is
// really looking for something else, and what helps there is a better search
// word rather than a longer list.
const maxHits = 20

// maxQuery bounds what is accepted as a search word. Anything longer is not a
// search word any more but something somebody is trying out.
const maxQuery = 200

func init() {
	plugin.OnRoute(search)
}

func search(in plugin.RequestIn) (plugin.RequestOut, error) {
	query := strings.TrimSpace(queryOf(in.Query))
	if len(query) > maxQuery {
		query = query[:maxQuery]
	}

	list := plugin.RenderSearch{Query: query, Submitted: query != ""}
	if query != "" {
		hits, err := plugin.SearchPages(query, maxHits)
		if err != nil {
			return plugin.RequestOut{}, err
		}
		for _, hit := range hits {
			list.Results = append(list.Results, plugin.RenderHit{
				Title:   hit.Title,
				URL:     "/" + hit.Slug,
				Snippet: hit.Snippet,
			})
		}
	}

	title := "Suche"
	if query != "" {
		title = "Suche: " + query
	}

	html, err := plugin.Render(plugin.RenderArg{
		Title: title,
		Slug:  "suche",
		View:  plugin.ViewSearch,
		// A list of hits is not content of its own and must not compete in a
		// search engine with the pages it points at.
		NoIndex: true,
		Search:  &list,
	})
	if err != nil {
		return plugin.RequestOut{}, err
	}

	return plugin.RequestOut{
		Handled: true,
		Status:  200,
		Body:    html,
		Headers: map[string]string{
			// The result hangs off the question and off content that can change
			// at any moment. There is nothing here worth keeping.
			"Cache-Control": "no-store",
			"X-Robots-Tag":  "noindex",
		},
	}, nil
}

// queryOf takes q out of the query string.
func queryOf(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	return values.Get("q")
}

func main() {}
