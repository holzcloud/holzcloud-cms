package dsr

// The machine-readable answer to an access request (D-02): one JSON file
// holding every record the search found, the plugins it could not ask, and
// the sources that failed. Keys are English and stable; values are the data
// as stored.
//
// This is where the export's shape is decided, and with it what never enters
// it. No source reads a password hash, a second-factor secret, a token hash, a
// recovery code or an order's confirmation token, so none of them can reach a
// Record and none can reach this file. They are credentials, not the
// person's data in any useful sense: handing one over would let whoever holds
// the file act as the account or open the order, and the right of access
// stops where it would harm the rights of others (GDPR Art. 15(4)).

import (
	"time"
)

// Export is the file.
type Export struct {
	GeneratedAt string                    `json:"generated_at"`
	Website     ExportWebsite             `json:"website"`
	Email       string                    `json:"email"`
	Records     map[string][]ExportRecord `json:"records"`
	NotCovered  []ExportHolder            `json:"not_covered"`
	Incomplete  []ExportFailure           `json:"incomplete"`
}

// ExportWebsite names the website the export is about.
type ExportWebsite struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ExportRecord is one record.
type ExportRecord struct {
	Key    string            `json:"key"`
	Kind   string            `json:"kind"`
	Time   string            `json:"time"`
	Fields map[string]string `json:"fields"`
}

// ExportHolder is a plugin that keeps data for the website and was not
// asked.
type ExportHolder struct {
	Plugin  string `json:"plugin"`
	Name    string `json:"name"`
	Entries int    `json:"entries"`
	Reason  string `json:"reason"`
}

// ExportFailure is a source that could not be searched. The error text is
// the program's own, never a value from a record.
type ExportFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

// BuildExport turns a search result into the file. Every list is empty
// rather than null, so the file always has the same keys.
func BuildExport(res *Result, websiteID int64, websiteName string, now time.Time) Export {
	out := Export{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Website:     ExportWebsite{ID: websiteID, Name: websiteName},
		Email:       res.Email,
		Records:     map[string][]ExportRecord{},
		NotCovered:  []ExportHolder{},
		Incomplete:  []ExportFailure{},
	}
	for _, rec := range res.Records {
		fields := make(map[string]string, len(rec.Fields))
		for _, f := range rec.Fields {
			fields[f.Name] = f.Value
		}
		var when string
		if !rec.Time.IsZero() {
			when = rec.Time.UTC().Format(time.RFC3339)
		}
		out.Records[rec.Source] = append(out.Records[rec.Source], ExportRecord{
			Key: rec.Key, Kind: rec.Kind, Time: when, Fields: fields,
		})
	}
	for _, h := range res.NotCovered() {
		out.NotCovered = append(out.NotCovered, ExportHolder{
			Plugin: h.PluginID, Name: h.Name, Entries: h.Entries, Reason: h.Reason,
		})
	}
	for _, f := range res.Failed {
		out.Incomplete = append(out.Incomplete, ExportFailure{Source: f.Source, Error: f.Err.Error()})
	}
	return out
}
