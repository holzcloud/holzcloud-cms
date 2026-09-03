package field

import (
	"strconv"
	"strings"
	"time"
)

// What a theme sees.
//
// The stored value is always a string — that is what keeps the storage simple.
// A template should not have to know that: `{{ if .Page.Felder.verfuegbar }}`
// has to work, and `{{ .Page.Felder.preis }}` has to print 8.50 and not "8.5"
// with a comma in the wrong place. So the values are turned into the types
// they mean, once, on the way out.

// Image is a picture as a theme uses it.
type Image struct {
	URL    string
	Alt    string
	Width  int
	Height int
	// Focus is the CSS object-position of the picture's focal point, so a theme
	// that crops keeps the important part.
	Focus string
}

// Lookup resolves a media id for the website being rendered.
type Lookup func(id int64) (Image, bool)

// Ref is another page of this website, as a theme uses it.
//
// Enough to make a link and nothing more: a reference that carried the whole
// target page would make one page's rendering depend on another's, and a
// theme that prints {{.Titel}} and {{.Adresse}} covers what a reference is for.
type Ref struct {
	// Title is the target's title as it is right now, not as it was when
	// somebody chose it.
	Title string
	// URL is the address to link to, with the language prefix already on it.
	URL string
	// Kind is "page" or "post", for a theme that marks them differently.
	Kind string
}

// RefLookup resolves a page id for the website being rendered.
//
// It is where the two rules live that a stored id cannot enforce by itself:
// the page has to belong to this website, and — on the public site — it has to
// be published. A draft that somebody referenced must not become visible
// through the reference.
type RefLookup func(id int64) (Ref, bool)

// Links are the lookups Resolve needs to turn stored ids into things a theme
// can use. A nil one means that kind resolves to nothing, which is what a
// caller without a media library or a page store wants.
type Links struct {
	Image Lookup
	Page  RefLookup
}

// Number is a number as a theme uses it. It carries the raw value too, so a
// price can be printed exactly as it was typed.
type Number struct {
	Value float64
	Raw   string
}

// String prints the number the way it was entered.
func (n Number) String() string { return n.Raw }

// Resolve turns stored values into what a template can use.
//
// Every defined field appears in the result, filled or not: a template that
// writes {{ .Page.Felder.preis }} must not fail on a page where nobody entered
// one. An empty value is the zero of its kind — empty string, false, zero
// number, zero time, nil picture.
func Resolve(defs []Def, data Data, links Links) map[string]any {
	// A field whose condition is not met has no value that applies. The stored
	// one stays in the database for the day the condition is met again; until
	// then the theme sees the empty value of its kind — the same thing it sees
	// for a field nobody has filled in.
	data = Effective(defs, data)

	out := make(map[string]any, len(defs))
	for _, d := range defs {
		// A heading is not a value and does not belong in the map a theme
		// looks things up in.
		if !d.HoldsValue() {
			continue
		}
		if d.IsGroup() {
			rows := data.Rows[d.Key]
			resolved := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				resolved = append(resolved, Resolve(d.Sub, Data{Values: row}, links))
			}
			out[d.Key] = resolved
			continue
		}
		raw := strings.TrimSpace(data.Values[d.Key])
		switch d.Kind {
		case KindBool:
			out[d.Key] = raw != "" && raw != "0"

		case KindNumber:
			n := Number{Raw: raw}
			if raw != "" {
				n.Value, _ = strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
			}
			out[d.Key] = n

		case KindDate:
			// A pointer, because that is what the themes' formatDate takes —
			// and because "no date" and "the first of January year one" are
			// different things a template has to be able to tell apart.
			t, err := time.Parse("2006-01-02", raw)
			if err != nil {
				out[d.Key] = (*time.Time)(nil)
				continue
			}
			out[d.Key] = &t

		case KindImage:
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || links.Image == nil {
				out[d.Key] = (*Image)(nil)
				continue
			}
			img, ok := links.Image(id)
			if !ok {
				// A picture that was deleted after it was chosen. Nil rather
				// than a broken <img>: a theme's {{ with }} then simply leaves
				// the block out.
				out[d.Key] = (*Image)(nil)
				continue
			}
			out[d.Key] = &img

		case KindRef:
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || links.Page == nil {
				out[d.Key] = (*Ref)(nil)
				continue
			}
			ref, ok := links.Page(id)
			if !ok {
				// Deleted, moved to another website, or still a draft on the
				// public site. Nil, so a theme's {{ with }} leaves the link
				// out rather than pointing at a page that is not there.
				out[d.Key] = (*Ref)(nil)
				continue
			}
			out[d.Key] = &ref

		default:
			out[d.Key] = raw
		}
	}
	return out
}

