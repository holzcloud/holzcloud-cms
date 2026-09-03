---
phase: quick-260903-rqq
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd/holzcloud/templates/public/weide/style.css        # ersetzt, 2332 Zeilen -> neue helle Fassung
  - cmd/holzcloud/templates/public/weide/layout.html      # ersetzt
  - cmd/holzcloud/templates/public/weide/page.html        # ersetzt
  - cmd/holzcloud/templates/public/weide/home.html        # ersetzt
  - cmd/holzcloud/templates/public/weide/list.html        # ersetzt
  - cmd/holzcloud/templates/public/weide/search.html      # ersetzt
  - cmd/holzcloud/templates/public/weide/gate.html        # ersetzt
  - cmd/holzcloud/templates/public/weide/404.html         # ersetzt
  - cmd/holzcloud/templates/public/weide/maintenance.html # ersetzt
  - cmd/holzcloud/templates/public/weide/shop.html        # ersetzt
  - cmd/holzcloud/templates/public/weide/product.html     # ersetzt
  - cmd/holzcloud/templates/public/weide/cart.html        # ersetzt
  - cmd/holzcloud/templates/public/weide/checkout.html    # ersetzt
  - cmd/holzcloud/templates/public/weide/order.html       # ersetzt
  - cmd/holzcloud/templates/public/weide/favicon.svg      # neu
  - cmd/holzcloud/templates/public/weide/fonts/manrope-latin.woff2      # neu, Kopie
  - cmd/holzcloud/templates/public/weide/fonts/manrope-latin-ext.woff2  # neu, Kopie
  - cmd/holzcloud/assets/VENDOR.md                        # Schriften-Abschnitt: zweiter Ort
autonomous: true
requirements: [QUICK-260903-rqq]

estimate:
  tokens: 46000
  raw_tokens: 46000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Eine Besucherin oeffnet eine Seite der Vorlage `weide` und sieht warmes Papier, Erdbraun und Manrope — keinen Systemschriftstapel und keine schmale Blogspalte (D-04, D-08)."
    - "`holzcloud template check cmd/holzcloud/templates/public/weide` meldet „no problems found\" — dasselbe Urteil, das ein Upload gaebe."
    - "Kein Byte kommt von einem fremden Ursprung: Manrope liegt in fonts/ der Vorlage und wird ueber /t/fonts/ ausgeliefert (D-08, D-19)."
    - "Die Startseite zeigt neben der Schlagzeile das Titelbild der Seite; ohne Titelbild eine schmueckende Huegelzeichnung. Beide Zweige rendert die Pruefung, weil SampleData ein OGImage traegt und MinimalData keines (D-14)."
    - "Alle zwoelf Ansichten sind erhalten und in die neue Sprache uebersetzt; keine ist verschwunden (D-16)."
    - "Die schwaechste Tinte erfuellt AA fuer die kleinste Schriftgroesse — auf hellem Papier gemessen, nicht aus holzcloud uebernommen (D-07)."
    - "Ein in der Verwaltung eingestellter Regler gewinnt gegen die Vorlage, und eine eingestellte Marke zieht die halbe Palette mit (D-02, D-06, D-20)."
    - "Die neun Bausteinarten sind ueber die Bruecke gekleidet, nicht einzeln nachgebaut (D-03, D-18)."
  artifacts:
    - cmd/holzcloud/templates/public/weide/style.css
    - cmd/holzcloud/templates/public/weide/layout.html
    - cmd/holzcloud/templates/public/weide/page.html
    - cmd/holzcloud/templates/public/weide/home.html
    - cmd/holzcloud/templates/public/weide/favicon.svg
    - cmd/holzcloud/templates/public/weide/fonts/manrope-latin.woff2
    - cmd/holzcloud/templates/public/weide/fonts/manrope-latin-ext.woff2
    - cmd/holzcloud/assets/VENDOR.md
  key_links:
    - "@font-face -> /t/fonts/manrope-latin.woff2 -> internal/web/asset.go setzt font/woff2, nosniff und einen Jahres-Cache"
    - "internal/design/tokens.go -> <style> nach /t/style.css -> die vier Brueckenzeilen der CMS-Schicht"
    - "cmd/holzcloud/assets/bausteine.css liest --ink/--paper/--brand/--line/--space-*/--step-*/--radius -> zehn Brueckenzeilen kleiden alle neun Bausteinarten"
    - "internal/public/pagedata.go withOGImage -> .Meta.OGImage -> das Bild im Aufmacher von home.html"
    - "internal/template/render_test.go TestShippedThemesRenderEveryView kennt `weide` bereits — eine kaputte Vorlage faellt in CI auf, nicht beim Besucher"
---

<objective>
Die mitgelieferte oeffentliche Vorlage `weide` wird neu gestaltet: dieselbe
Architektur und dieselben Gesten wie die Vorlage `holzcloud`, aber warmes
Papier statt dunklem Grund und Erdbraun statt Messing.

Purpose: Die heutige Fassung sieht aus wie ein Blog — Systemschriftstapel,
schmale linke Textspalte, Ueberschrift-Bild-Absatz-Link im Wechsel, keine
Flaechen, kein Rhythmus. Die Formensprache von `holzcloud` ist im Repository
bereits fertig, gepruefte und ohne Skript; sie wird hier ins Helle uebersetzt,
nicht neu erfunden.

Output: 15 Dateien in `cmd/holzcloud/templates/public/weide/` (zwoelf
Ansichten, `layout.html`, `style.css`, `favicon.svg`) plus zwei kopierte
woff2 in `fonts/`, dazu eine Ergaenzung im Schriften-Abschnitt von VENDOR.md.
Kein Go-Code aendert sich: `weide` steht bereits in `BuiltinTemplates`
(internal/template/loader.go:519) und bereits in der Testreihe
(internal/template/render_test.go:195).

## Die Entscheidungen des Anwenders

Diese Nummern werden in den Aufgaben zitiert. Sie sind nicht verhandelbar.

- **D-01** Architektur von `holzcloud`: eine `style.css`, `@font-face` zuerst,
  dann die Gestaltungsschichten in `@layer`, dann die CMS-Schicht ganz unten
  OHNE jede `@layer`. Drei Stufen: Schichten < CMS-Schicht < `.Site.Design`.
- **D-02** Bruecke zu den sechs Reglern der Verwaltung, woertlich nach dem
  Muster in holzcloud/style.css. Richtung immer vom Regler zum System, nie
  zurueck (kein Zyklus).
- **D-03** Bruecke zu den Bausteinen ueber die generischen Namen, die
  `bausteine.css` ableitet. Nur wo daraus etwas Falsches folgt, kommt eine
  gezielte Regel dazu — keine `.hc-block`-Regeln nachbauen.
- **D-04** Warmes Gebrochenweiss als Papier, sehr dunkles Warmbraun als Tinte,
  Erdbraun als Marke. Kein reines Schwarz auf reinem Weiss.
- **D-05** Der Grund ist nicht flach: drei sehr leise radiale Farbschleier
  (Braun, Strohgelb, Salbeigruen) ueber einem hellen Verlauf, jeder zwischen
  6 % und 10 % Deckkraft, abgeleitet aus dem Papierwert.
