package admin

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The website form groups fields in <fieldset class="form-fieldset"> — the
// legal frameworks, the details about the business. Without a rule for the
// class the browser draws its own framed box and legend, which nothing else
// in the admin looks like (IN-01 of the 33.1 review). The stylesheet must
// style the class and its legend, and only with the admin's tokens.
func TestTheWebsiteFormFieldsetsAreStyled(t *testing.T) {
	form, err := os.ReadFile("../../cmd/holzcloud/templates/admin/website_form.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(form), `class="form-fieldset"`) {
		t.Skip("the website form has no form-fieldset any more")
	}
	raw, err := os.ReadFile("../../cmd/holzcloud/assets/admin.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(raw)
	for _, sel := range []string{".form-fieldset", ".form-fieldset > legend"} {
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(sel) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
		if rule == nil {
			t.Errorf("admin.css has no rule for %s", sel)
			continue
		}
		for _, literal := range regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|rgb\(|hsl\(`).FindAllString(rule[1], -1) {
			t.Errorf("%s uses the colour literal %q; the admin's colours are tokens", sel, literal)
		}
	}
	if m := regexp.MustCompile(`(?s)\.form-fieldset\s*\{([^}]*)\}`).FindStringSubmatch(css); m != nil &&
		!strings.Contains(m[1], "border: 0") {
		t.Error(".form-fieldset keeps the browser's frame")
	}
}
