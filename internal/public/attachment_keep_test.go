package public

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// A keep that overruns the plugin's call does not take the message with it
// (WR-03 of the phase 38 review).
//
// files.keep runs inside the plugin's call, whose deadline is two seconds and
// whose module the engine closes when the deadline passes. A keep that took
// longer (a 64 MB video on a slow node) failed its database work on the expired
// deadline, the closed module failed the whole call, the breaker counted it,
// and the visitor got a 404 for a message that was lost. Now the keep answers
// the plugin before the deadline with what was stored so far, the plugin
// stores the message without the rest, and what the late keep still writes is
// taken away again, so no file is left that no message names.
func TestAKeepThatOverrunsTheCallStoresTheMessageWithoutTheFile(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)
	h.SetUploadLimits(5<<20, 50<<20)

	release := make(chan struct{})
	var once sync.Once
	letGo := func() { once.Do(func() { close(release) }) }
	keepHook = func() {
		// Longer than the whole call, unless the test lets it go first.
		select {
		case <-release:
		case <-time.After(3 * time.Second):
		}
	}
	t.Cleanup(func() { keepHook = nil; letGo() })

	form := func() ([]byte, string) {
		return rawMultipart(t, false,
			fld("seite", "kontakt"), fld("pfad", "/kontakt"),
			fld("gestellt", timeToken(t, database, -10*time.Second)),
			fld("name", "Eva Muster"), fld("email", "eva@example.test"),
			fld("nachricht", "Mit Bild."),
			filePart("anhang", "bild.png", onePixel(t)))
	}
	mediaRows := func() int {
		var n int
		if err := database.Read.QueryRow(`SELECT COUNT(*) FROM media WHERE owner_plugin = 'kontaktformular'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	body, ctype := form()
	start := time.Now()
	rec := postMultipart(t, h, ws, "/formular", body, ctype)
	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("a keep over the call's time: %d %q, want 303 formular=gesendet", rec.Code, ort)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("the answer took %v, longer than the plugin's call may", took)
	}
	if n := messages(t, database); n != 1 {
		t.Fatalf("%d messages stored, want the one without its file", n)
	}

	// The late keep finishes, finds it came too late, and takes its file away;
	// then the spool goes and the slot is given back.
	letGo()
	deadline := time.Now().Add(5 * time.Second)
	for {
		inside, _ := held(h.uploads)
		if mediaRows() == 0 && len(spoolEntries(t, h.dataDir)) == 0 && inside == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after the late keep: %d media rows, spool %v, %d inside; want none of each",
				mediaRows(), spoolEntries(t, h.dataDir), inside)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The plugin was not held back for it: the next message keeps its file.
	keepHook = nil
	body, ctype = form()
	rec = postMultipart(t, h, ws, "/formular", body, ctype)
	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("the next message: %d %q, want 303 formular=gesendet", rec.Code, ort)
	}
	if n := mediaRows(); n != 1 {
		t.Errorf("%d media rows after an ordinary message with a file, want 1", n)
	}
}