- **D-06** Alle Ableitungen ueber `color-mix(in srgb, …)`.
- **D-07** Die schwaechste Tinte erfuellt AA fuer die kleinste Schriftgroesse.
- **D-08** Manrope, variabel, 200–800, aus dem Repository kopiert und ueber
  `/t/fonts/…` ausgeliefert. Nichts wird heruntergeladen.
- **D-09** JetBrains Mono wird NICHT uebernommen. Beschriftungen sind gesperrte
  Versalien in Manrope.
- **D-10** `@font-face` mit `font-weight` als SPANNE und getrennter
  `unicode-range` fuer latin und latin-ext.
- **D-11** Schriftgrade fluid ueber `clamp()`; die Schlagzeile gross, schmal
  geschnitten, `line-height` um 1.03.
- **D-12** `--hc-font-sans` liest den Schriftregler mit Manrope als Rueckfall.
- **D-13** `layout.html`: klebende Kopfleiste auf halbdurchsichtigem Papier mit
  Haarlinie, Marke links, Menue rechts, Sprachen und Warenkorb; Untermenues
  ohne eine Zeile JavaScript ueber `:hover` und `:focus-within`; auf schmalen
  Fenstern bricht die Leiste um; Sprunglink bleibt erstes Tabulatorziel;
  mehrspaltige Fusszeile mit Marke, Fussnavigation, `footer-kontakt`,
  `schwesterseite` und Schlagwoertern.
- **D-14** `home.html`: grosser Aufmacher mit Augenbrauen-Zeile, Schlagzeile,
  einer Zeile Anspruch und abgesicherten Knoepfen; daneben das Titelbild aus
  `.Meta.OGImage`, sonst eine schmueckende Huegelzeichnung.
- **D-15** `page.html`: schlankerer Kopf, Zweispalten-Raster — der Textkoerper
  ist schmal, die Bilder sind es nicht.
- **D-16** Alle zwoelf Ansichten bleiben erhalten und werden uebersetzt.
- **D-17** Ein `favicon.svg` in der Vorlage samt Rueckfallzeile im `<head>`.
- **D-18** Auf hellem Papier ist eine Karte ein deckender Kasten mit Haarlinie
  und weichem Schatten, keine Glasflaeche. Der Knopf im Aufruf-Baustein braucht
  dieselbe ausdrueckliche Schriftfarbe wie in holzcloud.
- **D-19** Kein JavaScript, keine externe Ressource.
- **D-20** Die Vorlage setzt selbst keinen der vier gebrueckten Reglernamen;
  sie liest sie nur.
- **D-21** Der Datenvertrag wird nicht erweitert, kein Go-Code geaendert.
- **D-22** Die Kommentare sind auf DEUTSCH und erklaeren das WARUM.

## Zwei Feststellungen aus der Vorbereitung

**Die Alphastufe aus holzcloud traegt auf hellem Papier NICHT.** In
holzcloud/style.css steht `--hc-ink-3` bei 52 % mit dem Vermerk, dass 42 %
durchfiel. Diese Zahl gilt fuer helle Tinte auf dunklem Grund. Auf
`#FAF6EF` gemessen: Tinte `#231A11` bei 52 % ergibt 3.44:1 und faellt durch;
60 % ergibt 4.40:1 und faellt knapp durch; 62 % ergibt 4.67:1; **64 % ergibt
5.02:1** und ist der Wert, den dieser Plan festschreibt. `--hc-ink-2` bei 80 %
ergibt 8.60:1. Die Marke `#6E4D32` auf dem Papier ergibt 7.04:1 in beide
Richtungen, traegt also als Verweisfarbe und als Knopffuellung mit heller
Schrift.

**Beide Zweige des Aufmachers werden automatisch geprueft.**
`internal/template/sample.go:237` setzt `Meta.OGImage`, `MinimalData()` traegt
gar keine `Meta` — `template check` rendert jede Ansicht gegen beide Fixtures,
also einmal mit Bild und einmal mit der Huegelzeichnung. `CheckNoExternalRefs`
liest die Dateien auf der Platte und nicht das gerenderte Ergebnis
(internal/tmplmgr/external.go:89), deshalb ist `{{.Meta.OGImage}}` in einem
`src`-Attribut kein Befund.
</objective>

<execution_context>
@/Users/holz/.claude/gsd-core/workflows/execute-plan.md
@/Users/holz/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md

Die Referenz — Aufbau, Umfang und Tonfall dieser Arbeit:
@cmd/holzcloud/templates/public/holzcloud/style.css
@cmd/holzcloud/templates/public/holzcloud/layout.html
@cmd/holzcloud/templates/public/holzcloud/home.html
@cmd/holzcloud/templates/public/holzcloud/page.html
@cmd/holzcloud/templates/public/holzcloud/favicon.svg

Der heutige Stand, der ersetzt wird — die Fusszeile und die zwoelf Ansichten
sagen, welche Felder nicht verlorengehen duerfen:
@cmd/holzcloud/templates/public/weide/layout.html

Die Vertraege:
@internal/tmplspec/TEMPLATE-SPEC.md
@internal/template/loader.go
@internal/design/tokens.go
@cmd/holzcloud/assets/bausteine.css
@internal/block/render.go
</context>

<tasks>

<task type="tracer" tdd="false">
  <name>Aufgabe 1: Der Durchstich — Schrift, Gestaltungssystem, Huelle, Textseite</name>

  <files>
cmd/holzcloud/templates/public/weide/fonts/manrope-latin.woff2
cmd/holzcloud/templates/public/weide/fonts/manrope-latin-ext.woff2
cmd/holzcloud/templates/public/weide/favicon.svg
cmd/holzcloud/templates/public/weide/style.css
cmd/holzcloud/templates/public/weide/layout.html
cmd/holzcloud/templates/public/weide/page.html
  </files>

  <read_first>
cmd/holzcloud/templates/public/holzcloud/style.css — Zeilen 1..120 (Kopf und die
vier @font-face) und Zeilen 823..1150 (die CMS-Schicht: Bruecke zu den Reglern,
Bruecke zu den Bausteinen, Sprunglink, Kopfleiste, .page-content). Diese beiden
Bereiche sind die Vorlage fuer Aufbau und Tonfall.
cmd/holzcloud/templates/public/holzcloud/layout.html — der ganze <head>, die
Kopfleiste, das Verhaeltnis von bausteine.css zu /t/style.css zu .Site.Design.
cmd/holzcloud/templates/public/weide/layout.html — die Fusszeile: das Schaf als
eingebauter Pfad, footer-kontakt, schwesterseite, .Site.Terms, .lang-nav.
internal/design/tokens.go Zeilen 95..115 — welche sechs Namen ankommen.
  </read_first>

  <reversibility rating="costly">
Die Vorlage `weide` kleidet heute eine laufende Kundenwebsite. Ein Deploy
aendert deren Aussehen sofort. Rueckbau ist ein `git revert` dieser drei
Commits, aber nicht unsichtbar — deshalb costly, nicht reversible.
  </reversibility>

  <action>
