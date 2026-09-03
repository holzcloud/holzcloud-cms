// Package field lets an operator add their own fields to a website's pages.
//
// The fixed page — title, address, text, excerpt, preview image — is what a
// piece of prose needs. It is not what a product needs, or an event, or a
// breeding animal: those have a price, a date, an availability. Until now each
// of them meant a new version of the program.
//
// The shape is deliberately small. Eight kinds of input, one value per field,
// no nesting. Everything that a form can ask for in one line — and nothing
// that would need a second editor to fill in. What it buys is that the next
// content type is a screen in the admin rather than a release.
package field

import (
	"encoding/json"
	"fmt"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The kinds of input a field can be.
const (
	KindText   = "text"
	KindLong   = "langtext"
	KindNumber = "zahl"
	KindDate   = "datum"
	KindBool   = "janein"
	KindChoice = "auswahl"
	KindImage  = "bild"
	KindLink   = "link"
	// KindRef points at another page of this website — chosen from a list, not
	// typed. The difference from KindLink is what happens afterwards: a typed
	// address goes stale the moment the target is renamed, a reference follows
	// it, and when the target is deleted the theme sees nothing instead of a
	// link into the void.
	KindRef = "verweis"
	// KindGroup is a set of fields filled in more than once: opening hours,
	// team members, product variants. Its own value is the rows; what a row
	// contains are this field's children.
	KindGroup = "gruppe"
	// KindSection is a heading between the fields, and nothing else: no value,
	// no validation, nothing in the theme. Twenty fields in a row are a wall;
	// "Preis und Verfügbarkeit" over four of them is a form somebody fills in.
	KindSection = "abschnitt"
)

// Where a field applies.
const (
	ForBoth = "beides"
	ForPage = "seite"
	ForPost = "beitrag"
)

// Kind describes an input for the menu in the admin.
type Kind struct {
	Kind string
	Name string
	Hint string
}

// Kinds is the menu, in the order it is offered.
//
// The eight the contact form's builder already offers, minus the two that only
// make sense in a form someone fills in (e-mail, telephone) and plus the two a
// page needs and a form does not (picture, link).
var Kinds = []Kind{
	{KindText, i18n.N("Kurzer Text"), i18n.N("Eine Zeile: ein Name, eine Menge, eine Sorte.")},
	{KindLong, i18n.N("Langer Text"), i18n.N("Mehrere Zeilen ohne Formatierung.")},
	{KindNumber, i18n.N("Zahl"), i18n.N("Ein Preis, ein Gewicht, ein Jahrgang.")},
	{KindDate, i18n.N("Datum"), i18n.N("Ein Tag, ohne Uhrzeit.")},
	{KindBool, i18n.N("Ja/Nein"), i18n.N("Ein Ankreuzfeld: verfügbar, vergriffen.")},
	{KindChoice, i18n.N("Auswahl"), i18n.N("Eine Liste von Möglichkeiten, eine davon.")},
	{KindImage, i18n.N("Bild"), i18n.N("Ein Bild aus der Mediathek dieser Website.")},
	{KindLink, i18n.N("Link"), i18n.N("Eine eigene Seite oder eine fremde Adresse.")},
	{KindRef, i18n.N("Verweis"), i18n.N("Eine Seite dieser Website, ausgewählt statt eingetippt. Für mehrere: eine Gruppe mit einem Verweis darin.")},
	{KindGroup, i18n.N("Gruppe"), i18n.N("Mehrere Felder, mehrfach ausgefüllt — z. B. Öffnungszeiten.")},
	{KindSection, i18n.N("Abschnitt"), i18n.N("Keine Eingabe, sondern eine Überschrift über den folgenden Feldern.")},
}

// SubKinds are the kinds a field inside a group may have: everything except a
// group and a section. One level, on purpose — see the migration; and a
// heading inside a row that is repeated forty times is noise, not structure.
func SubKinds() []Kind {
	out := make([]Kind, 0, len(Kinds)-2)
	for _, k := range Kinds {
		if k.Kind != KindGroup && k.Kind != KindSection {
			out = append(out, k)
		}
	}
	return out
}

// BlockKinds are the kinds a field of a block kind may have.
//
// Everything a group may have, minus the reference. A reference exists to
// survive a rename — it stores the page, not its address, and follows it. A
// block is rendered to HTML when the page is saved and that HTML is what
// visitors get, so a reference inside one would freeze the address it had that
// day and quietly break on the next rename. A promise that cannot be kept here
// is better not offered: the link field does the same job and says what it is.
func BlockKinds() []Kind {
	out := make([]Kind, 0, len(Kinds)-3)
	for _, k := range Kinds {
		switch k.Kind {
		case KindGroup, KindSection, KindRef:
			continue
		}
		out = append(out, k)
	}
	return out
}

// blockKind reports whether a kind may be used inside a block kind.
func blockKind(kind string) bool {
	for _, k := range BlockKinds() {
		if k.Kind == kind {
			return true
		}
	}
	return false
}

// KindName is the label of a kind, or the raw value for one that no longer
// exists — a field defined by an older version must still be listable.
func KindName(kind string) string {
	for _, k := range Kinds {
		if k.Kind == kind {
			return k.Name
		}
	}
	return kind
}

// KnownKind reports whether a kind is one this version can render.
func KnownKind(kind string) bool {
	for _, k := range Kinds {
		if k.Kind == kind {
			return true
		}
	}
	return false
}

// MaxFields bounds how many fields one website may define, groups and their
// contents counted together.
//
// Not a technical limit but an editorial one: a form with sixty extra fields
// is a form nobody fills in correctly.
const MaxFields = 60

// MaxRows bounds how many times one group may be filled in on one page.
const MaxRows = 40

// MaxValueBytes bounds one stored value.
const MaxValueBytes = 4000

// Def is one field an operator has defined.
type Def struct {
	ID        int64
	WebsiteID int64
	Key       string
	Label     string
	Kind      string
	Required  bool
	Hint      string
	// Choices are the options of a choice field, one per line as stored.
	Choices []string
	// AppliesTo is ForBoth, ForPage or ForPost. Ignored on a field inside a
	// group, which is shown wherever its group is.
	AppliesTo string
	Position  int
	// ParentID names the group this field belongs to, or 0 for a field that
	// stands on the page itself.
	ParentID int64
	// BlockTypeID names the block kind this field belongs to, or 0 for a field
	// of the page. The two are separate worlds sharing one table — see
	// migration 00038 — so a page field and a block kind's field may carry the
	// same key without meeting.
	BlockTypeID int64
	// Sub are the fields of a group, in order. Empty for everything else.
	Sub []Def
	// Condition is the key of the field this one hangs on: it is asked for only
	// once that field is filled in. Empty for a field that is always shown.
	Condition string
}

// IsGroup reports whether this field holds rows rather than one value.
func (d Def) IsGroup() bool { return d.Kind == KindGroup }

// IsSection reports whether this is a heading rather than an input.
func (d Def) IsSection() bool { return d.Kind == KindSection }

// HoldsValue reports whether this field has something to store, check and hand
// to a theme. Everything but a heading does.
func (d Def) HoldsValue() bool { return d.Kind != KindSection }

// MayControl reports whether other fields may be made to depend on this one.
//
// Two limits, and both come from the browser rather than from taste. A
// dependent field is shown and hidden by CSS alone — no script, see admin.css
// — and CSS can only tell "filled in" from "empty" where the state is in the
// markup: a ticked box, a chosen option, a text field whose placeholder is no
// longer showing. A date input has none of those, and a group and a heading
// are not single values at all.
//
// The screen offers only the fields this returns true for, so nobody has to
// know the rule to avoid breaking it.
func (d Def) MayControl() bool {
	switch d.Kind {
	case KindGroup, KindSection, KindDate:
		return false
	}
	return d.ParentID == 0
}

// Applies reports whether this field belongs on an entry of the given kind.
//
// The kind is the *effective* one: "page", "post", or the key of one of the
// website's own kinds. A field can be tied to an own kind — a price belongs to
// a product and to nothing else — and "beides" keeps meaning everything.
func (d Def) Applies(pageKind string) bool {
	switch d.AppliesTo {
	case ForBoth, "":
		return true
	case ForPage:
		// The built-in page only. A product is technically a page, but a field
		// meant for pages on a website that has products would otherwise turn
		// up in the product form as well.
		return pageKind == "page" || pageKind == ""
	case ForPost:
		return pageKind == "post"
	default:
		return d.AppliesTo == pageKind
	}
}

// KindLabel is the human name of this field's kind, for the list in the admin.
func (d Def) KindLabel() string { return KindName(d.Kind) }

// Hidden returns the keys of the fields that are not being asked for, because
// the field they hang on is empty.
//
// "Filled in" is the whole of the condition: a ticked box, a chosen option,
// text in a text field. Comparing against a particular value would read better
// on paper and would need a stylesheet generated per website to keep the
// browser in step with the server — two places for one rule, which is one too
// many.
//
// A condition naming a field that is not there is no condition at all: the
// field is shown. The alternative is a field that can never be reached and
// gives no hint why.
func Hidden(defs []Def, values Values) map[string]bool {
	known := map[string]bool{}
	for _, d := range defs {
		if d.MayControl() {
			known[d.Key] = true
		}
	}
	hidden := map[string]bool{}
	// A chain — C hangs on B, B hangs on A — settles on the second pass, and
	// the fields are in no guaranteed order. Bounded by their number: every
	// pass but the last adds at least one.
	for range defs {
		grew := false
		for _, d := range defs {
			if d.Condition == "" || !known[d.Condition] || hidden[d.Key] {
				continue
			}
			v := strings.TrimSpace(values[d.Condition])
			if hidden[d.Condition] || v == "" || v == "0" {
				hidden[d.Key] = true
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	return hidden
}

// Effective is a page's values with the fields dropped whose condition is not
// met.
//
// One place where "is this field being asked for" is decided, so the form, the
// check, the theme and the export cannot each answer it slightly differently.
// Nothing is written back: the values stay in the database and come back the
// moment the condition is met again.
func Effective(defs []Def, d Data) Data {
	conditional := false
	for _, def := range defs {
		if def.Condition != "" {
			conditional = true
			break
		}
	}
	if !conditional {
		return d
	}

	hidden := Hidden(defs, d.Values)
	if len(hidden) == 0 {
		return d
	}
	out := Data{Values: make(Values, len(d.Values)), Rows: make(map[string][]Values, len(d.Rows))}
	for k, v := range d.Values {
		if !hidden[k] {
			out.Values[k] = v
		}
	}
	for k, v := range d.Rows {
		if !hidden[k] {
			out.Rows[k] = v
		}
	}
	return out
}

// ControlsOf maps each field key to the keys that hang on it, so a caller can
// render a dependent field where the browser can find it: inside its
// controller's group.
func ControlsOf(defs []Def) map[string][]Def {
	out := map[string][]Def{}
	known := map[string]bool{}
	for _, d := range defs {
		if d.MayControl() {
			known[d.Key] = true
		}
	}
	for _, d := range defs {
		if d.Condition != "" && known[d.Condition] {
			out[d.Condition] = append(out[d.Condition], d)
		}
	}
	return out
}

// FieldName is the name this field carries in the edit form.
//
// Prefixed so a field called "title" cannot collide with the page's own title,
// which would silently overwrite it.
func (d Def) FieldName() string { return "feld_" + d.Key }

// For returns the fields that belong on a page of the given kind.
func For(defs []Def, pageKind string) []Def {
	out := make([]Def, 0, len(defs))
	for _, d := range defs {
		if d.Applies(pageKind) {
			out = append(out, d)
		}
	}
	return out
}

// Values are a page's answers to the simple fields, keyed by field key.
type Values map[string]string

// Data is everything a page holds for its website's own fields.
//
// Two maps rather than one of `any`: a template, an export and a check all
// want to know which of the two they are looking at, and a map of `any` makes
// every one of them start with a type switch.
type Data struct {
	Values Values `json:"werte,omitempty"`
	// Rows are the filled-in rows of each group, keyed by the group's key.
	Rows map[string][]Values `json:"gruppen,omitempty"`
}

// Empty reports whether nothing at all is filled in.
func (d Data) Empty() bool { return len(d.Values) == 0 && len(d.Rows) == 0 }

// Row returns a group's rows, or none.
func (d Data) Row(key string) []Values { return d.Rows[key] }

// Decode reads the stored JSON.
//
// An unreadable value is treated as empty rather than as an error: a page whose
// extra fields cannot be parsed must still be editable, or the only way out
// would be the database.
//
// It also reads the flat shape that the first version of this feature wrote —
// a plain {"kennung": "wert"} object, before groups existed. Five lines here
// instead of a migration that rewrites every page's JSON, and an export written
// by that version still imports.
func Decode(raw string) Data {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Data{Values: Values{}, Rows: map[string][]Values{}}
	}

	var d Data
	if err := json.Unmarshal([]byte(raw), &d); err == nil && (d.Values != nil || d.Rows != nil) {
		if d.Values == nil {
			d.Values = Values{}
		}
		if d.Rows == nil {
			d.Rows = map[string][]Values{}
		}
		return d
	}

	// The old flat shape.
	var flat Values
	if err := json.Unmarshal([]byte(raw), &flat); err == nil && flat != nil {
		return Data{Values: flat, Rows: map[string][]Values{}}
	}
	return Data{Values: Values{}, Rows: map[string][]Values{}}
}

// Encode writes the values back. Nothing filled in stores as the empty string,
// so a page on a website without extra fields carries no JSON at all.
func Encode(d Data) (string, error) {
	if d.Empty() {
		return "", nil
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("felder sichern: %w", err)
	}
	return string(raw), nil
}

// Clean drops what no longer has a field and trims what is left.
//
// Deliberately not done when a field is deleted: keeping the value means a
// field deleted by accident can be defined again and everything is back. It is
// dropped here, on the next save of that page, which is late enough to be
// forgiving and early enough not to accumulate.
func Clean(defs []Def, d Data) Data {
	simple := map[string]bool{}
	groups := map[string][]Def{}
	for _, def := range defs {
		if def.IsGroup() {
			groups[def.Key] = def.Sub
			continue
		}
		if !def.HoldsValue() {
			continue
		}
		// A field whose condition is not met keeps its value. Unticking a box
		// hides the field; it must not also empty it, or a mis-click would cost
		// somebody the text they wrote — and ticking it again brings everything
		// back exactly as it was.
		simple[def.Key] = true
	}

	out := Data{Values: Values{}, Rows: map[string][]Values{}}
	for key, val := range d.Values {
		if !simple[key] {
			continue
		}
		if val = trimTo(val); val != "" {
			out.Values[key] = val
		}
	}
	for key, rows := range d.Rows {
		sub, ok := groups[key]
		if !ok {
			continue
		}
		kept := make([]Values, 0, len(rows))
		for _, row := range rows {
			if cleaned := cleanRow(sub, row); len(cleaned) > 0 {
				kept = append(kept, cleaned)
			}
			if len(kept) >= MaxRows {
				break
			}
		}
		if len(kept) > 0 {
			out.Rows[key] = kept
		}
	}
	return out
}

// cleanRow keeps one row's known, non-empty values. A row where everything was
// left blank disappears — that is what "remove this row" looks like to someone
// who cleared it instead of pressing the button.
func cleanRow(sub []Def, row Values) Values {
	out := Values{}
	for _, def := range sub {
		if val := trimTo(row[def.Key]); val != "" {
			out[def.Key] = val
		}
	}
	return out
}

func trimTo(val string) string {
	val = strings.TrimSpace(val)
	if len(val) > MaxValueBytes {
		val = val[:MaxValueBytes]
	}
	return val
}

// Check validates one value against its definition and returns a reason, or
// the empty string when it is fine.
//
// The reasons are written for the person filling the form in, not for a log:
// "Preis muss eine Zahl sein" and not "strconv.ParseFloat: invalid syntax".
func Check(d Def, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		if d.Required {
			return d.Label + " muss ausgefüllt werden."
		}
		return ""
	}

	switch d.Kind {
	case KindGroup:
		// A group is checked row by row, not as one value.
		return ""
	case KindNumber:
		if _, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64); err != nil {
			return d.Label + " muss eine Zahl sein."
		}
	case KindDate:
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return d.Label + " muss ein Datum sein."
		}
	case KindChoice:
		for _, c := range d.Choices {
			if c == value {
				return ""
			}
		}
		return d.Label + ": „" + value + "“ steht nicht zur Auswahl."
	case KindImage:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return d.Label + ": das ist kein Bild aus der Mediathek."
		}
	case KindRef:
		// That the page exists and belongs to this website is decided where
		// the pages are known — the editor only offers its own, and the theme
		// resolves through a lookup that checks. Here it is a number or it is
		// somebody typing into the form by hand.
		if id, err := strconv.ParseInt(value, 10, 64); err != nil || id <= 0 {
			return d.Label + ": das ist keine Seite dieser Website."
		}
	case KindLink:
		if reason := checkLink(value); reason != "" {
			return d.Label + ": " + reason
		}
	}
	return ""
}

