package dsr

import (
	"context"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// hookPrivacy is the hook a plugin declares to answer the privacy search.
// A literal until the plugin package declares the hook itself.
const hookPrivacy = "privacy"

// The reasons a plugin that keeps data was not asked, catalogue keys.
var (
	reasonNoHook     = i18n.N("The plugin does not answer the privacy hook, so its entries cannot be searched.")
	reasonNotRunning = i18n.N("The plugin is not running on this server.")
)

// pluginSources are the plugins that keep data for the website.
//
// A plugin keeps its data in plugin_store, as JSON only it can read, so only
// the plugin can say which entries belong to an address. Every plugin with
// entries for the website comes back as a Holder, and a plugin that cannot be
// asked is named with its entry count rather than skipped: the screen must
// not look complete while a contact form's messages were never searched.
// Rows of website 0 are a plugin's installation-wide settings, nobody's
// data on this website, and are not counted.
//
// A plugin is covered when the manager says it serves the privacy hook on the
// website (plugin.Manager.Serves). No plugin does yet, so this returns no
// source; the reason says whether the plugin is running here at all.
func (s *Service) pluginSources(ctx context.Context, websiteID int64) ([]Source, []Holder, error) {
	rows, err := s.db.Read.QueryContext(ctx, `SELECT ps.plugin_id, COALESCE(p.name, ''), COUNT(*)
		FROM plugin_store ps LEFT JOIN plugins p ON p.id = ps.plugin_id
		WHERE ps.website_id = $1 GROUP BY ps.plugin_id ORDER BY ps.plugin_id`, websiteID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var holders []Holder
	for rows.Next() {
		var h Holder
		if err := rows.Scan(&h.PluginID, &h.Name, &h.Entries); err != nil {
			return nil, nil, err
		}
		if h.Name == "" {
			h.Name = h.PluginID
		}
		h.Covered = s.plugins.Serves(h.PluginID, hookPrivacy, websiteID)
		switch {
		case h.Covered:
		case s.plugins.ActsOn(h.PluginID, websiteID):
			h.Reason = reasonNoHook
		default:
			h.Reason = reasonNotRunning
		}
		holders = append(holders, h)
	}
	return nil, holders, rows.Err()
}