Der Durchstich: eine Textseite laeuft nach dieser Aufgabe end-to-end in der
neuen Gestaltung — Schriftdatei, Gestaltungssystem, Huelle, Ansicht. Die zehn
uebrigen Ansichten bleiben vorerst stehen und rendern weiter (sie sind gueltige
Go-Templates), damit der Pruefbefehl in jeder Aufgabe laufen kann.

**Die Schriften (D-08, D-10).** Kopiere `manrope-latin.woff2` und
`manrope-latin-ext.woff2` aus `cmd/holzcloud/templates/public/holzcloud/fonts/`
nach `cmd/holzcloud/templates/public/weide/fonts/`. Kopieren, nicht
herunterladen: die Dateien sind in VENDOR.md mit Groesse, Quelle, SHA-256 und
Lizenz verzeichnet und liegen bereits im Repository. JetBrains Mono wird nicht
kopiert (D-09).

**`style.css`, Teil 1 — der Dateikopf.** Ein deutscher Kommentar, der erklaert,
warum die Datei so gebaut ist: die drei Stufen der Kaskade (Schichten, dann die
schichtlose CMS-Schicht, dann `.Site.Design` im `<style>` danach), und warum
unten keine `@layer` stehen darf. Kommentarzeilen im Rumpf bleiben eingerueckt,
damit die Zaehlung der Schichten am Zeilenanfang eindeutig bleibt.

**`style.css`, Teil 2 — die Schriften (D-10).** Zwei `@font-face`-Blocke fuer
Manrope, je einer fuer latin und latin-ext, mit `font-weight: 200 800` als
Spanne (eine Datei je Teilmenge traegt jedes Gewicht), `font-display: swap`,
`src: url("/t/fonts/manrope-latin.woff2") format("woff2")` und der
`unicode-range` aus holzcloud/style.css Zeilen 45..60, unveraendert
uebernommen. Ein deutscher Kommentar sagt, warum eine Spanne und nicht ein
Block je Schnitt, und warum die zwei Teilmengen getrennt sind.

**`style.css`, Teil 3 — vier Gestaltungsschichten, in dieser Reihenfolge:**
`@layer weide.tokens`, `@layer weide.base`, `@layer weide.components`,
`@layer weide.motif`. Genau vier, alle vor der CMS-Schicht.

*weide.tokens* — die einzige Stelle, an der ein Farbwert entschieden wird
(D-04, D-05, D-06). Setze:
`--hc-ground` auf `#FAF6EF`, `--hc-ink` auf `#231A11`, `--hc-erde` auf
`#6E4D32`, `--hc-on-erde` auf `var(--hc-ground)`.
Der Name `--hc-erde` ersetzt das `--hc-brass` von holzcloud — Erdbraun statt
Messing, und der Name macht jede Regel darunter lesbar.
Alle weiteren Farben sind Ableitungen ueber `color-mix(in srgb, …)`:
`--hc-ink-2` = 80 % Tinte auf transparent (8.60:1),
`--hc-ink-3` = 64 % Tinte auf transparent (5.02:1) — die Zahl traegt einen
deutschen Kommentar mit der gemessenen Zahl und dem Hinweis, dass die 52 % aus
holzcloud fuer dunklen Grund gelten und hier 3.44:1 ergaeben, also durchfielen
(D-07);
`--hc-hairline` = 14 % Tinte, `--hc-pane-edge` = 12 % Tinte;
`--hc-pane` = Papier mit 18 % Weiss (eine Karte liegt heller als das Blatt, sie
schwebt nicht ueber ihm), `--hc-pane-sunken` = Papier mit 6 % Tinte;
`--hc-pane-bar` = Papier mit 82 % Deckung, `--hc-pane-bar-solid` = 98 % — die
Kopfleiste ist halbdurchsichtiges Papier, kein Glas;
`--hc-erde-soft` = 35 % Marke, `--hc-erde-wash` = 8 % Marke;
`--hc-focus` = `var(--hc-erde)`.
Der Grund (D-05): `--hc-ground-2` = Papier mit 4 % Weiss;
`--hc-wash-erde` = 9 % der Marke auf transparent, `--hc-wash-stroh` = 8 % eines
Strohgelbs, `--hc-wash-salbei` = 7 % eines Salbeigruens — Stroh und Salbei sind
hier als Literal richtig, weil sie eine zweite und dritte Farbe sind und nicht
die Marke; das Braun leitet aus der Marke ab, damit eine eingestellte Marke den
Grund mitzieht. `--hc-ground-image` fasst die drei radialen Schleier ueber
einem `linear-gradient` von `--hc-ground-2` nach `--hc-ground` zusammen, damit
eine Flaeche nur `background: var(--hc-ground-image)` zu sagen braucht. Beide
Enden des Verlaufs leiten aus dem Papierwert ab — sonst wirkt ein anderes
Papier nur an einem Ende.
Weiter die Namensfamilien, die die CMS-Schicht braucht: `--hc-r-sm`, `--hc-r`
(Vorgabe 14px), `--hc-r-lg`, `--hc-r-pill`; `--hc-space-1` bis `--hc-space-9`
in 4px-Schritten; die Schriftgrade `--hc-text-label`, `--hc-text-xs`,
`--hc-text-sm`, `--hc-text-base`, `--hc-text-lg`, `--hc-text-xl`,
`--hc-text-2xl`, `--hc-display-sm`, `--hc-display` — die vier groessten fluid
ueber `clamp()`, `--hc-display` mit dem Richtwert
`clamp(2.4rem, 5.5vw, 4.2rem)` (D-11); die Gewichte `--hc-weight-thin` 250,
`--hc-weight-regular` 400, `--hc-weight-medium` 500, `--hc-weight-semibold` 650;
`--hc-track-snug` leicht negativ, `--hc-track-wide` und `--hc-track-widest`
gesperrt fuer Versalien-Beschriftungen (D-09 — es gibt keine Mono-Familie, also
auch kein `--hc-font-mono`); `--hc-leading-tight` um 1.03 fuer die Schlagzeile;
`--hc-shadow-pane` und `--hc-shadow-lift` als weiche, aus der Tinte gemischte
Schatten; `--hc-dur-fast`, `--hc-ease`; `--hc-wide` als Breite des Satzspiegels
und `--hc-gutter`; `--hc-measure` als Vorgabe `74ch`; `--hc-font-sans` als
`Manrope, ui-sans-serif, system-ui, -apple-system, sans-serif`.

*weide.base* — Grundreset, `body` mit `background: var(--hc-ground-image)`,
`background-attachment: fixed` und `color: var(--hc-ink)`, die Schrift aus
`--hc-font-sans`, `-webkit-font-smoothing: antialiased`, ein sichtbarer
Fokusring aus `--hc-focus`, `:target`-Sprungabstand, `prefers-reduced-motion`.
Die Ueberschriftengroessen und die Textgrundlinie.

