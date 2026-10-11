package web

import "testing"

// Menus are daily content work and have their own place in the bar; the
// settings screens still fold into settings.
func TestMenusHaveTheirOwnPlaceInTheBar(t *testing.T) {
	for nav, want := range map[string]string{
		"menus": "menus", "pages": "pages", "media": "media",
		"terms": "settings", "seo": "settings", "": "settings", "plugin:/x": "plugin",
	} {
		if got := (LayoutData{ActiveNav: nav}).NavSection(); got != want {
			t.Errorf("NavSection(%q) = %q, want %q", nav, got, want)
		}
	}
}
