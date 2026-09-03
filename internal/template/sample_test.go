package template

import (
	"reflect"

	"testing"
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