*weide.components* — die Bauteile, die die zwoelf Ansichten benutzen, mit
denselben Klassennamen wie in holzcloud, damit dieselben Gesten dieselben Namen
tragen: `.hc-wide`, `.hc-bar` und `.hc-bar__inner`, `.hc-pane` (mit
`--pad`/`--tight`), `.hc-btn` (mit `--ghost`/`--sm`/`--danger`), `.hc-input`,
`.hc-select`, `.hc-textarea`, `.hc-field`, `.hc-label`, `.hc-hint`,
`.hc-error`, `.hc-alert` (mit `--ok`/`--warn`/`--danger`/`--info`),
`.hc-status`, `.hc-chip`, `.hc-eyebrow`, `.hc-display`, `.hc-title`,
`.hc-claim`, `.hc-lede`, `.hc-dim`, `.hc-defs`, `.hc-table`, `.hc-stack`,
`.hc-row`, `.hc-sr-only`, `.hc-lbl`. `.hc-pane` ist auf hellem Papier ein
deckender Kasten mit Haarlinie und weichem Schatten — kein `backdrop-filter`,
keine Glasflaeche (D-18). `.hc-eyebrow` und `.hc-lbl` sind gesperrte Versalien
in Manrope statt einer Mono-Beschriftung (D-09), weil eine Schreibmaschine auf
einem Bauernhof technisch klingt. `.hc-bar` klebt oben, liegt auf
`--hc-pane-bar` und traegt eine Haarlinie unten (D-13).

*weide.motif* — das Zeichenvokabular fuer die schmueckende Huegelzeichnung:
Klassen wie `.hc-huegel` fuer das svg, `.hc-h-fern`, `.hc-h-nah`,
`.hc-h-linie`, `.hc-sonne`, alle Farben aus `--hc-erde`, `--hc-ink` und
`--hc-ground` gemischt. Die Zeichnung selbst kommt in Aufgabe 2; die Klassen
entstehen hier, weil sie zum Gestaltungssystem gehoeren und nicht zur Ansicht.

**`style.css`, Teil 4 — die CMS-Schicht.** Eingeleitet von einer
Markierungszeile, die am Zeilenanfang mit `/* ══ CMS` beginnt, gefolgt von
einem deutschen Kommentar: hier steht kein Farbwert, und hier steht bewusst
keine Schicht, denn eine fuenfte wuerde diesen Bereich wieder unter die eigenen
vier und unter `.Site.Design` stellen.

*Die Bruecke zu den Reglern (D-02, D-20).* Ein `:root`-Block mit genau vier
Zeilen, jede liest den Regler mit dem Entwurfswert als Rueckfall:
`--hc-erde` aus `var(--hc-brand, #6E4D32)`,
`--hc-ground` aus `var(--hc-paper, #FAF6EF)`,
`--hc-font-sans` aus `var(--hc-font, Manrope, ui-sans-serif, system-ui, -apple-system, sans-serif)` (D-12),
`--hc-r` aus `var(--hc-radius, 14px)`.
Die zwei uebrigen Reglernamen wirken allein durch ihren Namen, weil das System
sie schon so nennt — sie brauchen keine Zeile. Ein deutscher Kommentar haelt
fest, dass die Richtung immer vom Regler zum System geht: die Rueckrichtung
waere ein Zyklus im benutzerdefinierten Wert und beide Werte fielen aus. Weil
alle Ableitungen in `weide.tokens` ueber `var()` auf diese vier Namen zeigen
und `var()` erst am Ende aufgeloest wird, zieht eine eingestellte Marke die
halbe Palette mit, ohne dass hier eine Ableitung wiederholt werden muss.

*Die Bruecke zu den Bausteinen (D-03).* Ein zweiter `:root`-Block mit zehn
Zeilen auf die generischen Namen, die `bausteine.css` liest: `--ink`,
`--paper`, `--brand`, `--line`, `--space-3`, `--space-4`, `--step--1`,
`--step-1`, `--step-2`, `--radius`. Ein deutscher Kommentar sagt, warum
`--paper` der Seitengrund ist und nicht die Kartenflaeche: `.hc-knopf` setzt
ihn als Schriftfarbe auf die Markenflaeche. Diese zehn Zeilen kleiden alle neun
Bausteinarten auf einmal; wer stattdessen `.hc-block`-Regeln nachbaut, hat zwei
Gestaltungen zu pflegen und behaelt die zweite nicht.

*Sprunglink und Huelle.* `.skip-link` heisst woertlich so — es ist die
Verabredung aller mitgelieferten Vorlagen und die Testreihe sucht den String.
Geklemmt statt nach -9999px geschoben. Dazu `.sr-only`, `html`
`scroll-padding-top`, `main`, `.page-wrap`.

*Die Kopfleiste (D-13).* `.site-mark`, `.site-logo`, `.site-menu` mit
Untermenues ueber `:hover` und `:focus-within` (Sichtbarkeit und Deckkraft, kein
`display`, damit der Uebergang etwas zu animieren hat), `.site-cart`,
`.site-search`. Die Sprachwahl behaelt den Namen `.lang-nav` aus der heutigen
Fassung, weil `bausteine.css` sie unter diesem Namen schon grundgestaltet;
hier kommt nur dazu, was die neue Sprache braucht. Eine Medienabfrage laesst
die Leiste umbrechen statt einen Schalter zu brauchen, und schaltet die
schwebenden Untermenues auf eingerueckt und immer sichtbar.

*Die Fusszeile (D-13).* Mehrspaltiges Raster fuer Marke, Fussnavigation,
`footer-kontakt`, `schwesterseite` und Schlagwoerter, dazu `.site-footer__mark`
fuer das Schaf, `.site-footer__heading`, `.site-footer__legal`.

*Der Textkoerper (D-15).* `.page-content` als Zweispalten-Raster:
`grid-template-columns: [text-start] min(100%, var(--hc-measure, 74ch)) [text-end] minmax(0, 1fr) [breit-ende]`;
alles liegt in der Textspalte, und `.hc-galerie`, `.hc-video` und
`.hc-bild--voll` laufen bis `breit-ende`. Ein deutscher Kommentar sagt, warum
zwei Rasterspalten und nicht `max-width` am Elternteil: die zweite Spalte ist
unter dem Mass null breit, also stapelt sich das auf dem Telefon von selbst,
ohne zweite Regel in einer Medienabfrage. Dazu die Innengestaltung des
Fliesstextes (Ueberschriften, Listen, Verweise in der Marke, Tabellen, `pre`,
`mark`) und die Moeblierung von `page.html`: `.preview-banner`,
`.page-header`, `.page-date`, `.page-fields`, `.term-list`, `.term`,
`.post-nav`.

**`favicon.svg` (D-17).** Ein Zeichen aus dem Vokabular der Vorlage — die
ruhige Huegellinie oder das Schaf aus der heutigen Fusszeile —, mit festen
Farbwerten, weil ein Favicon nichts erbt: Erdbraun auf dem Papierton. Keine
externe Referenz, kein `<script>`.

