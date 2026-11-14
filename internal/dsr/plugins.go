package dsr

import (
	"context"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

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
// website (plugin.Manager.Serves), and each covered plugin becomes a source
// that asks it. For the others the reason says whether the plugin is running
// here at all.
func (s *Service) pluginSources(ctx context.Context, websiteID int64) ([]Source, []Holder, error) {
	rows, err := s.db.Read.QueryContext(ctx, `SELECT ps.plugin_id, COALESCE(p.name, ''), COUNT(*)
		FROM plugin_store ps LEFT JOIN plugins p ON p.id = ps.plugin_id
		WHERE ps.website_id = $1 GROUP BY ps.plugin_id ORDER BY ps.plugin_id`, websiteID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var holders []Holder
	var sources []Source
	for rows.Next() {
		var h Holder
		if err := rows.Scan(&h.PluginID, &h.Name, &h.Entries); err != nil {
			return nil, nil, err
		}
		if h.Name == "" {
			h.Name = h.PluginID
		}
		h.Covered = s.plugins.Serves(h.PluginID, plugin.HookPrivacy, websiteID)
		switch {
		case h.Covered:
			sources = append(sources, pluginSource{plugins: s.plugins, id: h.PluginID})
		case s.plugins.ActsOn(h.PluginID, websiteID):
			h.Reason = reasonNoHook
		default:
			h.Reason = reasonNotRunning
		}
		holders = append(holders, h)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows.Close()
	// The name the plugin list shows: the manifest's own translation in the
	// operator's language, for a plugin that is still installed. Asked after
	// the rows are closed, so the walk holds one read connection at a time.
	if s.plugins != nil {
		for i := range holders {
			st, err := s.plugins.Get(ctx, holders[i].PluginID)
			if err != nil || st.Manifest == nil {
				continue
			}
			if name := st.Manifest.NameIn(i18n.Lang(ctx)); name != "" {
				holders[i].Name = name
			}
		}
	}
	return sources, holders, nil
}

// pluginSource asks one plugin through its privacy hook.
//
// The host never reads a plugin's JSON: what the plugin answers is what the
// screen shows, under the plugin's own labels. An error — a plugin that
// failed, timed out, or tried to write during find — is the source's error,
// so the screen names the plugin as not searched instead of showing an
// answer that only looks complete.
type pluginSource struct {
	plugins *plugin.Manager
	id      string
}

func (p pluginSource) Name() string { return "plugin:" + p.id }

func (p pluginSource) Find(ctx context.Context, websiteID int64, email string) ([]Record, error) {
	out, err := p.plugins.Privacy(ctx, websiteID, p.id, plugin.PrivacyIn{Op: plugin.PrivacyFind, Email: email})
	if err != nil {
		return nil, err
	}
	link := "/admin/websites/" + strconv.FormatInt(websiteID, 10) + "/plugins/" + p.id
	recs := make([]Record, 0, len(out.Records))
	for _, r := range out.Records {
		rec := Record{
			Source:      p.Name(),
			Key:         r.Key,
			Kind:        r.Kind,
			Label:       r.Label,
			Link:        link,
			MediaIDs:    r.MediaIDs,
			Correctable: r.Correctable,
			Erased:      r.Erased,
		}
		// A time the plugin wrote in another shape is left zero rather than
		// guessed; the record still shows.
		if t, err := time.Parse(time.RFC3339, r.Time); err == nil {
			rec.Time = t
		}
		for _, f := range r.Fields {
			rec.Fields = append(rec.Fields, Field{Name: f.Name, Label: f.Label, Value: f.Value})
		}
		recs = append(recs, rec)
	}
	return recs, nil
}
