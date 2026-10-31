package web

import (
	"context"
	"fmt"
	"strings"
)

// The narrow bar on the left holds seven places, not twenty-seven. Every screen
// still has its own ActiveNav; NavSection folds those into the place the bar
// marks, so a handler keeps naming its screen and nothing else has to change
// when a screen moves from one place to another.
//
// Menus have their own place: they are daily content work, not a setting.
// Only the place in the bar moved; no address changed, so nothing redirects.
//
// A screen missing from the list lands in settings. That is deliberate: a new
// screen is far more often a setting than a new kind of daily work, and a
// wrong guess shows up as the wrong mark in the bar, not as a missing screen.
var navSections = map[string]string{
	"dashboard":      "start",
	"pages":          "pages",
	"translations":   "pages",
	"trash":          "pages",
	"menus":          "menus",
	"media":          "media",
	"albums":         "media",
	"products":       "shop",
	"orders":         "shop",
	"shop":           "shop",
	"shop-settings":  "shop",
	"website-design": "design",
	"account":        "account",
}

// NavSection is the place in the bar this screen belongs to.
func (d LayoutData) NavSection() string {
	if strings.HasPrefix(d.ActiveNav, "plugin:") {
		return "plugin"
	}
	if s, ok := navSections[d.ActiveNav]; ok {
		return s
	}
	return "settings"
}

// NavCounts are the small numbers on the bar: what is waiting in each place.
// Zero draws no badge — a bar full of noughts would teach everybody to ignore
// it.
type NavCounts struct {
	// Review is the pages marked as awaiting review.
	Review int
	// MissingAlt is the images without a description. Shown in the warning
	// colour: it is a defect, not work that came in.
	MissingAlt int
	// NewOrders is the orders nobody has touched yet.
	NewOrders int
}

var navCounter func(ctx context.Context, websiteID int64) NavCounts

// SetNavCounter installs what fills the badges. Called once at startup, like
// SetBuild; without it the bar simply carries no numbers.
func SetNavCounter(f func(ctx context.Context, websiteID int64) NavCounts) {
	navCounter = f
}

// PluginScreenURL is the address of a plugin's own admin screen. The bar's link
// and the handler that answers it both take it from here: they used to spell it
// twice, and the handler's copy drifted into marking the wrong place.
// websiteID 0 is a screen beside the websites, not under one.
func PluginScreenURL(pluginID string, websiteID int64) string {
	if websiteID > 0 {
		return fmt.Sprintf("/admin/websites/%d/plugins/%s", websiteID, pluginID)
	}
	return "/admin/plugins/" + pluginID + "/bildschirm"
}

// PluginNav is the ActiveNav of the screen at url. base.html compares it with
// "plugin:" plus the link's address to light the plugin's own entry.
func PluginNav(url string) string { return "plugin:" + url }