// checkLink keeps a field from becoming a way to put javascript: into a theme.
//
// Only three shapes are allowed, and they are the three that mean something on
// a website: a path on this site, an http(s) address, and a mail address.
func checkLink(value string) string {
	switch {
	case strings.HasPrefix(value, "/"):
		if strings.HasPrefix(value, "//") {
			return "eine Adresse mit zwei Schrägstrichen führt auf einen fremden Server."
		}
		return ""
	case strings.HasPrefix(value, "https://"), strings.HasPrefix(value, "http://"),
		strings.HasPrefix(value, "mailto:"), strings.HasPrefix(value, "tel:"):
		return ""
	}
	return "das muss mit / beginnen (eigene Seite) oder mit https:// (fremde Adresse)."
}

// SlugifyKey turns a label into a key.
func SlugifyKey(label string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(label)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == 'ä':
			b.WriteString("ae")
			prevDash = false
		case r == 'ö':
			b.WriteString("oe")
			prevDash = false
		case r == 'ü':
			b.WriteString("ue")
			prevDash = false
		case r == 'ß':
			b.WriteString("ss")
			prevDash = false
		case unicode.IsSpace(r), r == '-', r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteRune('_')
				prevDash = true
			}
		}
	}
	key := strings.Trim(b.String(), "_")
	// A key has to be usable as a Go template field lookup, so it must not
	// start with a digit.
	if key != "" && key[0] >= '0' && key[0] <= '9' {
		key = "f" + key
	}
	if len(key) > 40 {
		key = strings.Trim(key[:40], "_")
	}
	return key
}

