---
phase: 10-authentik
verified: 2026-09-10T16:00:00Z
status: gaps_found
score: 1/6 must-haves verified
tree_verified: >-
  480f21b. Kriterien 2-6 aus dem ersten Lauf, Kriterium 1 aus dem Nachlauf;
  jeder hier tragende Befund ist entweder von einer Gegenprobe oder vom
  Code-Durchgang unabhaengig bestaetigt, oder er steht am Code selbst.
closed_since: [0536f96, 88bab2e, e724cdd, e391023, 5a5e753, 0c7b15b]
gaps:
  - truth: "SC1 — die Gruppen entscheiden Rolle und Website-Zugang, bei JEDER Anmeldung neu, damit eine Herabstufung hier wirkt"
    status: partial
    reason: >-
      Gemessen: die Abgleichung laeuft nur fuer Anfragen ohne angemeldete
      Sitzung (forwardauth.go Schritt 3). Ein Administrator, dem Authentik die
      Gruppe nimmt, bleibt in derselben Sitzung Administrator; ein Redakteur,
      dem eine Website entzogen wird, erreicht sie weiter — bis zu 24 Stunden
      Laufzeit oder 4 Stunden Leerlauf. DEPLOY.md sagt woertlich das Gegenteil.
      Dazu CR-01: die Identitaet wird dem Konto mit derselben E-Mail-Adresse
      zugeordnet, nicht dem Benutzernamen.
    missing:
      - "Abgleichung auch fuer eine laufende SSO-Sitzung, und eine Sitzung, deren eingehende Identitaet nicht mehr zur Sitzung passt, wird verworfen"
      - "Bindung des Kontos an den Benutzernamen beim Anbieter statt an die Adresse (CR-01)"
      - "Ein Test, der die Protokollzeile beim Entzug einer Website haelt (Mutation M4 blieb gruen)"
  - truth: "SC3 — mit eingeschalteter Kontoerstellung verweigert der Dienst den Start ohne Standard-Website; Schweigen heisst nicht 'jede Website'"
    status: partial
    reason: >-
      Die Startverweigerung haelt. Aber: der Aufruf von checkDefaultWebsite in
      main() ist von keinem Test gehalten — ihn ganz zu entfernen liess die volle
      Suite gruen; die Website-IDs aus HOLZCLOUD_SSO_WEBSITE_GROUPS werden nie
      gegen die Tabelle geprueft; deploy/Caddyfile.example und DEPLOY.md liest
      kein Test; und wie HOLZCLOUD_SSO_SECRET in Caddys Umgebung kommt, steht
      nirgends. Die Loeschung der Standard-Website im Betrieb ist seit e724cdd
      geschlossen.
    missing:
      - "Ein Test, der den Aufruf von checkDefaultWebsite im Startpfad haelt"
      - "Pruefung der Gruppen-Website-IDs gegen die Datenbank beim Start"
      - "Anleitung, wie das Geheimnis in Caddys Umgebung gelangt"
  - truth: "SC4 — die Abhaengigkeit des zweiten Faktors ist in DEPLOY.md genannt UND in der Verwaltung gezeigt; Abmelden meldet auch bei Authentik ab"
    status: partial
    reason: >-
      Ein Praedikat, fuenf Aufrufstellen, beide Verwaltungsbildschirme —
      mutationsfest. Aber cmd/holzcloud/cli.go:452 formuliert dieselbe Regel ein
      zweites Mal und ist seit dieser Phase falsch: 'holzcloud user 2fa disable'
      verspricht einem SSO-Administrator eine Einrichtungspflicht, die nie kommt.
      Die Abmelde-Antwort verzweigt auf HX-Request ohne Vary: HX-Request. Dass
      der Ausweisdienst sein Cookie wirklich verwirft, ist eine vergangene
      Browserhandlung und hier nicht nachmessbar.
    missing:
      - "cli.go:452 an auth.MustHaveSecondFactor anbinden"
      - "Vary: HX-Request auf der Abmelde-Antwort"
  - truth: "SC5 — mit ausgeschaltetem SSO aendert sich am Anmelden nichts"
    status: failed
    reason: >-
      Gemessen: eine Sitzung mit via_sso, die das Abschalten ueberdauert (SQLite,
      24 Stunden), wird mit SSOEnabled=false nicht zur Zwei-Faktor-Einrichtung
      geschickt (200 statt 303) und darf den zweiten Faktor dauerhaft entfernen
      (totp_confirmed_at 1 -> 0), waehrend eine Passwortsitzung abgewiesen wird.
      RequireSecondFactor und Handler.viaSSO lesen die Marke ohne den Schalter;
      nur der Abmelde-Leser prueft ihn. Die Marke wird ausserdem nie entfernt
      und von einer spaeteren Passwortanmeldung derselben Sitzung geerbt
      (Code-Durchgang WR-04, gelesen).
    missing:
      - "Ein Praedikat fuer via_sso, das den Schalter einschliesst, an allen Lesern"
      - "via_sso bei jeder Anmeldung entfernen und nur auf dem SSO-Weg setzen"
  - truth: "SC6 — 0 offen, 0 verwaist ueber alles, was v1.6 hinzufuegte"
    status: partial
    reason: >-
      Die Zahl haelt und das Tor lebt (1325 Zeichenketten, 0/0 auf en/es/fr/it;
      ein neuer {{t}}-Satz macht sofort 1 offen). Aber das Tor sieht keinen
      verketteten Satz — gemessen: ein neuer deutscher Satz per + angehaengt
      laesst die Zahl unbewegt. In dieser Form stehen v1.6-Saetze, die auf einer
      englischen Verwaltung erscheinen: neun Ablehnungsgruende in
      internal/field/field.go (Sonde mit lang=en: 422 und 'Preis muss eine Zahl
      sein.') und sechs errors.New-Saetze in internal/field/store.go, die ueber
      err.Error() in die Meldezeile gehen (auf dem Bildschirm eines englischen
      Admins gelesen). Dazu T-10-53: der Browserdurchgang wurde vor einer
      Code-Durchgangs-Fixrunde abgezeichnet, die es nie gab.
    missing:
      - "Die fuenfzehn v1.6-Saetze durch den Katalog"
---

# Phase 10 — Verifizierung

**Ergebnis: 1 von 6 Kriterien erfüllt.** Gebaut ist die Phase vollständig — zehn
Pläne, alle gefahren —, und ihr Sicherheitskern, die vier Schichten vor dem
ersten geglaubten Header, hält jede Messung aus, die ihm gemacht wurde. Was nicht
hält, liegt dahinter: was mit einer geglaubten Identität geschieht.

Mess­grundlage, Hygienefehler des ersten Laufs und die Isolationslücke des
Nachlaufs sind in `10-SECURITY.md` unter „Wie gemessen wurde" beschrieben und
gelten hier genauso.

## Kriterium 1 — SSO-Anmeldung, Gruppen, jede Anmeldung, Protokoll · teilweise

**Hält, gemessen:** die Abgleichung läuft bei jeder Anmeldung ohne bestehende
Sitzung, nicht nur beim Anlegen (Mutation M6: 12 Tests rot). Gruppen werden auf
`|` getrennt und als ganzes Element verglichen (M1 rot in `web` und `admin`).
`completeLogin` hat genau vier Aufrufer, `RenewToken` steht davor (M5 rot).
Herabstufung, Hochstufung und Entzug schreiben je genau eine Protokollzeile,
eine unveränderte Anmeldung keine (M3, M8 rot).

**Hält nicht:**

- **Eine laufende Sitzung sieht keine Herabstufung.** Gemessen mit demselben
  Cookie: Admin-Gruppe entzogen → `users.role` bleibt `admin`; Website B
  entzogen → B bleibt erreichbar. Erst eine neue Sitzung wendet es an.
- **CR-01** — die Identität wird über die Adresse gefunden, siehe
  `10-SECURITY.md`.
- Die Protokollzeile beim Entzug einer Website ist heute da, aber **kein Test
  hält sie** (M4 blieb grün).
- Klein: die SSO-Rechtezeilen tragen `user_id NULL` und sind über den
  Benutzerfilter des Protokolls nicht auffindbar; ein von Hand entzogenes Recht
  stellt die nächste SSO-Anmeldung kommentarlos wieder her.

**Geschlossen seit 480f21b:** drei Wege, auf denen die Abgleichung oder eine
Löschung einen begrenzten Redakteur zum Redakteur aller Websites machte
(`0536f96`/`88bab2e` rot, `e724cdd`/`e391023` grün).

## Kriterium 2 — der Abnahmetest · erfüllt

Alle sieben Teilaussagen einzeln gemessen, live gegen das gebaute Binär über IPv4
und IPv6: Bindung an Loopback (ein einziger Socket, LAN-Adresse abgelehnt),
Gegenpart vor Header (`IsTrustedPeer` liest nur `RemoteAddr`; ein gefälschtes
`X-Forwarded-For` hilft nicht), Strip aller Schreibweisen inklusive
Unterstrich, gemeinsames Geheimnis, Umgebung statt Datenbank (`strings` über die
lebende Datenbank: 0 Treffer), und das Passwortformular für einen fremden
Gegenpart ohne Header. Die schärfste Einzelmessung ist die Gegenprobe: dieselbe
Anfrage, geändert nur `HOLZCLOUD_TRUSTED_PROXIES` auf Loopback → Dashboard. Das
Formular kommt also wirklich vom Peer-Tor.

**Ausdrücklich nicht wörtlich gemessen:** die zweite Maschine. Hergestellt wurde
der gleichwertige Zustand, indem Loopback zum nicht vertrauenswürdigen Gegenpart
gemacht wurde.

**Ungehalten, obwohl heute richtig:** `subtle.ConstantTimeCompare` (durch `==`
ersetzt: alles grün) und die Position der Middleware außerhalb von `RequestID`
(unter beide geschoben: alles grün).

**Seither geschlossen, aber nicht Teil des Kriteriums:** ein Identitätsheader mit
zwei Werten wurde geglaubt (`5a5e753` rot, `0c7b15b` grün). Das Kriterium spricht
von Schreibweisen, nicht von Vielfachheit; der Befund stand in keiner seiner
Aussagen und untergrub doch, was es sichern soll.

## Kriterium 3 — kein Konto ohne Erlaubnis, Start nur mit Standard-Website · teilweise

**Hält:** die Startverweigerung ohne und mit nicht existierender Standard-Website;
das Caddy-Beispiel löscht jeden kopierten Header ausdrücklich in beiden
Schreibweisen (acht Zeilen); `DEPLOY.md` nennt 2.11.2 an zwei Stellen.

**Hält nicht:** siehe `gaps` oben. Die Löschung der Standard-Website im Betrieb —
der schwerste Befund dieses Kriteriums — ist seit `e724cdd` geschlossen.

## Kriterium 4 — zweiter Faktor, sichtbar gemacht, Abmelden · teilweise

**Hält, mutationsfest:** `MustHaveSecondFactor` steht genau einmal, fünf
Aufrufstellen, keine zweite Formulierung in `internal/`; beide
Verwaltungsbildschirme zeigen die Abhängigkeit.

**Hält nicht:** die sechste Formulierung derselben Regel in `cmd/holzcloud/cli.go`
— genau die Fehlerfamilie „an jeder bekannten Stelle richtig, an einer
übersehenen still falsch". Dazu `Vary` und die nicht nachmessbare
Browserhälfte.

## Kriterium 5 — mit SSO aus ändert sich nichts · nicht erfüllt

Die gemessene `via_sso`-Lücke ist genau das, was das Kriterium ausschließt: eine
Anlage, deren Betreiber SSO im Notfall abschaltet, behält für jede über SSO
entstandene Administratorsitzung die Befreiung vom zweiten Faktor — und die
Möglichkeit, ihn zu löschen.

Daneben, ohne Sicherheitswirkung: `RequireFreshPassword` fragt vor fünf
Verwaltungsaktionen nach einem Passwort, das ein bereitgestelltes Konto nicht
hat (WR-07). Und zwei der vier Klauseln — der zweite Faktor am
Anmeldebildschirm, die Wiederherstellungscodes — haben keinen Test auf
Handler-Ebene, weder mit SSO an noch aus.

## Kriterium 6 — Katalog und Browserdurchgang · teilweise

Siehe `gaps`. Der Feldbildschirm, dessen vier Titel dieselbe Klasse waren, ist
seit `46e0722` geschlossen.

Zum Browserdurchgang: ein Prüfer buchte ihn als unbelegt; die Gegenprobe hat das
widerlegt, das Protokoll steht in `10-10-SUMMARY.md`. Er bleibt eine vergangene
Handlung, und er wurde vor einer Fixrunde abgezeichnet, die die Roadmap
ausdrücklich vor Welle 9 verlangte und die nie stattfand — der Code-Durchgang
dieser Phase ist vom 2026-09-10.

## Was seit 480f21b geschlossen ist

| Befund | Rotbeweis | Flick | Mutation |
|---|---|---|---|
| Website löschen → Redakteur aller Websites | `0536f96` | `e724cdd` | M1, M3 rot |
| `SetRights` halb geschrieben | `88bab2e` | `e724cdd` | M2 rot |
| Degradierter Admin → Redakteur aller Websites | `88bab2e` | `e391023` | S1, S3 rot; S2 grün → eigener Test |
| Identitätsheader mit zwei Werten | `5a5e753` | `0c7b15b` | W1: 8 Unterfälle rot |
