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

// A plugin's screen lights the plugin's own entry in the bar. It used to name
// itself "plugins", which folds into settings: the installation list opened with
// "Plugins" marked whoever had clicked "Messages".
func TestAPluginScreenMarksItsOwnEntryNotSettings(t *testing.T) {
	for _, websiteID := range []int64{0, 7} {
		url := PluginScreenURL("kontaktformular", websiteID)
		if got := (LayoutData{ActiveNav: PluginNav(url)}).NavSection(); got != "plugin" {
			t.Errorf("website %d: NavSection = %q, want plugin", websiteID, got)
		}
	}
	if got := PluginScreenURL("kontaktformular", 7); got != "/admin/websites/7/plugins/kontaktformular" {
		t.Errorf("per-website address = %q", got)
	}
	if got := PluginScreenURL("kontaktformular", 0); got != "/admin/plugins/kontaktformular/bildschirm" {
		t.Errorf("global address = %q", got)
	}
}
