package public

import (
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/locale"
)

// mdEscape makes text safe as the text of a markdown link: one list item must
// stay one line, and brackets and parentheses must not end the link early.
func mdEscape(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`, "(", `\(`, ")", `\)`).Replace(s)
}

// HandleLLMS serves /llms.txt: a markdown overview of the website for language
// models, listing exactly the URLs /sitemap.xml lists.
func (h *Handler) HandleLLMS(w http.ResponseWriter, r *http.Request) error {
	website := domain.WebsiteFromContext(r.Context())
	if website == nil {
		http.NotFound(w, r)
		return nil
	}
	items, err := h.listing(r, website)
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# " + strings.Join(strings.Fields(website.Name), " ") + "\n")
	if d := strings.Join(strings.Fields(website.MetaDescription), " "); d != "" {
		b.WriteString("\n> " + d + "\n")
	}
	for _, tag := range website.AllLocales() {
		first := true
		for _, it := range items {
			if it.Locale != tag {
				continue
			}
			if first {
				b.WriteString("\n## " + strings.Join(strings.Fields(locale.Native(tag)), " ") + "\n\n")
				first = false
			}
			title := mdEscape(it.Title)
			if title == "" {
				title = it.Loc
			}
			b.WriteString("- [" + title + "](" + it.Loc + ")")
			if d := strings.Join(strings.Fields(it.Description), " "); d != "" {
				b.WriteString(": " + d)
			}
			b.WriteString("\n")
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(b.String()))
	return nil
}
