package plugin

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// These tests run natively, not in WebAssembly. That is exactly what they are
// meant to show: a plugin author can check their hooks with an ordinary
// `go test` and only needs the wasm toolchain once they want the module.

func reset() {
	onContent, onRequest, onRoute, onAdmin, onEvent = nil, nil, nil, nil, nil
	warned = false
	SetTestHost(nil)
}

func TestTheContentHookRunsThroughTheRealEnvelope(t *testing.T) {
	reset()
	OnContent(func(in ContentIn) (ContentOut, error) {
		html := strings.ReplaceAll(in.HTML, "[[jahr]]", "2026")
		return ContentOut{HTML: html, Changed: html != in.HTML}, nil
	})

	in, _ := json.Marshal(ContentIn{WebsiteID: 3, Slug: "home", HTML: "© [[jahr]]"})
	raw, err := Dispatch("content", in)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	var out ContentOut
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Changed || out.HTML != "© 2026" {
		t.Errorf("answer: %+v", out)
	}
}

func TestAHookThatIsNotRegisteredStaysSilent(t *testing.T) {
	reset()
	OnContent(func(ContentIn) (ContentOut, error) { return ContentOut{}, nil })

	// "request" is not registered. That is not an error: a manifest may name a
	// hook that only a later version handles.
	raw, err := Dispatch("request", []byte(`{}`))
	if err != nil || raw != nil {
		t.Errorf("raw=%q err=%v", raw, err)
	}
}

func TestAnUnknownHookIsAnError(t *testing.T) {
	reset()
	OnContent(func(ContentIn) (ContentOut, error) { return ContentOut{}, nil })
	if _, err := Dispatch("gibtsnicht", []byte(`{}`)); err == nil {
		t.Error("an unknown hook was accepted")
	}
}

func TestAnErrorFromTheHookIsPassedOn(t *testing.T) {
	reset()
	own := errors.New("something went wrong")
	OnContent(func(ContentIn) (ContentOut, error) { return ContentOut{}, own })
	if _, err := Dispatch("content", []byte(`{}`)); !errors.Is(err, own) {
		t.Errorf("wanted %v, got %v", own, err)
	}
}

func TestNotRegisteringAnythingIsWarnedAbout(t *testing.T) {
	reset()
	var lines []string
	SetTestHost(func(op string, arg []byte) ([]byte, error) {
		if op == "log" {
			var a struct{ Message string }
			json.Unmarshal(arg, &a)
			lines = append(lines, a.Message)
		}
		return nil, nil
	})

	Dispatch("content", []byte(`{}`))
	Dispatch("content", []byte(`{}`))

	// The mistake every author makes exactly once. Without this warning the
	// plugin is silent and looks correct, and the only symptom is a feature
	// that does not happen.
	if len(lines) != 1 {
		t.Fatalf("%d warnings, wanted exactly one: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "init") || !strings.Contains(lines[0], "main") {
		t.Errorf("the warning does not name the cause: %q", lines[0])
	}
}

func TestStorageCallsGoOutAsJSON(t *testing.T) {
	reset()
	var seen []string
	SetTestHost(func(op string, arg []byte) ([]byte, error) {
		seen = append(seen, op+" "+string(arg))
		switch op {
		case "store.get":
			return json.Marshal(map[string]any{"value": "grün", "found": true})
		case "store.list":
			return json.Marshal(map[string]string{"a": "1", "b": "2"})
		}
		return nil, nil
	})

	if v, ok, err := Get("farbe"); err != nil || !ok || v != "grün" {
		t.Errorf("Get: %q %v %v", v, ok, err)
	}
	if err := Set("farbe", "blau"); err != nil {
		t.Errorf("Set: %v", err)
	}
	if err := GlobalSet("fassung", "2"); err != nil {
		t.Errorf("GlobalSet: %v", err)
	}
	m, err := List("prefix", 10)
	if err != nil || len(m) != 2 {
		t.Errorf("List: %v %v", m, err)
	}

	// The global space has to go out as such, or a setting meant for the whole
	// installation lands on a single website.
	if !strings.Contains(seen[2], `"global":true`) {
		t.Errorf("GlobalSet did not go out globally: %s", seen[2])
	}
	if strings.Contains(seen[1], `"global"`) {
		t.Errorf("Set went out global without needing to: %s", seen[1])
	}
}

func TestARefusedPermissionArrivesAsErrDenied(t *testing.T) {
	reset()
	SetTestHost(func(string, []byte) ([]byte, error) { return nil, ErrDenied })

	// A plugin that appears to store and does not is worse than one that stops
	// — the error has to reach the author.
	if err := Set("x", "y"); !errors.Is(err, ErrDenied) {
		t.Errorf("wanted ErrDenied, got: %v", err)
	}
	if _, _, err := Get("x"); !errors.Is(err, ErrDenied) {
		t.Errorf("wanted ErrDenied, got: %v", err)
	}
}

func TestWithoutAHostTheErrorIsAClearOne(t *testing.T) {
	reset()
	if err := Set("x", "y"); err == nil {
		t.Error("without a host a write was reported as though it had succeeded")
	}
}

func TestTheEventHookGetsTheData(t *testing.T) {
	reset()
	var seen EventIn
	OnEvent(func(in EventIn) error { seen = in; return nil })

	in, _ := json.Marshal(EventIn{Name: EventNotFound, WebsiteID: 4,
		Data: map[string]string{"path": "/alte-seite"}})
	if _, err := Dispatch("event", in); err != nil {
		t.Fatal(err)
	}
	if seen.Name != EventNotFound || seen.Data["path"] != "/alte-seite" {
		t.Errorf("event: %+v", seen)
	}
}
