package template

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/field"
)

// SampleData is what an uploaded template is rendered against before it is
// accepted, and what the authoring specification describes. A field left at its
// zero value there is a field no template ever gets exercised on: the check
// would accept an archive whose handling of it has never once run.
//
// Adding a field to PageData without adding it here is the easy mistake, and it
// is invisible — everything still compiles and every existing test still
// passes. So the fixture is walked by reflection instead of by eye.
func TestSampleDataFillsEveryField(t *testing.T) {
	var missing []string
	walkContract(reflect.ValueOf(SampleData()), "PageData", &missing)

	for _, path := range missing {
		t.Errorf("SampleData leaves %s at its zero value — no template is ever "+
			"rendered with it, so the upload check cannot catch a mistake there", path)
	}
}

// walkContract descends through the data contract and records every exported
// field that is still zero.
//
// It only follows types declared in this package. time.Time is a struct with
// unexported fields and no meaning to a template beyond "a date"; descending
// into it would report its internals as missing.
func walkContract(v reflect.Value, path string, missing *[]string) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			*missing = append(*missing, path)
			return
		}
		walkContract(v.Elem(), path, missing)

	case reflect.Map, reflect.Slice:
		if v.Len() == 0 {
			*missing = append(*missing, path)
			return
		}
		// One element is enough: they are all the same type, and the point is
		// that the element type gets rendered at all.
		if v.Kind() == reflect.Slice {
			walkContract(v.Index(0), path+"[0]", missing)
			return
		}
		iter := v.MapRange()
		iter.Next()
		walkContract(iter.Value(), path+"[…]", missing)

	case reflect.Struct:
		if !ownType(v.Type()) {
			// A foreign struct — time.Time, menu.MenuNode — counts as filled
			// once it is reachable. Its own package owns its completeness.
			if v.IsZero() {
				*missing = append(*missing, path)
			}
			return
		}
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			walkContract(v.Field(i), path+"."+field.Name, missing)
		}

	default:
		if v.IsZero() {
			*missing = append(*missing, path)
		}
	}
}

// MinimalData is the other half of the check: it must leave the optional fields
// empty, or it stops being the case that catches `{{.Page.Prev.Title}}`.
func TestMinimalDataLeavesOptionalFieldsEmpty(t *testing.T) {
	d := MinimalData()

	if d.Page.Prev != nil || d.Page.Next != nil {
		t.Error("MinimalData has post neighbours; a template dereferencing them unguarded would pass the check")
	}
	if d.Page.PublishedAt != nil || d.Page.UpdatedAt != nil {
		t.Error("MinimalData has dates; a template calling formatDate on a nil date would pass the check")
	}
	if len(d.Menus) != 0 {
		t.Error("MinimalData has menus; a site before its first menu exists would not be covered")
	}
	if len(d.Archive.Entries) != 0 {
		t.Error("MinimalData has archive entries; an empty archive would not be covered")
	}
	if len(d.Page.Terms) != 0 || len(d.Site.Terms) != 0 {
		t.Error("MinimalData has labels; a site that uses none would not be covered")
	}
	if d.Site.LogoURL != "" || d.Meta.OGImage != "" {
		t.Error("MinimalData has images; a site without a logo would not be covered")
	}

	// The required minimum has to stay present, or the check reports failures
	// that say nothing about the template.
	if d.Site.Name == "" || d.Page.Title == "" || d.Site.Locale == "" {
		t.Error("MinimalData is missing a field every page genuinely always has")
	}
}

