package plugin

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Diese Tests laufen nativ, nicht in WebAssembly. Genau das sollen sie zeigen:
// ein Plugin-Autor kann seine Haken mit gewöhnlichem `go test` prüfen und
// braucht die wasm-Werkzeugkette erst, wenn er das Modul haben will.

func zuruecksetzen() {
	onContent, onRequest, onRoute, onAdmin, onEvent = nil, nil, nil, nil, nil
	warned = false
	SetTestHost(nil)
}

func TestInhaltsHakenLaeuftUeberDieEchteVerpackung(t *testing.T) {
	zuruecksetzen()
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
		t.Errorf("Antwort: %+v", out)
	}
}

func TestNichtRegistrierterHakenSchweigt(t *testing.T) {
	zuruecksetzen()
	OnContent(func(ContentIn) (ContentOut, error) { return ContentOut{}, nil })

	// "request" ist nicht registriert. Das ist kein Fehler: das Manifest darf
	// einen Haken nennen, den erst eine spätere Fassung behandelt.
	raw, err := Dispatch("request", []byte(`{}`))
	if err != nil || raw != nil {
		t.Errorf("raw=%q err=%v", raw, err)
	}
}

func TestUnbekannterHakenIstEinFehler(t *testing.T) {
	zuruecksetzen()
	OnContent(func(ContentIn) (ContentOut, error) { return ContentOut{}, nil })
	if _, err := Dispatch("gibtsnicht", []byte(`{}`)); err == nil {
		t.Error("ein unbekannter Haken wurde angenommen")
	}
}

func TestFehlerAusDemHakenWirdWeitergereicht(t *testing.T) {
	zuruecksetzen()
	eigen := errors.New("etwas ging schief")
	OnContent(func(ContentIn) (ContentOut, error) { return ContentOut{}, eigen })
	if _, err := Dispatch("content", []byte(`{}`)); !errors.Is(err, eigen) {
		t.Errorf("erwartet %v, bekommen %v", eigen, err)
	}
}

func TestOhneRegistrierungWirdGewarnt(t *testing.T) {
	zuruecksetzen()
	var zeilen []string
	SetTestHost(func(op string, arg []byte) ([]byte, error) {
		if op == "log" {
			var a struct{ Message string }
			json.Unmarshal(arg, &a)
			zeilen = append(zeilen, a.Message)
		}
		return nil, nil
	})

	Dispatch("content", []byte(`{}`))
	Dispatch("content", []byte(`{}`))

	// Der Fehler, den jeder Autor genau einmal macht. Ohne diese Warnung ist
	// das Plugin still und sieht richtig aus, und das einzige Anzeichen ist
	// eine Funktion, die nicht passiert.
	if len(zeilen) != 1 {
		t.Fatalf("%d Warnungen, erwartet genau eine: %v", len(zeilen), zeilen)
	}
	if !strings.Contains(zeilen[0], "init") || !strings.Contains(zeilen[0], "main") {
		t.Errorf("die Warnung nennt die Ursache nicht: %q", zeilen[0])
	}
}

func TestSpeicherzugriffeGehenAlsJSONHinaus(t *testing.T) {
	zuruecksetzen()
	var gesehen []string
	SetTestHost(func(op string, arg []byte) ([]byte, error) {
		gesehen = append(gesehen, op+" "+string(arg))
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

	// Der globale Raum muss als solcher hinausgehen, sonst landet eine
	// Einstellung bei einer einzelnen Website.
	if !strings.Contains(gesehen[2], `"global":true`) {
		t.Errorf("GlobalSet ging nicht global hinaus: %s", gesehen[2])
	}
	if strings.Contains(gesehen[1], `"global"`) {
		t.Errorf("Set ging unnötig global hinaus: %s", gesehen[1])
	}
}

func TestVerweigerteBerechtigungKommtAlsErrDenied(t *testing.T) {
	zuruecksetzen()
	SetTestHost(func(string, []byte) ([]byte, error) { return nil, ErrDenied })

	// Ein Plugin, das scheinbar speichert und es nicht tut, ist schlimmer als
	// eines, das aufhört — der Fehler muss beim Autor ankommen.
	if err := Set("x", "y"); !errors.Is(err, ErrDenied) {
		t.Errorf("erwartet ErrDenied, bekommen: %v", err)
	}
	if _, _, err := Get("x"); !errors.Is(err, ErrDenied) {
		t.Errorf("erwartet ErrDenied, bekommen: %v", err)
	}
}

func TestOhneHostGibtEsEinenKlarenFehler(t *testing.T) {
	zuruecksetzen()
	if err := Set("x", "y"); err == nil {
		t.Error("ohne Host wurde ein Schreibvorgang gemeldet, als sei er gelungen")
	}
}

func TestEreignisHakenBekommtDieDaten(t *testing.T) {
	zuruecksetzen()
	var gesehen EventIn
	OnEvent(func(in EventIn) error { gesehen = in; return nil })

	in, _ := json.Marshal(EventIn{Name: EventNotFound, WebsiteID: 4,
		Data: map[string]string{"path": "/alte-seite"}})
	if _, err := Dispatch("event", in); err != nil {
		t.Fatal(err)
	}
	if gesehen.Name != EventNotFound || gesehen.Data["path"] != "/alte-seite" {
		t.Errorf("Ereignis: %+v", gesehen)
	}
}
