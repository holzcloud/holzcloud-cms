package dsr

import (
	"context"
)

// pluginSources are the plugins that keep data for the website.
//
// A plugin keeps its data in plugin_store, as JSON only it can read, so only
// the plugin can say which entries belong to an address. Every plugin with
// entries for the website comes back as a Holder, and a plugin that cannot be
// asked is named with its entry count rather than skipped: the screen must
// not look complete while a contact form's messages were never searched.
//
// No plugin answers the privacy hook yet, so this returns no source and
// every holder uncovered.
func (s *Service) pluginSources(ctx context.Context, websiteID int64) ([]Source, []Holder, error) {
	rows, err := s.db.Read.QueryContext(ctx, `SELECT plugin_id, COUNT(*) FROM plugin_store
		WHERE website_id = $1 GROUP BY plugin_id ORDER BY plugin_id`, websiteID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var holders []Holder
	for rows.Next() {
		var h Holder
		if err := rows.Scan(&h.PluginID, &h.Entries); err != nil {
			return nil, nil, err
		}
		h.Name = h.PluginID
		holders = append(holders, h)
	}
	return nil, holders, rows.Err()
}
