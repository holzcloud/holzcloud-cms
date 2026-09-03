// Ein Plugin, das zeigt, wie wenig ein Plugin sein muss.
//
// Es ersetzt [[jahr]] im Seitentext durch das laufende Jahr und zählt mit, wie
// oft es das getan hat. Damit berührt es beide Seiten der Schnittstelle — den
// Inhalt und den eigenen Speicher — und ist trotzdem in dreissig Zeilen lesbar.
package main

import (
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

const marke = "[[jahr]]"

// init und nicht main: der Host startet ein Plugin als Reaktor-Modul, das
// heisst er ruft _initialize auf. Das führt die Paket-Initialisierung aus und
// kehrt zurück — main läuft nie. Wer hier main schriebe, bekäme ein Plugin,
// das sich einspielen und einschalten lässt und dann bei jedem Haken nichts
// tut. Das SDK sagt es einem beim ersten Aufruf ins Protokoll.
func init() {
	plugin.OnContent(func(in plugin.ContentIn) (plugin.ContentOut, error) {
		if !strings.Contains(in.HTML, marke) {
			// Nichts zu tun ist die häufigste Antwort. Sie kostet den Host
			// einen Umlauf und keine Kopie der Seite.
			return plugin.ContentOut{}, nil
		}
		jahr := strconv.Itoa(time.Now().Year())
		html := strings.ReplaceAll(in.HTML, marke, jahr)

		// Mitzählen, wie oft. Ein Fehler dabei darf die Seite nicht kosten:
		// der Besucher will das Jahr sehen, nicht unsere Buchhaltung.
		if n, _, err := plugin.Get("ersetzungen"); err == nil {
			z, _ := strconv.Atoi(n)
			_ = plugin.Set("ersetzungen", strconv.Itoa(z+1))
		}
		return plugin.ContentOut{HTML: html, Changed: true}, nil
	})

	plugin.OnAdmin(func(in plugin.AdminIn) (plugin.AdminOut, error) {
		n, _, _ := plugin.Get("ersetzungen")
		if n == "" {
			n = "0"
		}
		return plugin.AdminOut{
			Title: "Jahreszahl",
			HTML: "<p>Schreibe <code>" + marke + "</code> in eine Seite; " +
				"beim Ausliefern steht dort das laufende Jahr.</p>" +
				"<p>Bisher ersetzt: <strong>" + n + "</strong></p>",
		}, nil
	})
}

func main() {}
