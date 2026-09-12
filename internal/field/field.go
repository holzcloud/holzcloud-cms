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
	"github.com/holzcloud/holzcloud-cms/internal/page"
	"math"
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
	// "Price and availability" over four of them is a form somebody fills in.
	KindSection = "abschnitt"
	// KindMulti holds several of its options at once, one value per line in the
	// same string slot every other kind uses.
	//
	// A kind of its own, and deliberately not KindChoice with a cap on how many
	// may be picked: Resolve has no case for KindChoice, so a choice reaches a
	// theme as a plain string. Widening KindChoice would silently retype what
	// every theme on every existing website already reads — no compile error,
	// no runtime error, just a theme printing a list where it printed a word.
	// A new kind is purely additive, and KindChoice keeps its contract.
	KindMulti = "mehrfachauswahl"
	// KindTime is a time of day without a date and without a time zone: a
	// departure, a closing time, the start of a guided tour.
	//
	// Separate from KindDate rather than its second half, because what a time
	// of day needs is exactly what a date does not have: "nothing entered" and
	// "midnight" are two different facts, and a string could not tell them
	// apart. That is why the field resolves to a pointer. It carries no time
	// zone — a shop opens at eight, and that stays eight wherever somebody is
	// looking from.
	KindTime = "zeit"
	// KindRange is a number between two bounds — and an <input type="number">
	// with min, max and step, deliberately not a slider.
	//
	// The condition three slider variants fail: the chosen number has to be
	// readable before saving, and this program carries no JavaScript besides
	// htmx. A slider on its own never shows its number; with an <output>
	// beside it, it shows one only when a script writes it there — exactly the
	// pattern internal/tmplmgr/script.go refuses in an uploaded template. In a
	// number field the chosen number *is* the visible content.
	KindRange = "bereich"
	// KindCode is the one kind whose content appears exactly as it was typed:
	// a snippet, a line of an address, a line of configuration.
	//
	// Never through the Markdown renderer — that is not an omission but the
	// whole purpose. What is written in is escaped and shown, not interpreted.
	// For flowing text with markup there is KindLong.
	KindCode = "code"
	// KindTerm is one of this website's terms — chosen from what the website
	// already carries, not typed.
	//
	// The difference from a text field is the same as the one between KindLink
	// and KindRef: a typed name goes wrong the moment somebody renames the
	// term, and a page would then carry a label that no longer exists with
	// nothing anywhere to report it. What is stored is therefore the slug —
	// the address a rename deliberately keeps — and what is printed is the
	// name as it reads right now. A rename thus changes what every page shows
	// without a single page being touched.
	KindTerm = "schlagwort"
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
	{KindText, i18n.N("Short text"), i18n.N("One line: a name, an amount, a variety.")},
	{KindLong, i18n.N("Long text"), i18n.N("Several lines without formatting.")},
	{KindCode, i18n.N("Code"), i18n.N("Several lines, shown exactly as they were typed.")},
	{KindNumber, i18n.N("Number"), i18n.N("A price, a weight, a year.")},
	{KindRange, i18n.N("Range"), i18n.N("A number between two limits — the limits themselves are allowed too.")},
	{KindDate, i18n.N("Date"), i18n.N("A day, without a time.")},
	{KindTime, i18n.N("Time"), i18n.N("A time, without a day.")},
	{KindBool, i18n.N("Yes/no"), i18n.N("A checkbox: in stock, sold out.")},
	{KindChoice, i18n.N("Choice"), i18n.N("A list of options, one of them.")},
	{KindMulti, i18n.N("Multiple choice"), i18n.N("The same list, but as many of them as you like — as checkboxes.")},
	{KindImage, i18n.N("Image"), i18n.N("An image from this website's media library.")},
	{KindLink, i18n.N("Link"), i18n.N("One of your own pages or an outside address.")},
	{KindRef, i18n.N("Reference"), i18n.N("A page of this website, chosen rather than typed. For several: a group with a reference in it.")},
	{KindTerm, i18n.N("Term"), i18n.N("A term of this website, chosen rather than typed. The page always shows the current name.")},
	{KindGroup, i18n.N("Group"), i18n.N("Several fields, filled in more than once — opening hours, say.")},
	{KindSection, i18n.N("Section"), i18n.N("Not an input but a heading over the fields that follow.")},
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
// Everything a group may have, minus the reference and the label. Both exist
// to survive a rename: a reference stores the page rather than its address and
// follows it, and a label field stores the label's address rather than its name
// and prints whatever the label is called right now. A block is rendered to
// HTML when the page is saved and that HTML is what visitors get, so either one
// inside a block would freeze what it had that day and quietly go stale on the
// next rename, with nothing anywhere reporting it. A promise that cannot be
// kept here is better not offered: the link field does the reference's job and
// says what it is, and a text field does the label's.
//
// The count in the capacity hint tracks the exclusions below. It is the one
// thing here that goes quietly wrong when a fifth is added.
func BlockKinds() []Kind {
	out := make([]Kind, 0, len(Kinds)-4)
	for _, k := range Kinds {
		switch k.Kind {
		case KindGroup, KindSection, KindRef, KindTerm:
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

// MaxFields bounds how many fields one carrier may define: the page's own
// fields with their groups, or one block kind, or one text snippet. Groups and
// their contents count together with the carrier they stand on.
//
// Not a technical limit but an editorial one: a form with sixty extra fields
// is a form nobody fills in correctly.
//
// And that reason is exactly what puts the count on the CARRIER rather than on
// the website (D-05): a form ever draws the fields of one carrier only. A
// shared allowance would let one carrier deny another, and ErrTooMany would
// then name a reason that is not true.
const MaxFields = 60

// MaxRows bounds how many times one group may be filled in on one page.
const MaxRows = 40

// MaxValueBytes bounds one stored value.
const MaxValueBytes = 4000

// maxKeyBytes bounds a field key.
//
// One number and not two: SlugifyKey truncates the derived key here, and
// validKey (store.go) checks against it. If the two drifted apart, validate
// would refuse what SlugifyKey itself produced — and would silently delete
// every condition pointing at a field with a key in between.
const maxKeyBytes = 40

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
	// SnippetID names the text snippet this field belongs to, or 0 for a field
	// that belongs to no snippet. The fourth carrier of the same table — see
	// migration 00047 — a world of its own beside the page, the group and the
	// block kind: a page field and a snippet's field may carry the same key
	// without ever meeting.
	SnippetID int64
	// Sub are the fields of a group, in order. Empty for everything else.
	Sub []Def
	// Condition is the key of the field this one hangs on: it is asked for only
	// once that field is filled in. Empty for a field that is always shown.
	Condition string
	// Display is the display mode of a choice: empty for the drop-down,
	// DisplayButtons for the row of buttons. Meaningless for every other kind
	// and emptied on save.
	Display string
	// MaxValues is the greatest number of values that may be picked at once in
	// a multi-choice. Zero means no upper limit. A COUNT — not to be confused
	// with RangeMax, which is a BOUND.
	MaxValues int
	// RangeMin and RangeMax are the lower and the upper bound of a range
	// field. Text and not numbers, so that "no bound" stays distinguishable
	// from "the bound is zero": the empty string means no bound in that
	// direction.
	RangeMin string
	RangeMax string
}

// DisplayButtons is the one display mode beside the drop-down: a choice drawn
// as a row of buttons.
//
// An empty Display is the drop-down that exists today — which is why every
// field in an existing database keeps its appearance without any data having
// to move.
const DisplayButtons = "knopfreihe"

// IsButtonRow reports whether this field is a choice drawn as a row of
// buttons rather than as a drop-down.
//
// The predicate lives here and not in the template, so that neither the form
// nor switchOf spells the comparison out a second time — two places spelling
// out the same rule drift apart sooner or later.
func (d Def) IsButtonRow() bool {
	return d.Kind == KindChoice && d.Display == DisplayButtons
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
	case KindGroup, KindSection:
		return false
	case KindDate, KindTime:
		// Both for the same reason: a date field and a time field never show a
		// placeholder. The rule that hides the dependent fields is
		// :placeholder-shown — so it could never fire here, and every field
		// hanging off one of these would stay visible for good. A range field
		// deliberately does NOT stand here: it is a number field and does
		// carry a placeholder — looked at in the browser during plan 07-07's
		// pass and confirmed, not inferred from reading (D-08).
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

// IsMultiValued reports whether this field holds several values at once.
func (d Def) IsMultiValued() bool { return d.Kind == KindMulti }

// NameSuffix is what the form field name carries after the key so the handler
// can tell a multi-valued field from a single one without loading the
// definitions — see FieldName and fieldsFromRequest.
func (d Def) NameSuffix() string {
	if d.IsMultiValued() {
		return "[]"
	}
	return ""
}

// FieldName is the name this field carries in the edit form.
//
// Prefixed so a field called "title" cannot collide with the page's own title,
// which would silently overwrite it.
//
// The suffix is the one place multi-valuedness is spelled. The form handler
// reads by prefix, before the definitions are loaded — that is deliberate and
// load-bearing — so the name itself has to say that several values may arrive
// under it. A key is slug-like and cannot contain a bracket, so the marker
// cannot collide with one.
func (d Def) FieldName() string { return "feld_" + d.Key + d.NameSuffix() }

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
// a plain {"kennung": "wert"} object, before groups existed. Five lines here //nolint:german — quotes the stored JSON shape as it was
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
		return "", fmt.Errorf("store fields: %w", err)
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
	simple := map[string]Def{}
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
		simple[def.Key] = def
	}

	out := Data{Values: Values{}, Rows: map[string][]Values{}}
	for key, val := range d.Values {
		def, known := simple[key]
		if !known {
			continue
		}
		val = normalizeValue(def, trimTo(val))
		if val != "" {
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
		if val := normalizeValue(def, trimTo(row[def.Key])); val != "" {
			out[def.Key] = val
		}
	}
	return out
}

// normalizeValue is the one place a value's spelling is settled before it is
// stored. Only the boolean needs it today: Resolve reads a stored value back
// as `raw != "" && raw != "0"`, so the words have to become "1" or "" on the
// way in or they mean their opposite on the way out. Check has already refused
// a word neither reading knows, so an unreadable value here is left alone
// rather than silently turned into no.
func normalizeValue(d Def, val string) string {
	switch d.Kind {
	case KindBool:
		if canonical, ok := NormalizeBool(val); ok {
			return canonical
		}
	case KindTime:
		// "09:30:00" is valid because some browsers send the seconds along,
		// and the specification promises the theme HH:MM. field.List shapes it
		// on the way in, so that yesterday's stored values are right too; here
		// stands the other half, so that from now on nothing else reaches the
		// column at all.
		if t, ok := ParseTimeOfDay(val); ok {
			return t.Format("15:04")
		}
	}
	return val
}

// trimTo strips the whitespace around a value, which every caller relies on.
//
// It does not shorten any more: an over-long value is reported by Check before
// it can reach here, so shortening would only ever hide that report — and for
// a multi-valued field it would cut one value in half and store the fragment
// as if somebody had typed it (D-13).
func trimTo(val string) string {
	return strings.TrimSpace(val)
}

// ParseNumber reads a number the way it is typed on a keyboard here: the comma
// is a decimal separator.
//
// One place for both sides of the same question — the value that was entered
// and the two bounds it is held against. Two readings of the same digits would
// be exactly the kind of difference that only shows up on a pair of numbers
// nobody checks by hand.
func ParseNumber(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil {
		return 0, false
	}
	// NaN and infinity are not numbers anybody means, and NaN is the one that
	// takes a range's meaning away: n < lower and n > upper are both false for
	// it, so it lies inside every range there is. Here and not in the bounds
	// check, because this function reads both sides of the same question —
	// otherwise a bound typed as "NaN" would still be a bound that enforces
	// nothing.
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// NormalizeBool reads a yes/no and gives back the two values everything else
// in the tree agrees on: "" for no and "1" for yes.
//
// One reading, and it is exported for that reason. The CSV importer had the
// only reading of a boolean in the codebase — everything else simply stored
// what it was handed and let Resolve read `raw != "" && raw != "0"`, which
// makes the word "nein" mean yes. Two readings of the same four words would
// drift, and this one is where the drift showed: an archive or an assistant
// could store "nein" and every theme printed "ja".
//
// The vocabulary is the CSV importer's, unchanged, because it is the one an
// operator has already been able to type.
func NormalizeBool(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", true
	case "ja", "yes", "wahr", "true", "1", "x":
		return "1", true
	case "nein", "no", "falsch", "false", "0":
		return "", true
	}
	return "", false
}

// ParseTimeOfDay reads a time of day without a date and without a time zone.
//
// The length decides the pattern rather than letting time.Parse guess: its
// hour field also accepts a single digit, so "9:30" would quietly come through
// as half past nine. An <input type="time"> always sends two digits; somebody
// typing by hand should notice. The seconds are allowed because some browsers
// send them along.
//
// The result carries no zone offset and no usable date: it is a time of day
// and nothing else.
func ParseTimeOfDay(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	layout := ""
	switch len(value) {
	case len("15:04"):
		layout = "15:04"
	case len("15:04:05"):
		layout = "15:04:05"
	default:
		return time.Time{}, false
	}
	t, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// rangeReason writes the reason for the person at the form and names the bound
// the value fails against — both, where there are both.
func rangeReason(label, unten, oben string, hatUnten, hatOben bool) Reason {
	switch {
	case hatUnten && hatOben:
		return reasonf(i18n.N("%s has to be between %s and %s."), label, unten, oben)
	case hatUnten:
		return reasonf(i18n.N("%s has to be at least %s."), label, unten)
	default:
		return reasonf(i18n.N("%s may be at most %s."), label, oben)
	}
}

// tooLong reports whether a value exceeds the space one stored value has, and
// with what reason.
//
// The space is there for all of a field's values together: in a multi-value
// field including the line breaks between them, because that is exactly the
// string that goes into the database. It is measured with len, so in bytes and
// not in runes — the limit guards what is stored, and what is stored is bytes.
// An umlaut needs two of them.
//
// Reported and not truncated (D-13): a truncated value looks like one somebody
// typed that way, and in a multi-value field half a value would be a value
// that never existed. Check calls it before the per-kind switch, so the rule
// holds for every kind.
//
// A function of its own, because it also holds where nobody was asked for the
// value: a per-kind rule is a question to the person in front of the form, and
// somebody who cannot see the field cannot answer it. The byte limit is a
// question to nobody — it says how much room a value has in the row that gets
// written. That is why CheckAll skips everything but this one check for a
// hidden field.
func tooLong(d Def, value string) Reason {
	if len(value) > MaxValueBytes {
		return reasonf(i18n.N("%s is too long: at most %s characters, where accented letters count double."),
			d.Label, strconv.Itoa(MaxValueBytes))
	}
	return Reason{}
}

// Check validates one value against its definition and returns a reason, or
// the empty string when it is fine.
//
// The reasons are written for the person filling the form in, not for a log:
// "Preis muss eine Zahl sein" and not "strconv.ParseFloat: invalid syntax". //nolint:german — quotes the sentence this reason replaces
func Check(d Def, value string) Reason {
	value = strings.TrimSpace(value)
	if value == "" {
		if d.Required {
			return reasonf(i18n.N("%s has to be filled in."), d.Label)
		}
		return Reason{}
	}

	if reason := tooLong(d, value); !reason.Empty() {
		return reason
	}

	switch d.Kind {
	case KindGroup:
		// A group is checked row by row, not as one value.
		return Reason{}
	case KindBool:
		// The arm this switch was missing. Without it any non-empty string
		// passed, and Resolve reads everything that is not "0" as yes — so a
		// stored "nein" printed as "ja" on every shipped theme, on the three
		// paths that name CheckAll as their only per-kind gate.
		if _, ok := NormalizeBool(value); !ok {
			return reasonf(i18n.N("%s has to be yes or no."), d.Label)
		}
	case KindNumber:
		if _, ok := ParseNumber(value); !ok {
			return reasonf(i18n.N("%s has to be a number."), d.Label)
		}
	case KindRange:
		n, ok := ParseNumber(value)
		if !ok {
			return reasonf(i18n.N("%s has to be a number."), d.Label)
		}
		// Both comparisons include the bound: the bound itself is an allowed
		// value. A bound that is not a number is not a bound — the same
		// reading validate applies in store.go when it looks for an inverted
		// pair.
		unten, hatUnten := ParseNumber(d.RangeMin)
		oben, hatOben := ParseNumber(d.RangeMax)
		if (hatUnten && n < unten) || (hatOben && n > oben) {
			return rangeReason(d.Label, d.RangeMin, d.RangeMax, hatUnten, hatOben)
		}
	case KindTime:
		if _, ok := ParseTimeOfDay(value); !ok {
			return reasonf(i18n.N("%s has to be a time, for example 09:30."), d.Label)
		}
	case KindDate:
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return reasonf(i18n.N("%s has to be a date."), d.Label)
		}
	case KindChoice:
		for _, c := range d.Choices {
			if c == value {
				return Reason{}
			}
		}
		return reasonf(i18n.N("%s: “%s” is not one of the choices."), d.Label, value)
	case KindMulti:
		// Every picked value has to be on the list. The options are a closed
		// vocabulary, so an arbitrary string must not get in through a
		// checkbox group somebody rewrote before submitting. The first one
		// that is not on the list is named — a reason that says "something is
		// wrong" is a reason nobody can act on.
		for _, picked := range SplitValues(value) {
			known := false
			for _, c := range d.Choices {
				if c == picked {
					known = true
					break
				}
			}
			if !known {
				return reasonf(i18n.N("%s: “%s” is not one of the choices."), d.Label, picked)
			}
		}
		// The maximum holds here or nowhere: a checkbox group cannot be
		// bounded in the markup, that would need JavaScript, and this program
		// carries none of it besides htmx (D-05). Zero means no bound — which
		// is what every field created before this phase carries, and nothing
		// about any of them may change.
		if d.MaxValues > 0 {
			if n := len(SplitValues(value)); n > d.MaxValues {
				if d.MaxValues == 1 {
					return reasonf(i18n.N("%s: at most one value, %s are selected."),
						d.Label, strconv.Itoa(n))
				}
				return reasonf(i18n.N("%s: at most %s values, %s are selected."),
					d.Label, strconv.Itoa(d.MaxValues), strconv.Itoa(n))
			}
		}
	case KindImage:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return reasonf(i18n.N("%s: that is not a picture from the media library."), d.Label)
		}
	case KindRef:
		// That the page exists and belongs to this website is decided where
		// the pages are known — the editor only offers its own, and the theme
		// resolves through a lookup that checks. Here it is a number or it is
		// somebody typing into the form by hand.
		if id, err := strconv.ParseInt(value, 10, 64); err != nil || id <= 0 {
			return reasonf(i18n.N("%s: that is not a page of this website."), d.Label)
		}
	case KindTerm:
		// That the term exists and that it belongs to this website is decided
		// where the terms are known — the form offers only its own, and the
		// theme resolves through a lookup that checks. Here it is a slug, or
		// it is somebody typing into the form by hand.
		//
		// Checked against page.Slugify and not against a second spelling of
		// what a slug is: a slug is exactly the string that yields itself
		// again. Two rules side by side would be two rules that drift apart.
		if page.Slugify(value) != value {
			return reasonf(i18n.N("%s: that is not a term of this website."), d.Label)
		}
	case KindLink:
		if reason := checkLink(value); !reason.Empty() {
			return reasonf(i18n.N("%s: %s"), d.Label, reason)
		}
	}
	return Reason{}
}

// checkLink keeps a field from becoming a way to put javascript: into a theme.
//
// Only three shapes are allowed, and they are the three that mean something on
// a website: a path on this site, an http(s) address, and a mail address.
func checkLink(value string) Reason {
	switch {
	case strings.HasPrefix(value, "/"):
		if strings.HasPrefix(value, "//") {
			return reasonf(i18n.N("an address with two slashes leads to somebody else’s server."))
		}
		return Reason{}
	case strings.HasPrefix(value, "https://"), strings.HasPrefix(value, "http://"),
		strings.HasPrefix(value, "mailto:"), strings.HasPrefix(value, "tel:"):
		return Reason{}
	}
	return reasonf(i18n.N("this has to start with / (a page of your own) or with https:// (an address elsewhere)."))
}

// SlugifyKey turns a label into a key.
func SlugifyKey(label string) string {
	var b strings.Builder
	prevDash := false
	// page.Transliterate and not a second table here.
	//
	// This function used to carry its own four-entry list — ä, ö, ü, ß — and //nolint:german — names the letters it is about
	// silently DROPPED every other accented letter, because the first arm of
	// the switch only keeps a-z. So "Título" became "ttulo" and "État" became
	// "tat": a German operator's label transliterated and a Spanish or French
	// one lost a letter. page.Transliterate has known the full Latin-1 set all
	// along, and this file already argues the principle in its KindTerm arm —
	// two spellings of one rule are two rules that drift apart.
	//
	// Only NEW keys are affected. A key is minted once from the label and then
	// stands for good, precisely so a page does not lose its values when
	// somebody rewords a label.
	for _, r := range page.Transliterate(label) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
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
	if len(key) > maxKeyBytes {
		key = strings.Trim(key[:maxKeyBytes], "_")
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

// SplitValues reads a multi-valued field's stored value: one value per line.
//
// This and JoinValues are the ONE pair that encodes a multi-valued value. The
// page form, the theme's resolution and the bundle round trip all go through
// them, and they are exported so a later importer inherits them instead of
// inventing a third spelling. A newline cannot occur inside a value because
// the options a value is drawn from are themselves read one per line.
//
// Deliberately not the option-list pair above under another name, and neither
// pair calls the other: two names for two meanings, so a later change to how
// an option list is read cannot silently change how a value is stored.
func SplitValues(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// JoinValues writes the picked values back into the one string slot.
//
// Empty entries fall away — a checkbox group's hidden sentinel submits one,
// and a partly ticked group must not notice it. Duplicates and order are kept
// exactly as the caller passed them: no sorting, no removing, so saving the
// same form twice produces the same string byte for byte.
//
// An entry that carries the separator itself is folded: every line break
// inside it becomes a space, so that the number of values SplitValues reads
// back never exceeds the number of non-empty entries. One entry can therefore
// never become a second value.
//
// Folded and not escaped: an escape would be a second spelling of the same
// value, and a later reader would inherit both (D-02). No error path either,
// because the one caller that reads the form has none — fieldsFromRequest
// deliberately runs before the definitions are loaded (D-03). The folding is
// the same class of normalisation this function already does by trimming and
// by dropping empty entries: it enforces what SplitValues' doc comment has so
// far only claimed. For a field with a closed list of options it changes
// nothing — a folded value is no more one of the choices than the unfolded
// one. For a later caller whose values come out of a CSV column it is the
// boundary beyond which no extra value can be minted.
func JoinValues(values []string) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		// Only the separator, not every whitespace: two spaces inside a valid
		// value are part of the value.
		if v = strings.TrimSpace(faltZeilen.Replace(v)); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, "\n")
}

// foldLineBreaks replaces every spelling of the line break with a single
// space. The carriage-return form stands first, so that it yields one space
// and not two.
var faltZeilen = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

// CheckAll validates a page's answers against the fields that apply to it and
// returns the reasons, keyed by the form field they belong to.
//
// One function for the admin form, the import and the assistant. Three copies
// of these rules would be three chances for one of them to be laxer than the
// others, and the lax one is the one that gets used.
func CheckAll(defs []Def, d Data) map[string]Reason {
	errs := map[string]Reason{}
	hidden := Hidden(defs, d.Values)
	for _, def := range defs {
		// A heading has nothing to check.
		if !def.HoldsValue() {
			continue
		}
		// A field whose condition is not met is not asked for: demanding a
		// per-kind rule of it would mean demanding something the person cannot
		// even see — the one way a form cannot be submitted without saying
		// why. So what is skipped is the required check, the per-kind switch
		// and the refusal "needs at least one row".
		//
		// The byte limit stays: Clean deliberately keeps a hidden field's
		// value, and since D-13 trimTo truncates nothing — this is the only
		// place the byte budget still holds at all.
		if hidden[def.Key] {
			if !def.IsGroup() {
				if reason := tooLong(def, strings.TrimSpace(d.Values[def.Key])); !reason.Empty() {
					errs[def.Key] = reason
				}
				continue
			}
			for i, row := range d.Rows[def.Key] {
				for _, sub := range def.Sub {
					if reason := tooLong(sub, strings.TrimSpace(row[sub.Key])); !reason.Empty() {
						errs[RowKey(def.Key, i, sub.Key)] = inRow(def.Label, i, reason)
					}
				}
			}
			continue
		}
		if !def.IsGroup() {
			if reason := Check(def, d.Values[def.Key]); !reason.Empty() {
				errs[def.Key] = reason
			}
			continue
		}
		rows := d.Rows[def.Key]
		if def.Required && len(rows) == 0 {
			errs[def.Key] = reasonf(i18n.N("%s needs at least one row."), def.Label)
			continue
		}
		for i, row := range rows {
			for _, sub := range def.Sub {
				if reason := Check(sub, row[sub.Key]); !reason.Empty() {
					errs[RowKey(def.Key, i, sub.Key)] = inRow(def.Label, i, reason)
				}
			}
		}
	}
	return errs
}

// inRow wraps the reason of a sub-field in the row it stands in.
//
// The frame used to be fmt.Sprintf("%s, Zeile %d: %s", …), which is the shape
// broken window 29 was opened for: even once the inner reason carried a key,
// the frame around it never could. As a Reason holding a Reason, both halves
// are collected and both are rendered in the reader's language — which matters
// more here than anywhere, because the two ended up on the same line.
func inRow(groupLabel string, index int, inner Reason) Reason {
	return reasonf(i18n.N("%s, row %d: %s"), groupLabel, index+1, inner)
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
