package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	formTagPattern   = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	buttonTagPattern = regexp.MustCompile(`(?is)<(button|a)\b[^>]*>`)
	attrPattern      = regexp.MustCompile(`(?is)\b([a-z-]+)\s*=\s*"([^"]*)"`)
	// confirmTagPattern is the opening tag of any element that carries
	// hx-confirm, whatever the element.
	confirmTagPattern = regexp.MustCompile(`(?is)<[a-z][a-z0-9-]*\b[^>]*\bhx-confirm\b[^>]*>`)
)

func attrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrPattern.FindAllStringSubmatch(tag, -1) {
		out[strings.ToLower(m[1])] = m[2]
	}
	return out
}

func adminTemplateFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join("..", "..", "cmd", "holzcloud", "templates", "admin")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read admin templates: %v", err)
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		files[e.Name()] = string(content)
	}
	return files
}

// Every state-changing action must work without JavaScript.
//
// htmx is progressive enhancement, not a dependency: a form carrying only
// hx-post silently does nothing when the script is unavailable — which is
// exactly how page delete, publish toggle and domain removal became inert.
// A plain method/action pair keeps them working either way.
func TestEveryHxPostFormAlsoSubmitsWithoutJS(t *testing.T) {
	for name, content := range adminTemplateFiles(t) {
		for _, tag := range formTagPattern.FindAllString(content, -1) {
			a := attrs(tag)
			if _, ok := a["hx-post"]; !ok {
				continue
			}
			if !strings.EqualFold(a["method"], "post") || a["action"] == "" {
				t.Errorf("%s: form has hx-post but no method/action fallback:\n  %s", name, strings.TrimSpace(tag))
				continue
			}
			// And to the SAME place. An action fallback that goes somewhere
			// else is worse than none: the form works with the script and
			// works without it, and does two different things — which is the
			// one failure nobody reports, because both paths answer 200.
			if verb := a["hx-post"]; verb != a["action"] {
				t.Errorf("%s: hx-post goes to %q and the form's action to %q — "+
					"with the script the request lands in one place and without it in "+
					"the other:\n  %s", name, verb, a["action"], strings.TrimSpace(tag))
			}
		}
	}
}

// hx-confirm is the one place the enhancement-only rule is knowingly bent, and
// it is written down here because nothing else in the tree says so.
//
// With the script gone there is no confirmation step at all — the button
// simply posts. The action still happens, which is why this is not a
// correctness defect: the no-JS path does what the control says it does. What
// is lost is the second thought, on twenty-two destructive controls.
//
// The test does not forbid it. It counts it, so that the number is a thing
// somebody chose rather than a thing that grew: a jump means a new destructive
// control was given a confirmation that exists only with JavaScript, and that
// is worth one deliberate look at whether the action deserves a real
// confirmation page instead.
//
// The second half of the rule: an hx-confirm is only ever asked by htmx, and
// htmx only handles an element that issues a request itself. On a form without
// an htmx verb the browser submits natively and the question never appears —
// with the script loaded and working. Seven controls were in that state until
// the audit of 2026-11-10 (Part 1, item 11): the media list's "Delete anyway",
// the delete buttons of products, block kinds, content kinds, fields and
// language files, and forgetting a saved view of the page list.
// TestEveryHxConfirmSitsOnTheElementThatIssuesTheRequest holds it.
const knownHxConfirms = 22

func TestHxConfirmIsTheKnownDegradation(t *testing.T) {
	const expected = knownHxConfirms

	found := map[string]int{}
	total := 0
	for name, content := range adminTemplateFiles(t) {
		if n := strings.Count(content, "hx-confirm"); n > 0 {
			found[name] = n
			total += n
		}
	}
	if total != expected {
		t.Errorf("hx-confirm now appears %d times, not %d: %v\n"+
			"Not a failure of the code — a prompt. Each of these is a destructive "+
			"control whose confirmation disappears with the script. If the new one "+
			"deserves a confirmation that works without JavaScript, give it a page; "+
			"if it does not, raise the number here.", total, expected, found)
	}
}

// An hx-confirm on an element that issues no request is a question nobody is
// ever asked: htmx does not handle the element, the browser submits it, and the
// destructive action happens on the first click. See knownHxConfirms.
//
// The number of tags looked at is held to the known count, so a pattern that
// stops matching cannot turn this green; and an hx-confirm the tag pattern does
// not see at all — in a comment, in an attribute value with a ">" in it — is
// reported by the per-file comparison.
func TestEveryHxConfirmSitsOnTheElementThatIssuesTheRequest(t *testing.T) {
	verbs := []string{"hx-post", "hx-put", "hx-delete", "hx-patch", "hx-get"}
	inspected := 0
	for name, content := range adminTemplateFiles(t) {
		tags := confirmTagPattern.FindAllString(content, -1)
		if n := strings.Count(content, "hx-confirm"); n != len(tags) {
			t.Errorf("%s: hx-confirm appears %d times, but %d tags carrying it were found — "+
				"one sits where this test cannot see whether it is asked", name, n, len(tags))
		}
		for _, tag := range tags {
			inspected++
			a := attrs(tag)
			issues := false
			for _, verb := range verbs {
				if _, ok := a[verb]; ok {
					issues = true
					break
				}
			}
			if !issues {
				t.Errorf("%s: hx-confirm on an element without an htmx verb — the browser "+
					"submits it natively and the question never appears:\n  %s",
					name, strings.TrimSpace(tag))
			}
		}
	}
	if inspected != knownHxConfirms {
		t.Errorf("inspected %d tags carrying hx-confirm, want %d", inspected, knownHxConfirms)
	}
}

