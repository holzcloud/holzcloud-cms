package ai

import (
	"context"
	"errors"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// The values of a website: one line of plain text per key, named in page text
// as [[wert:key]] and replaced at delivery.
//
// The split is the snippets': a read-only key reads them, a content key writes
// them, and no publishing right is asked — the level of the key decides (Phase
// 31 D-11). Every write goes through the admin handler
// (internal/admin/ops_values.go) into the store, which checks the key and the
// text on every path; nothing here decides what a valid key or text is, it only
// says in English which rule the store's answer names.

// The activity log's names for what these tools do, local for the reason the
// structure tools give.
const (
	actValueSet    = "value.set"
	actValueDelete = "value.delete"
)

// valueOps is the admin handler's part for values, an interface of its own so a
// build without it loses these tools only.
type valueOps interface {
	OpValues(ctx context.Context, websiteID int64) ([]value.Value, value.Report, error)
	OpValue(ctx context.Context, websiteID int64, key string) (*value.Value, value.Report, error)
	OpSetValue(ctx context.Context, websiteID int64, key, text string) (*value.Value, bool, error)
	OpDeleteValue(ctx context.Context, websiteID int64, key string) error
}

// valuesAbout is what every value tool tells an assistant about values, so the
// four descriptions cannot contradict each other.
const valuesAbout = "A value is one line of plain text (at most 400 bytes, no line break, no [[), " +
	"the same in every language of the website. Its marker [[wert:key]] is replaced in page " +
	"text — prose, headings, lists, link addresses — at delivery, so a change reaches every " +
	"page without saving it. Titles, descriptions and menu labels do not replace it. A marker " +
	"whose key the website does not define shows nothing."

var valueKeyProp = Property{Type: "string",
	Description: "the key, such as version-holzcloud-cms: lower-case letters, digits, - and _"}

// valueRefusal says in English which rule the store's answer names, and passes
// every other error on unchanged.
func valueRefusal(err error) error {
	switch {
	case errors.Is(err, value.ErrBadKey):
		return errors.New("the key may only contain lower-case letters, digits, - and _ " +
			"(not first or last), at most 60 bytes")
	case errors.Is(err, value.ErrTextEmpty):
		return errors.New("the value is empty: give one line of text")
	case errors.Is(err, value.ErrTextTooLong):
		return errors.New("the value is longer than 400 bytes: shorten it")
	case errors.Is(err, value.ErrTextInvalid):
		return errors.New("the value must be one line of valid UTF-8 text, without a line " +
			"break, a tab or another control character")
	case errors.Is(err, value.ErrTextMarker):
		return errors.New("the value must not contain [[: a value cannot carry a placeholder")
	}
	return err
}

// valueUsers splits who names a key into the pages, described the way brief
// describes a page, and the keys of the snippets.
func valueUsers(uses []value.Use) ([]map[string]any, []string) {
	pages := make([]map[string]any, 0, len(uses))
	snippets := make([]string, 0)
	for _, u := range uses {
		if u.Kind == "snippet" {
			snippets = append(snippets, u.Slug)
			continue
		}
		p := map[string]any{"id": u.ID, "title": u.Title, "slug": u.Slug, "status": wireStatus(u.Status)}
		// Only when there is something to say, as in brief.
		if u.Locale != "" {
			p["language"] = u.Locale
		}
		pages = append(pages, p)
	}
	return pages, snippets
}

// valueOut is one value with the pages and snippets that name it.
func valueOut(v value.Value, rep value.Report) map[string]any {
	pages, snippets := valueUsers(rep.Used[v.Key])
	return map[string]any{
		"key": v.Key, "value": v.Text,
		// The marker is what goes into a page's text; an assistant told the key
		// alone has to guess the brackets.
		"marker":   value.Marker(v.Key),
		"used_on":  pages,
		"snippets": snippets,
		"updated":  v.UpdatedAt.UTC().Format(timeLayout),
	}
}

func listValues(d Deps) Tool {
	return Tool{
		Name: "list_values",
		Description: "Lists the values of a website, each with its marker, the pages and snippets " +
			"that name it, and when it last changed. Beside them: undefined, every key a page or " +
			"snippet names that the website does not define, and outside_text, every marker in a " +
			"title, description or menu entry, where it is printed as written. " + valuesAbout,
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{"website": structWebsiteProp}, Required: []string{"website"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64 `json:"website"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[valueOps](d)
			if err != nil {
				return nil, err
			}
			list, rep, err := ops.OpValues(c.Ctx, a.Website)
			if err != nil {
				return nil, err
			}
			values := make([]map[string]any, 0, len(list))
			for _, v := range list {
				values = append(values, valueOut(v, rep))
			}
			undefined := make([]map[string]any, 0, len(rep.Undefined))
			for _, key := range rep.Undefined {
				pages, snippets := valueUsers(rep.Used[key])
				undefined = append(undefined, map[string]any{
					"key": key, "marker": value.Marker(key), "used_on": pages, "snippets": snippets,
				})
			}
			outside := make([]map[string]any, 0, len(rep.Outside))
			for _, o := range rep.Outside {
				e := map[string]any{"key": o.Key, "where": o.Where, "id": o.ID, "title": o.Title}
				if o.Locale != "" {
					e["language"] = o.Locale
				}
				outside = append(outside, e)
			}
			return map[string]any{"values": values, "undefined": undefined, "outside_text": outside}, nil
		},
	}
}

func setValue(d Deps) Tool {
	return Tool{
		Name:   "set_value",
		Writes: true,
		Description: "Sets the value of a key on a website: creates it, or changes its text when " +
			"the key exists. Every page naming the key shows the new text on its next delivery. " +
			"Answers whether the value was created. " + valuesAbout,
		InputSchema: Schema{Type: "object",
			Properties: map[string]Property{
				"website": structWebsiteProp,
				"key":     valueKeyProp,
				"value":   {Type: "string", Description: "the text, one line of plain text"},
			}, Required: []string{"website", "key", "value"}},
		Run: func(c Call) (any, error) {
			var a struct {
				Website int64  `json:"website"`
				Key     string `json:"key"`
				Value   string `json:"value"`
			}
			if err := withWebsite(c, &a, &a.Website); err != nil {
				return nil, err
			}
			ops, err := structOps[valueOps](d)
			if err != nil {
				return nil, err
			}
			v, created, err := ops.OpSetValue(c.Ctx, a.Website, a.Key, a.Value)
			if err != nil {
				return nil, valueRefusal(err)
			}
			changed(d, c, a.Website, activity.Entry{Action: actValueSet, EntityType: "value", EntityID: v.ID})
			return map[string]any{
				"key": v.Key, "value": v.Text, "marker": value.Marker(v.Key), "created": created,
			}, nil
		},
	}
}