**`layout.html` (D-13, D-17, D-19).** Nach dem Muster von
holzcloud/layout.html:
Im `<head>` alle Meta-Angaben der heutigen Fassung unveraendert (Titel,
description, robots, canonical, og:*, twitter:card, der `application/ld+json`-
Datenblock, das Atom-Feed, die hreflang-Zeilen), dazu neu die Rueckfallzeile
fuer das Favicon, wenn die Website keines gesetzt hat — ohne sie fragt der
Browser bei jedem Aufruf `/favicon.ico` an und bekommt einen 404. Die
Stylesheets in der Reihenfolge `bausteine.css`, `/t/style.css`, dann der
`<style>` mit `.Site.Design`. Kein preconnect, kein Stylesheet zu einem
Schrifthaus.
Der `<body>` beginnt mit dem Sprunglink — anders als bei holzcloud gibt es
keine Texturflaeche davor, weil der Grund hier ein Verlauf auf dem `body`
selbst ist. Der heutige Checkbox-Schalter faellt weg: die Leiste bricht um.
Die Leiste traegt Marke (Logo, sonst Name), Hauptnavigation ueber `menuFor`,
die Sprachwahl, und neu — nach dem Muster von holzcloud — den Warenkorb hinter
`.Shop.Enabled` und das Suchformular hinter `.Site.HasSearch`. Die heutige
Fassung hat beides nicht, obwohl sie `shop.html` und `cart.html` mitbringt;
ohne den Verweis kaeme niemand vom Kopf der Seite zum Korb. Beides muss
`MinimalData()` ueberstehen, wo `Shop` und `Search` leer sind.
Die Fusszeile behaelt alles, was die heutige Fassung zeigt: das Schaf als
eingebauten Pfad in `currentColor`, den Namen, die Beschreibung, die beiden
Textbausteine, die Fussnavigation, die Schlagwoerter aus `.Site.Terms` und die
Zeile mit dem Copyright.

**`page.html` (D-15).** Nach dem Muster von holzcloud/page.html: Vorschau-
Banner, schlanker Kopf (Titel nur wenn `not .Page.HasOwnHeading`, Datum nur am
Beitrag), `.page-content` mit `.Page.ContentHTML`, die Feldliste mit allen acht
Faellen unveraendert aus der heutigen Fassung uebernommen, Schlagwoerter, und
die Nachbarnavigation mit `{{with}}` statt `{{if}}` — `Prev` und `Next` sind am
Rand des Archivs nil.

Jede Datei traegt deutsche Kommentare, die das WARUM erklaeren und nicht das
WAS (D-22). Kein `.js`, kein `<script>` ausser dem Datenblock, kein `on*`, kein
`javascript:`, keine Adresse zu einem fremden Ursprung (D-19).
  </action>

  <verify>
    <automated>
cd /Users/holz/Projects/holzcloud-cms
cmp cmd/holzcloud/templates/public/holzcloud/fonts/manrope-latin.woff2 cmd/holzcloud/templates/public/weide/fonts/manrope-latin.woff2
cmp cmd/holzcloud/templates/public/holzcloud/fonts/manrope-latin-ext.woff2 cmd/holzcloud/templates/public/weide/fonts/manrope-latin-ext.woff2
test -f cmd/holzcloud/templates/public/weide/favicon.svg
awk '/^@layer/{n++; last=NR} /^\/\* ══ CMS/{cms=NR} END{ if (n==4 && cms>0 && last<cms) print "Kaskade ok: 4 Schichten, alle vor der CMS-Schicht"; else { print "FEHLER: Schichten=" n " letzte=" last " CMS=" cms; exit 1 } }' cmd/holzcloud/templates/public/weide/style.css
test "$(grep -c -E -- '--hc-(brand|paper|font|radius)[[:space:]]*:' cmd/holzcloud/templates/public/weide/style.css)" = 0 && echo "Regler werden nur gelesen, nicht gesetzt"
grep -q -- '--hc-ink-3:.*var(--hc-ink) 64%' cmd/holzcloud/templates/public/weide/style.css && echo "Kontraststufe 64% (5.02:1) gesetzt"
go build ./...
go run ./cmd/holzcloud template check cmd/holzcloud/templates/public/weide
go test ./internal/template/... ./internal/tmplmgr/...
    </automated>
  </verify>

  <done>
`template check` meldet „no problems found". Die beiden woff2 sind byteweise
identisch mit den geprueften Originalen. `style.css` traegt genau vier
Schichten, alle vor der Markierung der CMS-Schicht, und setzt keinen der vier
gebrueckten Reglernamen. Die schwaechste Tinte steht auf 64 %. `go build`
und die beiden Testpakete sind gruen. Eine Textseite laeuft end-to-end in der
neuen Gestaltung; die zehn uebrigen Ansichten stehen noch in der alten und
rendern weiter.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Aufgabe 2: Der grosse Aufmacher und die neun Bausteinarten</name>

  <files>
cmd/holzcloud/templates/public/weide/home.html
cmd/holzcloud/templates/public/weide/style.css
  </files>

  <read_first>
cmd/holzcloud/templates/public/holzcloud/home.html — der Aufbau des Aufmachers
und der Kommentar dazu, warum `.Page.Excerpt` dort NICHT steht.
cmd/holzcloud/templates/public/holzcloud/style.css — Zeilen 1130..1310 (die
Bausteine in der CMS-Schicht) und Zeilen 1450..1475 (der Aufmacher).
internal/block/render.go — die Auszeichnung der neun Bausteinarten. Sie ist die
einzige Wahrheit ueber die Klassennamen, nicht dieser Plan.
cmd/holzcloud/assets/bausteine.css — was die Bruecke schon leistet.
  </read_first>

  <action>
**`home.html` (D-14).** Ein grosser Aufmacher, zweispaltig, nach dem Aufbau von
holzcloud/home.html:
Links die Augenbrauen-Zeile aus `.Site.Name` als gesperrte Versalien in der
Marke, darunter die Schlagzeile aus `.Page.Title` — nur wenn
`not .Page.HasOwnHeading`, sonst begruesst die Startseite zweimal —, darunter
eine Zeile Anspruch aus `.Site.Description`, darunter die Knoepfe. Die Knoepfe
zeigen nur auf Orte, die es wirklich gibt: der Laden hinter `.Shop.Enabled`
auf `.Shop.URL`, das Archiv hinter `{{with .Page.ArchiveURL}}`, die Suche
hinter `.Site.HasSearch`. Jedes Stueck einzeln abgesichert — eine Seite, die
nichts als einen Titel hat, muss trotzdem rendern.
`.Page.Excerpt` steht NICHT im Aufmacher. Der Anriss wird beim Speichern aus
demselben Markdown abgeleitet, das zwei Abschnitte weiter unten als Inhalt
noch einmal kommt; in holzcloud stand die Begruessung dadurch zweimal
untereinander, und das war der einzige Fehler, den die Sichtpruefung dort fand.
Ein deutscher Kommentar haelt das fest, damit es nicht zurueckkommt.

