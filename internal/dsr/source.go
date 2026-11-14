package dsr

// The privacy search: everything one website holds about one e-mail address
// (Phase 33.2, DSR-01, D-01).
//
// The answer is put together from a list of sources, one per store that keeps
// personal data — orders, the legacy contact-form table, staff accounts,
// privacy requests, queued mail, and the plugins. Every feature of the phase
// walks the same list: the search and its export here, and later erasure,
// correction, retention and the record of processing. A store added to the
// list is covered by all of them at once; a store missing from it is missing
// everywhere, which is a mistake one sees, rather than covered in one place and
// forgotten in another, which is a mistake nobody sees.
//
// An incomplete answer says so. A source that fails is named in
// Result.Failed and the walk goes on; a plugin that keeps data for the website
// and cannot be asked is named in Result.Holders. An answer that silently
// leaves something out looks complete, and an operator who hands it to the
// person who asked has given a wrong answer without knowing it.

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/db"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// Field is one value of a record.
type Field struct {
	// Name is the stable English key the JSON export uses ("name", "city").
	Name string
	// Label is what the screen shows: a catalogue key for the core sources,
	// the plugin's own translated label for a plugin.
	Label string
	// Value is the data as stored, formatted for reading where it is an
	// amount or a time.
	Value string
}

// Record is one thing a store holds about the person: an order, a message,
// an account, a request, a mail.
type Record struct {
	// Source is the store: "orders", "form_messages", "accounts",
	// "requests", "mail" or "plugin:<id>".
	Source string
	// Key is unique inside its source: an order number, a row id, a plugin
	// key.
	Key string
	// Kind is the retention kind the record belongs to: "order",
	// "form_submission", "account", "request", "mail".
	Kind string
	// Label names the record on the screen, a catalogue key for the core
	// sources.
	Label string
	// Time is when the record came into being.
	Time   time.Time
	Fields []Field
	// Link is the admin address where the record lives, "" when there is
	// none.
	Link string
	// MediaIDs are files that belong to the record, such as the attachments
	// of a form submission. Erasure removes them with it.
	MediaIDs []int64
	// Correctable names the fields the operator may correct.
	Correctable []string
	// Erased is true when the record has already been reduced to what the
	// law asks to keep.
	Erased bool
}

// Holder is a plugin that keeps data for the website in plugin_store.
type Holder struct {
	PluginID string
	// Name is the plugin's name, or its id when the plugin is not known.
	Name string
	// Entries is how many plugin_store rows the plugin keeps for the
	// website — all of them, not only the person's.
	Entries int
	// Covered is true when the plugin answers the privacy hook and was
	// asked.
	Covered bool
	// Reason is a catalogue key saying why the plugin was not asked, "" when
	// it was.
	Reason string
}

// SourceError is a source that could not be searched.
type SourceError struct {
	Source string
	Err    error
}

// Result is the answer to one search.
type Result struct {
	WebsiteID int64
	// Email is the address as searched, trimmed.
	Email   string
	Records []Record
	Holders []Holder
	Failed  []SourceError
}

// BySource groups the records by their source.
func (r *Result) BySource() map[string][]Record {
	out := map[string][]Record{}
	for _, rec := range r.Records {
		out[rec.Source] = append(out[rec.Source], rec)
	}
	return out
}

// NotCovered is the plugins that keep data for the website and were not
// asked.
func (r *Result) NotCovered() []Holder {
	var out []Holder
	for _, h := range r.Holders {
		if !h.Covered {
			out = append(out, h)
		}
	}
	return out
}

// Source is one store that may hold personal data.
type Source interface {
	// Name is the Record.Source of every record the source returns.
	Name() string
	// Find returns the records of one website that belong to the address,
	// in any letter case. It never returns a record of another website.
	Find(ctx context.Context, websiteID int64, email string) ([]Record, error)
}

// Service finds a person's data across the stores of a website.
type Service struct {
	db *db.DB
	// plugins may be nil: then no plugin can be asked, and every plugin that
	// keeps data is reported as not covered.
	plugins *plugin.Manager
	// extra are sources a test adds after the core ones.
	extra []Source
	// media and dataDir delete the files of records the retention job
	// removes. Nil means no media store: a plugin that names a file then
	// fails its kind instead of leaving the file behind unreported.
	media   *media.Store
	dataDir string
	// failKind lets a test break one kind of one website in the retention
	// job; nil everywhere else.
	failKind func(websiteID int64, kind string) error
}

// NewService builds the search over the database.
func NewService(database *db.DB) *Service { return &Service{db: database} }

// SetPlugins hands the service the plugin manager. Nil, or never called,
// means the plugins are not running on this server.
func (s *Service) SetPlugins(m *plugin.Manager) { s.plugins = m }

// SetMedia hands the service the media store and the data directory the
// files live under, so the retention job can delete the attachments of what
// it expires.
func (s *Service) SetMedia(m *media.Store, dataDir string) { s.media, s.dataDir = m, dataDir }

// Find searches every source of the website for the address.
//
// The address is trimmed and must be one bare address (ValidEmail); anything
// else is ErrBadEmail before any store is asked. A source that fails is
// recorded in Result.Failed and the walk goes on.
func (s *Service) Find(ctx context.Context, websiteID int64, email string) (*Result, error) {
	email = strings.TrimSpace(email)
	if !ValidEmail(email) {
		return nil, ErrBadEmail
	}
	res := &Result{WebsiteID: websiteID, Email: email}
	sources := append(s.coreSources(), s.extra...)
	plugins, holders, err := s.pluginSources(ctx, websiteID)
	if err != nil {
		res.Failed = append(res.Failed, SourceError{Source: "plugins", Err: err})
	}
	res.Holders = holders
	sources = append(sources, plugins...)
	for _, src := range sources {
		recs, err := src.Find(ctx, websiteID, email)
		if err != nil {
			res.Failed = append(res.Failed, SourceError{Source: src.Name(), Err: err})
			continue
		}
		sort.SliceStable(recs, func(i, j int) bool { return recs[i].Time.Before(recs[j].Time) })
		res.Records = append(res.Records, recs...)
	}
	return res, nil
}