// The media card is a grid column of at least 220px with overflow: hidden. Its
// action row holds the address field, the crop link and two delete buttons; in
// one line the second delete button was cut off by the card's edge. The row
// has to wrap.
func TestTheMediaCardActionsWrap(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "holzcloud", "assets", "admin.css"))
	if err != nil {
		t.Fatalf("read admin.css: %v", err)
	}
	rule := regexp.MustCompile(`(?s)\.media-card__actions\s*\{([^}]*)\}`).FindStringSubmatch(string(raw))
	if rule == nil {
		t.Fatal("no .media-card__actions rule in admin.css")
	}
	if !regexp.MustCompile(`(?m)^\s*flex-wrap:\s*wrap\s*;`).MatchString(rule[1]) {
		t.Errorf(".media-card__actions does not wrap, so a control is cut off at 220px:\n%s", rule[1])
	}
}

// A control that changes state must still work with the script gone.
//
// There are exactly two shapes that satisfy that. A <button type="submit">
// submits its enclosing form on a plain click, whatever htmx does or does not
// do — that is the shape a fragment uses, because the form is in the template
// that includes it and not in the fragment's own file. Anything else — a bare
// <a hx-post>, a <button type="button"> — has no path at all without the
// script, so it has to sit in a file that visibly contains the POST form.
func TestNoStateChangingControlOutsideAForm(t *testing.T) {
	for name, content := range adminTemplateFiles(t) {
		for _, tag := range buttonTagPattern.FindAllString(content, -1) {
			a := attrs(tag)
			for _, verb := range []string{"hx-post", "hx-put", "hx-delete", "hx-patch"} {
				if _, ok := a[verb]; !ok {
					continue
				}
				if strings.EqualFold(a["type"], "submit") {
					// Submits the enclosing form on a plain click. A
					// formaction would send it somewhere else, so that has to
					// be a POST target too — but no template uses one, and if
					// one ever does this check should be revisited.
					if _, ok := a["formaction"]; !ok {
						continue
					}
				}
				if !strings.Contains(content, `method="POST"`) {
					t.Errorf("%s: <%s> with %s and no non-JS path:\n  %s",
						name, strings.Split(tag, " ")[0][1:], verb, strings.TrimSpace(tag))
				}
			}
		}
	}
}

// The vendored htmx asset must be the real library.
//
// It was a 47-byte placeholder comment from the commit that introduced it,
// which made every hx-* attribute in the admin inert in a browser while the
// source looked perfectly correct. Nothing in the UI surfaced that, so only an
// assertion catches it coming back.
//
// See cmd/holzcloud/assets/VENDOR.md for provenance and the update procedure.
func TestVendoredHtmxIsTheRealLibrary(t *testing.T) {
	path := filepath.Join("..", "..", "cmd", "holzcloud", "assets", "htmx.min.js")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("htmx asset missing entirely: %v", err)
	}

	// The minified library is around 50 kB; anything tiny is a stub.
	const minPlausibleSize = 20000
	if len(content) < minPlausibleSize {
		t.Fatalf("htmx.min.js is %d bytes, which cannot be the library — every hx-* "+
			"attribute in the admin is inert. Content: %q",
			len(content), strings.TrimSpace(string(content)))
	}

	// Spot-check that it is htmx and not some other minified bundle.
	head := string(content[:min(len(content), 200)])
	if !strings.Contains(head, "htmx") {
		t.Errorf("htmx.min.js does not look like htmx: %q", head)
	}
	for _, marker := range []string{"hx-swap", "hx-target", "hx-confirm", "hx-headers"} {
		if !strings.Contains(string(content), marker) {
			t.Errorf("htmx.min.js does not handle %s, which the admin templates use", marker)
		}
	}
}

// Every complete document in the admin has to name an icon.
//
// If it does not, the browser asks for /favicon.ico of its own accord and gets
// a 404 on every single page view — in the server's log and in the operator's
// console. That was the case for a long time and came to light while reading
// through the templates, not while using them: a 404 on an image is not
// something you can see on the page.
func TestEveryAdminDocumentNamesAFavicon(t *testing.T) {
	dir := filepath.Join("..", "..", "cmd", "holzcloud", "templates", "admin")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read admin templates: %v", err)
	}

	var checked int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		// Complete documents only. A page fragment has no head for an icon to
		// belong in.
		if !strings.Contains(strings.ToLower(string(body)), "<!doctype") {
			continue
		}
		checked++
		if !strings.Contains(string(body), `rel="icon"`) {
			t.Errorf("%s ist ein vollständiges Dokument ohne <link rel=\"icon\"> — "+
				"jeder Aufruf erzeugt damit einen 404 auf /favicon.ico", e.Name())
		}
	}

	// Without this line the test would be green if the templates were renamed
	// or moved and the loop found nothing at all any more.
	if checked < 5 {
		t.Fatalf("only %d complete documents found; the search no longer bites", checked)
	}
}