// Entry is one field with its label, for a theme that wants to print whatever
// the operator defined without knowing the names.
//
// This is what makes the feature useful in a theme that ships with the
// program: a theme cannot know that this website calls something "Preis pro
// Kilo", but it can print a list of label-and-value pairs.
type Entry struct {
	Key   string
	Label string
	Kind  string
	// Value is the typed value, for a theme that wants to format it itself.
	Value any
	// Text is the value ready to print, empty when nothing was entered. Dates
	// are left to the theme's formatDate, so this is empty for them.
	Text string
	// Image is set for a picture field, nil otherwise.
	Image *Image
	// Ref is set for a reference field whose target is still there.
	Ref *Ref
	// Yes is the state of a yes/no field.
	Yes bool
	// Rows are a group's filled rows, each already turned into entries with
	// their own labels — enough for a theme to print a table it has never seen.
	Rows [][]Entry
}

// List returns the filled fields in their defined order.
//
// Empty ones are left out: a list of labels with nothing beside them tells a
// reader less than no list at all.
func List(defs []Def, data Data, links Links) []Entry {
	resolved := Resolve(defs, data, links)
	data = Effective(defs, data)
	out := make([]Entry, 0, len(defs))
	for _, d := range defs {
		if !d.HoldsValue() {
			continue
		}
		if d.IsGroup() {
			rows := data.Rows[d.Key]
			if len(rows) == 0 {
				continue
			}
			e := Entry{Key: d.Key, Label: d.Label, Kind: d.Kind, Value: resolved[d.Key]}
			for _, row := range rows {
				e.Rows = append(e.Rows, List(d.Sub, Data{Values: row}, links))
			}
			out = append(out, e)
			continue
		}
		e := Entry{Key: d.Key, Label: d.Label, Kind: d.Kind, Value: resolved[d.Key]}
		switch v := resolved[d.Key].(type) {
		case string:
			if v == "" {
				continue
			}
			e.Text = v
		case Number:
			if v.Raw == "" {
				continue
			}
			e.Text = v.Raw
		case bool:
			if !v {
				continue
			}
			e.Yes = true
		case *time.Time:
			if v == nil {
				continue
			}
		case *Image:
			if v == nil {
				continue
			}
			e.Image = v
		case *Ref:
			if v == nil {
				continue
			}
			e.Ref = v
			e.Text = v.Title
		}
		out = append(out, e)
	}
	return out
}

// Filled reports whether a page has anything in its extra fields. Themes use
// it to leave out a whole panel rather than print an empty one.
func Filled(resolved map[string]any) bool {
	for _, v := range resolved {
		switch t := v.(type) {
		case string:
			if t != "" {
				return true
			}
		case bool:
			if t {
				return true
			}
		case Number:
			if t.Raw != "" {
				return true
			}
		case *time.Time:
			if t != nil {
				return true
			}
		case *Image:
			if t != nil {
				return true
			}
		case *Ref:
			if t != nil {
				return true
			}
		case []map[string]any:
			if len(t) > 0 {
				return true
			}
		}
	}
	return false
}