// SplitChoices reads the options as the admin types them: one per line.
func SplitChoices(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// JoinChoices writes them back for the textarea.
func JoinChoices(choices []string) string { return strings.Join(choices, "\n") }

// CheckAll validates a page's answers against the fields that apply to it and
// returns the reasons, keyed by the form field they belong to.
//
// One function for the admin form, the import and the assistant. Three copies
// of these rules would be three chances for one of them to be laxer than the
// others, and the lax one is the one that gets used.
func CheckAll(defs []Def, d Data) map[string]string {
	errs := map[string]string{}
	hidden := Hidden(defs, d.Values)
	for _, def := range defs {
		// A heading has nothing to check, and a field whose condition is not met
		// is not being asked for: demanding it would be demanding something the
		// person cannot even see — the one way a form can refuse to be sent
		// without saying why.
		if !def.HoldsValue() || hidden[def.Key] {
			continue
		}
		if !def.IsGroup() {
			if reason := Check(def, d.Values[def.Key]); reason != "" {
				errs[def.Key] = reason
			}
			continue
		}
		rows := d.Rows[def.Key]
		if def.Required && len(rows) == 0 {
			errs[def.Key] = def.Label + " braucht mindestens eine Zeile."
			continue
		}
		for i, row := range rows {
			for _, sub := range def.Sub {
				if reason := Check(sub, row[sub.Key]); reason != "" {
					errs[RowKey(def.Key, i, sub.Key)] = fmt.Sprintf("%s, Zeile %d: %s", def.Label, i+1, reason)
				}
			}
		}
	}
	return errs
}

// RowKey identifies one field of one row, in the form and in an error map.
func RowKey(group string, index int, sub string) string {
	return fmt.Sprintf("%s.%d.%s", group, index, sub)
}

// Sort orders definitions by position, then by id, so the order in the form is
// the order in the list and does not wobble between requests.
func Sort(defs []Def) {
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].Position != defs[j].Position {
			return defs[i].Position < defs[j].Position
		}
		return defs[i].ID < defs[j].ID
	})
}