Rechts steht — und das ist der Unterschied zu holzcloud — das Titelbild der
Seite. `{{with .Meta.OGImage}}` zeigt es gross, gerundet, mit weichem Schatten;
der `{{else}}`-Zweig zeigt die schmueckende Huegelzeichnung. Das Bild traegt
`loading="eager"`, `decoding="async"` und ein leeres `alt`, weil es schmueckend
neben der Schlagzeile steht und nicht der Inhalt ist. Ein deutscher Kommentar
sagt, woher der Wert kommt (`internal/public/pagedata.go`, `withOGImage`, aus
dem Titelbild der Seite) und dass er die absolute Adresse auf den eigenen
Ursprung ist — dieselbe, die zwei Zeilen weiter oben schon als `og:image` im
Kopf steht, also holt der Browser sie einmal.
Die Huegelzeichnung ist ein `<svg>` mit `aria-hidden="true"`,
`role="presentation"` und `focusable="false"`, gebaut allein aus den
Motiv-Klassen aus `weide.motif` — kein `fill` und kein `stroke` als
Literalwert im Markup, sonst folgt sie einer eingestellten Marke nicht. Eine
ruhige Linienzeichnung sanfter Huegel, weil die Vorlage `weide` heisst.
Darunter der Rumpf wie in holzcloud: `.page-content` mit `.Page.ContentHTML`,
sonst ein Willkommenssatz, dann die Schlagwoerter.

**`style.css` — der Aufmacher.** An die CMS-Schicht anfuegen: `.site-hero`,
`.site-hero-grid` (zwei Spalten, auf schmalen Fenstern untereinander — zwei
Spalten auf einem Telefon sind zwei zu enge Spalten), `.site-actions` mit
`:empty { display: none }`, damit auf einer nackten Website kein Abstand
uebrigbleibt, `.site-art` und `.site-art__bild` (volle Breite der Spalte,
`--hc-r-lg`, weicher Schatten aus `--hc-shadow-pane`, `aspect-ratio` und
`object-fit: cover`, damit ein Hochformat den Aufmacher nicht in die Laenge
zieht).

**`style.css` — die neun Bausteinarten (D-03, D-18).** Nur was aus der Bruecke
NICHT folgt, mit je einem deutschen Kommentar, warum die Zeile noetig ist:
- Bild, Galeriebild und eigenes Bild bekommen eine Haarlinie, damit ein helles
  Bild auf hellem Papier als Fenster liest und nicht ausfranst.
- Bildunterschriften werden gesperrte Versalien in `--hc-ink-3` statt der
  Mono-Beschriftung aus holzcloud (D-09) — die `opacity` aus `bausteine.css`
  wird dabei auf 1 zurueckgesetzt, weil die Farbe die Abstufung schon traegt.
- `.hc-karte` ist ein deckender Kasten: `--hc-pane` als Grund, `--hc-pane-edge`
  als Kante, `--hc-shadow-pane` als Schatten, KEIN `backdrop-filter`. Der
  Kommentar sagt, warum: der Glas-Kunstgriff aus holzcloud ergibt nur auf
  dunklem Grund Sinn, auf Papier wirkt eine halbdurchsichtige Karte schmutzig.
  `.hc-karte--link` hebt beim Ueberfahren an und wechselt auf
  `--hc-shadow-lift`, gebunden an `prefers-reduced-motion`.
- `.hc-zitat`: der Balken traegt die Marke, nicht die Schriftfarbe; die
  Beschriftung in gesperrten Versalien.
- `.hc-aufruf`: dieselbe deckende Flaeche wie eine Karte, eine Stufe groesser,
  mit `--hc-r-lg`.
- `.hc-knopf`: die Pillenform, und ausdruecklich `color: var(--hc-ground)`.
  Der Kommentar nennt den Grund, den holzcloud teuer gelernt hat: die Regel fuer
  Verweise im Inhalt faerbt weiter oben jeden Link in der Marke, steht spaeter
  in derselben Datei, und ohne diese Zeile stuende der Knopf als brauner Text
  auf brauner Flaeche.
- `.hc-trenner`: Abstand statt Auffaelligkeit.
- `.hc-video`: hier fragt der Kern `--hc-radius` und `--hc-space` direkt ab,
  also die Reglernamen der Verwaltung, und fiele ohne sie auf 0 zurueck. Die
  Regel zielt deshalb auf die Klasse und nicht auf `:root` — am `:root` waere es
  der Zyklus, den die Bruecke vermeidet.
- Absaetze innerhalb eines Bausteins (`.hc-text`, `.hc-eigen__text`,
  `.hc-bildtext__text`) brauchen ihren eigenen Rhythmus: die Grundschicht setzt
  Absaetze auf `margin: 0`, und die Regel fuer den Abstand zwischen Bausteinen
  trifft nur die Bausteine selbst — ein Textbaustein aus drei Absaetzen stuende
  sonst als eine Wand ohne Luft.
- `.hc-eigen`: die Felder als ruhige Faktenzeilen in `--hc-ink-2`. Eigene
  Bausteinarten gehoeren der Website, nicht der Vorlage — hier steht nur, was
  ohne Kenntnis des Schluessels richtig ist.
  </action>

  <verify>
    <automated>
cd /Users/holz/Projects/holzcloud-cms
grep -q 'Meta.OGImage' cmd/holzcloud/templates/public/weide/home.html && echo "Titelbild im Aufmacher verdrahtet"
grep -q 'loading="eager"' cmd/holzcloud/templates/public/weide/home.html && echo "Aufmacherbild laedt sofort"
awk '/^@layer/{n++; last=NR} /^\/\* ══ CMS/{cms=NR} END{ if (n==4 && cms>0 && last<cms) print "Kaskade ok"; else { print "FEHLER"; exit 1 } }' cmd/holzcloud/templates/public/weide/style.css
test "$(grep -c -E -- '--hc-(brand|paper|font|radius)[[:space:]]*:' cmd/holzcloud/templates/public/weide/style.css)" = 0 && echo "Regler werden nur gelesen"
go build ./...
go run ./cmd/holzcloud template check cmd/holzcloud/templates/public/weide
go test ./internal/template/... ./internal/tmplmgr/...
    </automated>
  </verify>

  <done>
`template check` meldet „no problems found" — und weil es jede Ansicht gegen
SampleData UND gegen MinimalData rendert, ist damit sowohl der Zweig mit
Titelbild als auch der mit der Huegelzeichnung bewiesen. Die neun Bausteinarten
sind ueber die Bruecke gekleidet, mit gezielten Regeln nur dort, wo aus der
Bruecke etwas Falsches folgt. Die Kaskade traegt weiterhin genau vier Schichten.
  </done>
</task>

<task type="auto" tdd="false">
  <name>Aufgabe 3: Die zehn uebrigen Ansichten, der Ausdruck und die Herkunft</name>

  <files>
cmd/holzcloud/templates/public/weide/list.html
cmd/holzcloud/templates/public/weide/search.html
cmd/holzcloud/templates/public/weide/gate.html
cmd/holzcloud/templates/public/weide/404.html
cmd/holzcloud/templates/public/weide/maintenance.html
cmd/holzcloud/templates/public/weide/shop.html
cmd/holzcloud/templates/public/weide/product.html
cmd/holzcloud/templates/public/weide/cart.html
cmd/holzcloud/templates/public/weide/checkout.html
cmd/holzcloud/templates/public/weide/order.html
cmd/holzcloud/templates/public/weide/style.css
cmd/holzcloud/assets/VENDOR.md
  </files>

  <read_first>
