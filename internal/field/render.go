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

// Term is one of the website's labels, as a theme uses it.
type Term struct {
	// Name is the label's name as it is right now, not as it was when
	// somebody chose it. That sentence is the whole point of the kind: a page
	// stores the label's address and prints its current name, so renaming the
	// label changes every page that carries it without any page being touched.
	Name string
	// Slug is the label's address, which is what the page actually stores.
	Slug string
	// URL is the label's archive address, with the language prefix already on
	// it.
	URL string
}

// TermLookup resolves a label's slug for the website being rendered.
//
// It is where the website rule lives, and it is the only place it can live: a
// label belongs to exactly one website, a stored slug says nothing about which,
// and a slug from another website must not resolve. The lookup knows the
// website; the stored value never will.
type TermLookup func(slug string) (Term, bool)

// Links are the lookups Resolve needs to turn stored ids into things a theme
// can use. A nil one means that kind resolves to nothing, which is what a
// caller without a media library or a page store wants.
type Links struct {
	Image Lookup
	Page  RefLookup
	Term  TermLookup
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

		case KindNumber, KindRange:
			// Ein Bereichsfeld ist eine Zahl mit Grenzen, und die Grenzen sind
			// eine Frage der Prüfung, nicht der Auflösung. Raw bleibt die
			// getippte Zeichenkette, damit 0.1 als 0.1 gedruckt wird und nicht
			// als das, was ein float64 daraus zurückformatiert.
			n := Number{Raw: raw}
			if raw != "" {
				n.Value, _ = ParseNumber(raw)
			}
			out[d.Key] = n

		case KindTime:
			// Ein Zeiger, aus demselben Grund wie beim Datum: „nichts
			// eingetragen“ und „Mitternacht“ sind zwei verschiedene Tatsachen,
			// und ein time.Time könnte sie nicht auseinanderhalten.
			t, ok := ParseTimeOfDay(raw)
			if !ok {
				out[d.Key] = (*time.Time)(nil)
				continue
			}
			out[d.Key] = &t

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

		case KindTerm:
			// Kein ParseInt: der gespeicherte Wert *ist* die Identität, also
			// geht das Kürzel unverändert in die Nachschlagefunktion. Der
			// Vergleich dort ist genaue Zeichengleichheit — ein Kürzel ist
			// bereits die kleingeschriebene Schreibweise eines Namens, und
			// hier noch einmal zu falten liesse zwei verschiedene
			// Schlagwörter zusammenfallen.
			if raw == "" || links.Term == nil {
				out[d.Key] = (*Term)(nil)
				continue
			}
			t, ok := links.Term(raw)
			if !ok {
				// Gelöscht, oder von einer anderen Website. Nil statt eines
				// alten Namens: ein {{ with }} im Theme lässt den Block dann
				// aus, statt eine Beschriftung zu drucken, die es nicht mehr
				// gibt.
				out[d.Key] = (*Term)(nil)
				continue
			}
			out[d.Key] = &t

		case KindMulti:
			// A slice, always — a theme loops over it with {{range}}, and an
			// empty one simply loops zero times. SplitValues returns nil for
			// an empty string, which ranges the same way but would make a
			// theme's {{if}} read differently, so it is normalised here.
			values := SplitValues(raw)
			if values == nil {
				values = []string{}
			}
			out[d.Key] = values

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
	// Term is set for a label field whose label is still there.
	Term *Term
	// Values are the picked values of a multi-valued field, nil for every
	// other kind. Text carries the same values joined for reading, so a theme
	// that prints label-and-value pairs without knowing the kinds still gets a
	// sentence rather than a blob.
	Values []string
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
		case []string:
			if len(v) == 0 {
				continue
			}
			e.Values = v
			e.Text = strings.Join(v, ", ")
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
			// Ein Datum überlässt der Text dem formatDate des Themes. Eine
			// Uhrzeit hat keinen solchen Helfer, also steht sie hier — sonst
			// druckt eine Liste aus Beschriftung und Wert neben „Abfahrt“
			// nichts.
			if d.Kind == KindTime {
				// Aus dem gelesenen Zeitpunkt und nicht aus der gespeicherten
				// Zeichenkette. ParseTimeOfDay nimmt „09:30:00" absichtlich an —
				// manche Browser schicken die Sekunden mit —, und die
				// Spezifikation verspricht dem Theme „.Text ist sie als HH:MM".
				// Wer die Rohform durchreichte, brach dieses Versprechen für
				// jeden Wert, den ein Formular oder eine Tabelle in der langen
				// Form abgeliefert hat.
				e.Text = v.Format("15:04")
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
		case *Term:
			if v == nil {
				continue
			}
			e.Term = v
			// Der Name und nicht das Kürzel: FIELD-03 verlangt genau das,
			// und eine Liste aus Beschriftung und Wert soll „Möbelbau“
			// zeigen und nicht „moebel“.
			e.Text = v.Name
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
		case []string:
			// Without this a theme's whole field panel disappears on a page
			// that carries nothing but multi-valued fields.
			if len(t) > 0 {
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
		case *Term:
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
