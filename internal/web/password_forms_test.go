package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// hxRequestAttribute is an htmx attribute that turns a form into an htmx
// request. hx-disabled-elt on the button is not one: it only stops a second
// click and changes nothing about how the answer is shown.
var hxRequestAttribute = regexp.MustCompile(`(?i)\bhx-(post|put|patch|boost)\b`)

// passwordForms are the admin screens that carry a password the server checks
// or hashes: the sign-in, its second step, the confirmation, setting a password
// from a link, the first set-up and a person's own password.
var passwordForms = []string{
	"login.html",
	"two_factor_verify.html",
	"confirm.html",
	"set_password.html",
	"setup.html",
	"user_password.html",
}

// The password forms stay plain POST forms.
//
// Phase 37 added answers a browser has to show: 429 when too many attempts
// were made, 503 when every password check is in use, 422 for a wrong
// password. htmx 2.0.11 does not swap a 4xx or 5xx answer into the page, so a
// password form that became an htmx form (hx-post, hx-boost) would show
// nothing at all on exactly those answers: the owner would press the button
// again and again, blind, and spend the attempts the throttle just counted.
// A plain method/action form makes the browser render the body, whatever the
// status. The same holds for the gate of a protected page in every shipped
// theme, which answers 429 and 503 with the gate itself.
//
// This is a guard, not a regression test: it passes on the tree as it was
// before the phase, by design.
func TestPasswordFormsArePlainPosts(t *testing.T) {
	files := adminTemplateFiles(t)
	for _, name := range passwordForms {
		content, ok := files[name]
		if !ok {
			t.Errorf("%s: no such admin template; the list of password forms is out of date", name)
			continue
		}
		forms := formTagPattern.FindAllString(content, -1)
		if len(forms) == 0 {
			t.Errorf("%s: no form found; the guard would pass on nothing", name)
		}
		for _, tag := range forms {
			a := attrs(tag)
			if !strings.EqualFold(a["method"], "post") || a["action"] == "" {
				t.Errorf("%s: a password form must be a plain POST with an action:\n  %s", name, strings.TrimSpace(tag))
			}
			if hxRequestAttribute.MatchString(tag) {
				t.Errorf("%s: a password form carries an htmx request attribute, so a 429, 503 or 422 answer would not be shown:\n  %s", name, strings.TrimSpace(tag))
			}
		}
		if m := hxRequestAttribute.FindString(content); m != "" {
			t.Errorf("%s: %s appears in a template with a password form; the browser must submit it natively", name, m)
		}
	}

	gates, err := filepath.Glob(filepath.Join("..", "..", "cmd", "holzcloud", "templates", "public", "*", "gate.html"))
	if err != nil {
		t.Fatalf("glob gates: %v", err)
	}
	if len(gates) < 8 {
		t.Errorf("found %d shipped gate.html files, want at least the eight themes", len(gates))
	}
	hxAny := regexp.MustCompile(`(?i)\bhx-[a-z-]+`)
	for _, path := range gates {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(raw)
		if m := hxAny.FindString(content); m != "" {
			t.Errorf("%s: the gate carries %s; a public gate is a plain POST form", path, m)
		}
		for _, tag := range formTagPattern.FindAllString(content, -1) {
			a := attrs(tag)
			if !strings.EqualFold(a["method"], "post") || a["action"] != "/freischalten" {
				t.Errorf("%s: the gate form must be a plain POST to /freischalten:\n  %s", path, strings.TrimSpace(tag))
			}
		}
		if len(formTagPattern.FindAllString(content, -1)) == 0 {
			t.Errorf("%s: no form found; the guard would pass on nothing", path)
		}
	}
}