cmd/holzcloud/templates/public/holzcloud/{list,search,gate,404,maintenance,shop,product,cart,checkout,order}.html
— der Aufbau in der neuen Sprache.
cmd/holzcloud/templates/public/weide/{list,search,gate,404,maintenance,shop,product,cart,checkout,order}.html
— der heutige Stand. Vier Felder zeigt er, die holzcloud nicht zeigt; sie sind
unten namentlich genannt und duerfen nicht verlorengehen.
cmd/holzcloud/templates/public/holzcloud/style.css — Zeilen 1473..1846: Archiv,
Suche, Schranke, die zwei kurzen Seiten, der Laden und der Ausdruck.
cmd/holzcloud/assets/VENDOR.md — der Abschnitt `## Schriften`.
  </read_first>

  <action>
**Die zehn Ansichten (D-16).** Jede wird nach dem Muster der gleichnamigen
Ansicht in `holzcloud/` neu geschrieben, also in derselben Bauteil-Sprache
(`.hc-wide`, `.hc-pane`, `.hc-btn`, `.hc-field`, `.hc-input`, `.hc-alert`,
`.hc-status`, `.hc-eyebrow`, `.hc-lede`, `.hc-dim`). Keine Ansicht darf
verschwinden: `internal/template/loader.go:587` kennt die Liste, und
`TestShippedThemesRenderEveryView` rendert jede mitgelieferte Vorlage
vollstaendig.

Vier Stellen zeigt die heutige Fassung, die holzcloud nicht hat. Sie werden
uebernommen, nicht weggelassen:
- `404.html` gibt `.Meta.Message` aus, wenn gesetzt, sonst den eigenen Satz.
  Diese Ansicht ist auch die Antwort auf einen abgelaufenen Vorschaulink —
  ohne die Meldung erfaehrt der Empfaenger nie, dass er nur einen neuen Link
  braucht. Der Titel kommt aus `.Page.Title`.
- `gate.html` zeigt am Ende `.Gate.Path`, wenn gesetzt: wohin es nach dem
  Freischalten weitergeht.
- `maintenance.html` faellt auf einen eigenen Satz mit `.Site.Name` zurueck,
  wenn `.Meta.Message` leer ist.
- `search.html` nimmt den Titel aus `.Page.Title`.
Umgekehrt bringt holzcloud drei Stellen mit, die die heutige Fassung nicht hat
und die uebernommen werden: die Zahl der Treffer im Archiv, die Schlagwortliste
am Fuss des Archivs und der Bestellstatus in `order.html`.
Ziel, Feldnamen und verstecktes Feld der Formulare bleiben unveraendert —
`/freischalten` mit `seite` und `passwort`, `.Checkout.Action`, die Namen der
Warenkorbfelder. Wer hier etwas umbenennt, macht die geschuetzte Seite oder die
Kasse unerreichbar.
`{{with}}` statt `{{if}}` ueberall dort, wo ein Zeiger fehlen kann.

**`style.css` — der Rest der CMS-Schicht.** An denselben Bereich anfuegen,
nach dem Vorbild der genannten Zeilen aus holzcloud, aber in der hellen
Fassung:
- Archiv: `.archive-count`, `.archive-list`, `.archive-entry` mit Bildspalte,
  `:has()` fuer den Eintrag ohne Bild, `.archive-pager`, `.archive-terms`,
  `.archive-empty`. Haarlinien zwischen den Eintraegen und keine Karten — eine
  Liste von Kaesten liest sich schlechter als eine Liste.
- Suche: `.search-form`, `.search-count`, `.search-results`, `.search-result`.
- Schranke: `.gate-card` und ihre Teile, eine Flaeche in der Mitte statt einer
  Seite, weil es hier genau eine Sache zu tun gibt.
- Die zwei kurzen Seiten: `.page--kurz`.
- Der Laden: `.price` mit `font-variant-numeric: tabular-nums` (ohne
  Mono-Familie ist das die Zeile, die Betraege untereinander ausrichtet — das
  ist der Ersatz fuer die Mono aus holzcloud, D-09), `.price-switch`,
  `.shop-categories`, `.product-grid`, `.product-card`, `.product`,
  `.product__gallery`, `.product__buy` (klebend, auf schmalen Fenstern
  statisch), `.cart-lines`, `.cart-line`, `.cart-summary`, `.cart-totals`,
  `.cart-actions`, `.checkout__grid`, `.checkout__group`, `.field-row`,
  `.choice` (die ganze Zeile ist das Ziel, nicht das Kaestchen — drei Zeichen
  trifft auf einem Telefon niemand), `.checkout__summary`, `.order__*`, samt
  den zwei Medienabfragen, die die Zweispalter auf schmalen Fenstern stapeln.
- Der Ausdruck: eine `@media print`-Regel. Anders als bei holzcloud ist der
  Grund hier schon hell, also bleibt der Papierteil kurz: der Farbschleier
  wird abgeschaltet, Leiste, Menue, Sprachwahl, Suche, Korb, Sprunglink,
  Nachbarnavigation und Blaetterleiste verschwinden, Schatten fallen weg,
  Adressen von Verweisen werden hinter dem Linktext ausgeschrieben — auf Papier
  ist ein Link sonst nur unterstrichen und fuehrt nirgendwohin. Der Textbaustein
  `footer-kontakt` bleibt stehen, wie in der heutigen Fassung: wer eine Hofseite
  ausdruckt, will meistens genau diese Zeilen.

**`VENDOR.md`.** Im Abschnitt `## Schriften` den Satz ergaenzen, dass die
beiden Manrope-Dateien seit dieser Aufgabe an ZWEI Orten liegen — in der
Vorlage `holzcloud` und in der Vorlage `weide` —, byteweise identisch, mit
denselben SHA-256-Werten aus der Manrope-Tabelle, und warum: eine Vorlage muss
als Archiv fuer sich stehen und ihre eigene Schrift mitbringen, sonst ist sie
kein gueltiges Archiv mehr, sobald jemand sie herunterlaedt und hochlaedt. Im
Aktualisierungsrezept den zweiten Zielpfad erwaehnen, damit ein kuenftiger
Wechsel nicht eine Kopie stehen laesst. JetBrains Mono bleibt bei `holzcloud`
allein.

**Zum Schluss** die vollstaendige Testreihe laufen lassen, nicht nur die zwei
Pakete: `go test ./...`. Ausserdem `go run ./tools/i18n` — diese Arbeit fuegt
keine uebersetzbaren Zeichenketten hinzu, aber der Lauf ist billig und die
Reihe ist ein stehendes Tor des Projekts.
  </action>

  <verify>
    <automated>
