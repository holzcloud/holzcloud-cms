package web

import (
	"net/http/httptest"
	"testing"
)

// Set replaced; Add duplicated. This one keeps what is there and says each
// name once.
func TestAddVaryKeepsWhatIsAlreadyThereAndSaysItOnce(t *testing.T) {
	for _, c := range []struct {
		name   string
		before []string
		add    []string
		want   string
	}{
		{"nothing there", nil, []string{"HX-Request"}, "HX-Request"},
		{"the session's Cookie survives", []string{"Cookie"}, []string{"HX-Request"}, "Cookie, HX-Request"},
		{"two libraries both added Cookie", []string{"Cookie", "Cookie"}, []string{"HX-Request"}, "Cookie, HX-Request"},
		{"a list in one line", []string{"Cookie, Accept-Language"}, []string{"HX-Request"}, "Cookie, Accept-Language, HX-Request"},
		{"the same name twice, in any case", []string{"cookie"}, []string{"Cookie"}, "cookie"},
		{"already named", []string{"HX-Request"}, []string{"HX-Request"}, "HX-Request"},
		{"a star swallows the rest", []string{"Cookie"}, []string{"*"}, "*"},
		{"nothing to add, nothing to say", nil, nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			for _, v := range c.before {
				rec.Header().Add("Vary", v)
			}
			AddVary(rec, c.add...)
			if got := rec.Header().Get("Vary"); got != c.want {
				t.Errorf("Vary = %q, want %q", got, c.want)
			}
			if n := len(rec.Header().Values("Vary")); n > 1 {
				t.Errorf("Vary stands on %d lines; it should be one", n)
			}
		})
	}
}
