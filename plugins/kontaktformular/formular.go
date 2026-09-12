package main

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

// Forms of one's own.
//
// The contact form asks for a name, an e-mail address, a subject and a message.
// For an enquiry about wool that is half of what you want to know — how much,
// which colour, by when — and for signing up to a farm festival it is the wrong
// thing. So an operator can assemble their own forms, each with its own fields,
// and put them into a page with the same marker.
//
// The built-in form stays as it was. Whoever defines nothing notices none of
// this.

const praefixFormular = "formular:"

// maxFields bounds a form.
//
// Twenty fields are already a form nobody fills in. The limit is not against
// abuse — it is the operator themselves typing here — but against a form that
// prevents the enquiries it exists for.
const maxFields = 20

// Field kinds. Few of them, because each is one the operator has to understand
// and the theme has to style.
const (
	ArtText    = "text"
	ArtLang    = "lang"
	ArtEmail   = "email"
	ArtTelefon = "telefon"
	ArtZahl    = "zahl"
	ArtDatum   = "datum"
	KindChoice = "auswahl"
	ArtAnkreuz = "ankreuz"
)

// fieldKind describes a kind for the select in the admin.
type fieldKind struct {
	Art  string
	Name string
}

var fieldKinds = []fieldKind{
	{ArtText, "Kurze Antwort"},
	{ArtLang, "Lange Antwort"},
	{ArtEmail, "E-Mail-Adresse"},
	{ArtTelefon, "Telefonnummer"},
	{ArtZahl, "Zahl"},
	{ArtDatum, "Datum"},
	{KindChoice, "Choice from a list"},
	{ArtAnkreuz, "Ankreuzfeld"},
}

func artName(art string) string {
	for _, a := range fieldKinds {
		if a.Art == art {
			return a.Name
		}
	}
	return art
}

// field is one question.
type field struct {
	// Key is the field name in the submitted form. It is derived from the label
	// and then stands: if it changed with the label, an answer would arrive
	// under a different name after every rewording, and the old ones could no
	// longer be matched up.
	Key      string   `json:"kennung"`
	Label    string   `json:"beschriftung"`
	Art      string   `json:"art"`
	Required bool     `json:"pflicht,omitempty"`
	Hint     string   `json:"hinweis,omitempty"`
	Choices  []string `json:"auswahl,omitempty"`
}

// formular ist ein zusammengestelltes Formular.
type formular struct {
	Key  string `json:"kennung"`
	Name string `json:"name"`
	// Subject stands in the notification when no field supplies one.
	Subject string `json:"betreff,omitempty"`
	// Thanks is the sentence after submitting.
	Dank   string  `json:"dank,omitempty"`
	Fields []field `json:"felder,omitempty"`
}

// reKey is this narrow because a key goes into a marker in the page text and
// into a field name: everything that would have to be escaped there is escaped
// wrongly sooner or later.
var reKey = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// keyFrom makes a key out of a label.
func keyFrom(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// The four German letters, spelled out because this IS the table that
	// transliterates them.
	replacer := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss") //nolint:german — the letters this rule is made of
	s = replacer.Replace(s)

	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" || !reKey.MatchString(out) {
		return ""
	}
	return out
}

// answerField looks for the first field of a kind. That way it is clear without
// an extra setting where an answer goes: the first e-mail address in the form is
// the sender's.
func (f formular) ersteArt(art string) (field, bool) {
	for _, fe := range f.Fields {
		if fe.Art == art {
			return fe, true
		}
	}
	return field{}, false
}

// load fetches a form. Not found is not an error: the marker in the text can
// name a key that no longer exists.
func formularLaden(key string) (formular, bool) {
	if !reKey.MatchString(key) {
		return formular{}, false
	}
	raw, ok, err := plugin.Get(praefixFormular + key)
	if err != nil || !ok {
		return formular{}, false
	}
	var f formular
	if json.Unmarshal([]byte(raw), &f) != nil || f.Key == "" {
		return formular{}, false
	}
	return f, true
}

func save(f formular) error {
	raw, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return plugin.Set(praefixFormular+f.Key, string(raw))
}

// allForms lists what is defined, sorted by name.
func allForms() []formular {
	raw, err := plugin.List(praefixFormular, 200)
	if err != nil {
		return nil
	}
	out := make([]formular, 0, len(raw))
	for _, v := range raw {
		var f formular
		if json.Unmarshal([]byte(v), &f) == nil && f.Key != "" {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// clean brings a form into a state in which it can be output and read back in.
func (f formular) clean() formular {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		f.Name = "Formular"
	}
	if len(f.Name) > 80 {
		f.Name = f.Name[:80]
	}
	f.Subject = strings.TrimSpace(f.Subject)
	f.Dank = strings.TrimSpace(f.Dank)

	fields := make([]field, 0, len(f.Fields))
	belegt := map[string]bool{}
	for _, fe := range f.Fields {
		fe.Label = strings.TrimSpace(fe.Label)
		if fe.Label == "" {
			continue
		}
		if artName(fe.Art) == fe.Art {
			fe.Art = ArtText
		}
		if fe.Key == "" {
			fe.Key = keyFrom(fe.Label)
		}
		if fe.Key == "" {
			continue
		}
		// Two fields with the same key would overwrite each other on receipt —
		// the second answer would never arrive.
		if belegt[fe.Key] {
			continue
		}
		belegt[fe.Key] = true

		if fe.Art != KindChoice {
			fe.Choices = nil
		} else {
			choices := make([]string, 0, len(fe.Choices))
			for _, w := range fe.Choices {
				if w = strings.TrimSpace(w); w != "" {
					choices = append(choices, w)
				}
			}
			if len(choices) == 0 {
				// A choice with no options is a field nobody can fill in.
				fe.Art = ArtText
			}
			fe.Choices = choices
		}
		fields = append(fields, fe)
		if len(fields) >= maxFields {
			break
		}
	}
	f.Fields = fields
	return f
}