cd /Users/holz/Projects/holzcloud-cms
test "$(ls cmd/holzcloud/templates/public/weide/*.html | wc -l | tr -d ' ')" = 13 && echo "12 Ansichten plus layout.html"
grep -q 'Meta.Message' cmd/holzcloud/templates/public/weide/404.html && echo "abgelaufener Vorschaulink erklaert sich"
grep -q 'Gate.Path' cmd/holzcloud/templates/public/weide/gate.html && echo "Ziel nach dem Freischalten steht da"
grep -q 'Site.Name' cmd/holzcloud/templates/public/weide/maintenance.html && echo "Wartungssatz nennt den Betrieb"
grep -q 'weide' cmd/holzcloud/assets/VENDOR.md && echo "der zweite Ort der Schriften ist verzeichnet"
awk '/^@layer/{n++; last=NR} /^\/\* ══ CMS/{cms=NR} END{ if (n==4 && cms>0 && last<cms) print "Kaskade ok"; else { print "FEHLER"; exit 1 } }' cmd/holzcloud/templates/public/weide/style.css
test "$(grep -c -E -- '--hc-(brand|paper|font|radius)[[:space:]]*:' cmd/holzcloud/templates/public/weide/style.css)" = 0 && echo "Regler werden nur gelesen"
go build ./...
go run ./cmd/holzcloud template check cmd/holzcloud/templates/public/weide
go test ./internal/template/... ./internal/tmplmgr/...
go test ./...
go run ./tools/i18n
    </automated>
  </verify>

  <done>
Dreizehn HTML-Dateien liegen in der Vorlage, jede in der neuen Sprache. Die vier
Felder, die nur die alte Fassung zeigte, sind erhalten. `template check` meldet
„no problems found", `go test ./...` ist gruen, `go run ./tools/i18n` meldet
0 offen und 0 verwaist. VENDOR.md verzeichnet den zweiten Ort der beiden
Schriftdateien.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Browser -> Vorlage | Jede Ressource, die eine Seite dieser Vorlage anfordert, muss vom eigenen Ursprung kommen. Ein `<link>`, ein `url()` oder ein `src` auf einen fremden Host verraet jeden Besucher an einen Dritten und bricht die `Content-Security-Policy` in internal/web/headers.go. |
| Vorlagen-Archiv -> Server | Diese Vorlage ist zugleich ein gueltiges hochladbares Archiv. Was `internal/tmplmgr` beim Upload prueft, muss sie selbst bestehen. |
| `.Site.Design` -> `<style>` | Die Werte der Website landen in einem `<style>`-Block im Kopf. Sie werden in internal/design geprueft, bevor sie dorthin kommen. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-rqq-01 | Information Disclosure | `@font-face` in weide/style.css | high | mitigate | Manrope wird kopiert, nicht verlinkt; `src` zeigt auf `/t/fonts/…`. Geprueft von `CheckNoExternalRefs` ueber `template check` und von `TestBuiltinTemplatesHaveNoExternalRefs` (internal/tmplmgr/external_test.go:216). |
| T-rqq-02 | Tampering | die dreizehn HTML-Dateien | high | mitigate | Kein `.js`, kein `<script>` ausser `application/ld+json`, kein `on*`, keine `javascript:`-Adresse. Geprueft von `CheckNoScripts` (internal/tmplmgr/script.go:60) ueber `template check`. |
| T-rqq-03 | Tampering | die zwei kopierten woff2 | medium | mitigate | `cmp` gegen die in VENDOR.md mit SHA-256 verzeichneten Originale, in jeder der drei Aufgaben. Eine stillschweigend veraenderte Schriftdatei faellt beim ersten Lauf auf. |
| T-rqq-04 | Elevation of Privilege | `{{.Site.Design}}` im `<style>` | medium | accept | Unveraendert gegenueber allen acht mitgelieferten Vorlagen: `internal/design` laesst nur Hex-Farben, eine Schrift aus einer festen Liste, eine Zahl fuer das Mass und eine fuer die Rundung durch. Dieser Plan aendert dort nichts. |
| T-rqq-05 | Information Disclosure | Entwuerfe auf der oeffentlichen Seite | medium | accept | Die Vorlage fragt nichts ab; die Auswahl trifft `internal/public`. Dieser Plan aendert dort nichts. |
| T-rqq-06 | Information Disclosure | `.Meta.OGImage` als `src` im Aufmacher | low | accept | Der Wert wird in `withOGImage` aus der eigenen kanonischen Adresse gebaut (internal/public/pagedata.go:272) und zeigt deshalb auf den eigenen Ursprung; dieselbe Adresse steht in jeder Vorlage bereits als `og:image` im Kopf. |
| T-rqq-SC | Tampering | Paketinstallation | low | accept | Diese Arbeit ruft keinen Paketmanager auf — kein npm, kein pip, kein cargo, kein neues Go-Modul. Es gibt nichts zu auditieren. |
</threat_model>

<verification>
Nach der letzten Aufgabe, vom Projektwurzelverzeichnis aus:

1. `go run ./cmd/holzcloud template check cmd/holzcloud/templates/public/weide`
   meldet „no problems found". Das ist das Tor, auf das es ankommt: es rendert
   alle zwoelf Ansichten zweimal — einmal gegen eine voll besetzte Seite und
   einmal gegen eine, bei der jedes optionale Feld leer ist. Der zweite Lauf
   ist der, der einen ungesicherten Zeiger findet.
2. `go test ./internal/template/... ./internal/tmplmgr/...` ist gruen.
3. `go build ./...` ist gruen.
4. `go test ./...` ist gruen.
5. `go run ./tools/i18n` meldet 0 offen, 0 verwaist.
6. Die Kaskade: genau vier `@layer` am Zeilenanfang, alle vor der Markierung
   der CMS-Schicht.
7. Die Vorlage setzt keinen der vier gebrueckten Reglernamen.
8. Die zwei woff2 sind byteweise identisch mit den geprueften Originalen.

**Nicht automatisiert, und darum wert, hinterher zu tun:** ein Blick im
Browser auf Startseite, Textseite, Archiv und ein schmales Fenster. Bei
`holzcloud` fand genau dieser Blick den einen Fehler, den keine der obigen
Pruefungen sah — die Startseite druckte denselben Text zweimal. Dafuer gibt es
`/gsd-verify-work` gegen einen laufenden Server.
</verification>

<success_criteria>
- Die Vorlage `weide` steht auf warmem Papier mit Erdbraun und Manrope; kein
  Systemschriftstapel, keine schmale Blogspalte.
- Alle zwoelf Ansichten sind erhalten und uebersetzt; keine ist verschwunden.
- Alle neun Bausteinarten sind ueber die Bruecke gekleidet.
- Die drei Stufen der Kaskade stehen in dieser Ordnung: Schichten, CMS-Schicht,
  `.Site.Design`.
- Die schwaechste Tinte erfuellt AA fuer die kleinste Schriftgroesse — mit der
  auf hellem Papier gemessenen Zahl, nicht mit der aus holzcloud.
- Kein JavaScript, keine externe Ressource, keine Aenderung an Go-Code, keine
  Erweiterung des Datenvertrags.
- Jede Datei traegt deutsche Kommentare, die das WARUM erklaeren.
</success_criteria>

<output>
Create `.planning/quick/260903-rqq-vorlage-weide-neu-gestalten-helle-fassun/260903-rqq-SUMMARY.md` when done
</output>