// The two views of the same data have to agree, and each entry has to be
// shaped the way field.List shapes it.
//
// A fixture is only worth what it resembles. An entry of kind "schlagwort"
// with a nil Term, or a "mehrfachauswahl" whose Text is not its values joined,
// documents a contract the renderer never produces — and the upload check
// would then accept a template that fails on a real page, or reject one that
// would have worked.
func TestSampleFieldsAreShapedLikeTheRendererProducesThem(t *testing.T) {
	d := SampleData()

	if len(d.Page.Feldliste) != len(d.Page.Felder) {
		t.Errorf("Feldliste has %d entries and Felder %d keys — they are two "+
			"views of the same data and a template author is told so",
			len(d.Page.Feldliste), len(d.Page.Felder))
	}

	for _, e := range d.Page.Feldliste {
		if e.Key == "" || e.Label == "" || e.Kind == "" {
			t.Errorf("an entry is missing its key, label or kind: %+v", e)
			continue
		}
		if !field.KnownKind(e.Kind) {
			t.Errorf("%s carries kind %q, which this version cannot render", e.Key, e.Kind)
			continue
		}
		if value, ok := d.Page.Felder[e.Key]; !ok {
			t.Errorf("%s is in Feldliste but not in Felder", e.Key)
		} else if !reflect.DeepEqual(value, e.Value) {
			t.Errorf("%s: Felder holds %#v and Feldliste holds %#v", e.Key, value, e.Value)
		}

		switch e.Kind {
		case field.KindMulti:
			if len(e.Values) == 0 {
				t.Errorf("%s: a multiple choice with no values would be left out of the list entirely", e.Key)
			}
			if want := strings.Join(e.Values, ", "); e.Text != want {
				t.Errorf("%s: Text is %q, but field.List joins the values to %q", e.Key, e.Text, want)
			}
		case field.KindTerm:
			if e.Term == nil {
				t.Errorf("%s: a label field whose label is gone is left out of the list, never carried with a nil Term", e.Key)
			} else if e.Text != e.Term.Name {
				t.Errorf("%s: Text is %q, but the list prints the label's current name %q", e.Key, e.Text, e.Term.Name)
			}
		case field.KindTime:
			if v, ok := e.Value.(*time.Time); !ok || v == nil {
				t.Errorf("%s: a time of day resolves to a *time.Time, and never a nil one in the list", e.Key)
			}
			if e.Text == "" {
				t.Errorf("%s: a time of day prints itself as HH:MM; there is no formatTime helper", e.Key)
			}
		case field.KindDate:
			if v, ok := e.Value.(*time.Time); !ok || v == nil {
				t.Errorf("%s: a date resolves to a *time.Time", e.Key)
			}
			if e.Text != "" {
				t.Errorf("%s: a date leaves Text empty and is printed with formatDate", e.Key)
			}
		case field.KindNumber, field.KindRange:
			n, ok := e.Value.(field.Number)
			if !ok || n.Raw == "" {
				t.Errorf("%s: a number resolves to a field.Number carrying what was typed", e.Key)
			} else if e.Text != n.Raw {
				t.Errorf("%s: Text is %q but the number was typed as %q", e.Key, e.Text, n.Raw)
			}
		case field.KindBool:
			if !e.Yes {
				t.Errorf("%s: a no is left out of the list, so an entry that is there is a yes", e.Key)
			}
		case field.KindImage:
			if e.Image == nil {
				t.Errorf("%s: a picture that is gone is left out of the list", e.Key)
			}
		case field.KindRef:
			if e.Ref == nil {
				t.Errorf("%s: a reference whose target is gone is left out of the list", e.Key)
			} else if e.Text != e.Ref.Title {
				t.Errorf("%s: Text is %q, but the list prints the target's current title %q", e.Key, e.Text, e.Ref.Title)
			}
		case field.KindGroup:
			if len(e.Rows) == 0 {
				t.Errorf("%s: a group with no rows is left out of the list", e.Key)
			}
		case field.KindText, field.KindLong, field.KindCode, field.KindChoice, field.KindLink:
			v, ok := e.Value.(string)
			if !ok || v == "" {
				t.Errorf("%s: this kind resolves to a plain string, and an empty one is left out of the list", e.Key)
			} else if e.Text != v {
				t.Errorf("%s: Text is %q but the value is %q", e.Key, e.Text, v)
			}
		default:
			t.Errorf("%s: kind %q is one this test has never been taught to check — "+
				"a new kind pays its tax here too", e.Key, e.Kind)
		}
	}
}

// MinimalData is where the empty case of an own field lives.
//
// Not in Feldliste: field.List drops every entry whose value is empty — every
// arm of its switch does — so a list carrying an empty entry is a state the
// program cannot produce, and a fixture that invented one would reject a
// template for reading the list the way the specification tells it to.
//
// In Felder: Resolve puts every defined field in the map, filled or not. That
// is where {{.Page.Felder.abholzeit}} is a nil time rather than a missing key,
// and where a theme that reaches through it unguarded has to fail.
func TestMinimalDataCarriesTheEmptyValueOfEveryOwnField(t *testing.T) {
	sample := SampleData()
	minimal := MinimalData()

	if len(minimal.Page.Feldliste) != 0 {
		t.Error("MinimalData carries field entries; field.List never produces an " +
			"empty one, so the fixture would describe a page that cannot exist")
	}

	kinds := map[string]string{}
	for _, e := range sample.Page.Feldliste {
		kinds[e.Key] = e.Kind
	}

	for key := range sample.Page.Felder {
		value, ok := minimal.Page.Felder[key]
		if !ok {
			t.Errorf("MinimalData has no %s; a page where nobody filled it in is "+
				"then never rendered, and the upload check cannot catch a theme "+
				"that assumes it is filled", key)
			continue
		}
		want := emptyValueOf(kinds[key])
		if !reflect.DeepEqual(value, want) {
			t.Errorf("MinimalData's %s is %#v; an unfilled %s resolves to %#v",
				key, value, kinds[key], want)
		}
	}
	for key := range minimal.Page.Felder {
		if _, ok := sample.Page.Felder[key]; !ok {
			t.Errorf("MinimalData has %s and SampleData does not — the filled case "+
				"of that field is then never rendered", key)
		}
	}
}

// emptyValueOf is what field.Resolve puts in the map for a field nobody filled
// in. It is written out rather than obtained by calling Resolve so that a
// change in Resolve shows up as a disagreement here instead of being copied.
func emptyValueOf(kind string) any {
	switch kind {
	case field.KindMulti:
		return []string{}
	case field.KindTerm:
		return (*field.Term)(nil)
	case field.KindTime, field.KindDate:
		return (*time.Time)(nil)
	case field.KindNumber, field.KindRange:
		return field.Number{}
	case field.KindImage:
		return (*field.Image)(nil)
	case field.KindRef:
		return (*field.Ref)(nil)
	case field.KindBool:
		return false
	default:
		return ""
	}
}
