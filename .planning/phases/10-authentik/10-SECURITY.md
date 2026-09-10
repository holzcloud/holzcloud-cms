---
phase: 10-authentik
audited: 2026-09-10
tree_audited: 480f21b
status: OPEN_THREATS
verdict_note: >-
  Blockierend. Die Register-Zeilen stehen überwiegend, aber die schwersten
  Befunde dieser Prüfung hatten gar keine Zeile: eine Identität wird dem Konto
  mit derselben E-Mail-Adresse zugeordnet (Code-Durchgang CR-01), ein
  Identitätsheader mit zwei Werten übernahm die Identität, und drei Wege
  machten aus einem begrenzten Redakteur einen Redakteur aller Websites.
  Die letzten beiden Gruppen sind seit e724cdd, e391023 und 0c7b15b geschlossen;
  CR-01 ist offen.
threats_total: 59
threats_closed: 43
threats_partial: 15
threats_open: 1
threats_not_applicable: 0
unregistered_surfaces: 39
asvs_level: 2
block_on: high
---

# Phase 10 — Sicherheitsprüfung

59 verschiedene Bedrohungs-IDs über zehn Pläne (`T-10-SC` kehrt je Plan
wieder und zählt einmal). Jede Zeile unten ruht auf gelesenem Code, einem
gefahrenen Test oder einer Mutationsprobe — nicht auf einer `SUMMARY.md`.

## Wie gemessen wurde, und was dabei schiefging

Das gehört an den Anfang, weil es bestimmt, wie weit man den Zahlen trauen darf.

**Der erste Lauf war verdorben.** Elf Prüfer arbeiteten im selben Arbeitsbaum
und schalteten dort Absicherungen aus, um zu sehen, ob Tests rot werden. Drei
von ihnen meldeten unabhängig, dass sie Mutationen *anderer* Prüfer im Code
vorfanden — einer bekam dadurch ein falsch-grünes Ergebnis. Als 66 der 86
Agenten am Wochenlimit starben, blieben zwei Mutationen im Baum stehen, eine
davon gefährlich: `isLocalPath` in `internal/config/config.go` gab `true` zurück,
womit die Prüfung des Abmeldeziels gegen `//fremd.example` ausgeschaltet war.
Beide wurden vor jeder weiteren Arbeit gefunden und zurückgesetzt; `grep` auf
Mutationsmarken im Baum ist leer.

**Der Nachlauf lief isoliert — auf dem falschen Stand.** Drei neue Prüfer
bekamen je einen eigenen Worktree. Alle drei standen auf `d4ca500`
(`origin/main`, ohne ein Commit der Phase 10). Die Hygieneregel prüfte, *ob*
ein Prüfer isoliert ist, nicht *auf welchem Stand*. Alle drei haben es selbst
bemerkt: zwei wechselten im eigenen Worktree auf `480f21b`, der Code-Durchgang
maß in einer `git archive 480f21b`-Kopie. Die Messungen gelten; die Regel war
trotzdem unvollständig und steht hier, damit die nächste sie nicht erbt.

**Gültige Grundlage:** Stapel 10-01 bis 10-08 aus dem ersten Lauf, soweit ihre
Befunde durch eine Gegenprobe oder den Nachlauf bestätigt sind; Stapel
10-09/10-10, Kriterium 1 und der Code-Durchgang aus dem Nachlauf. Wo ein
Befund des ersten Laufs nur von einem Prüfer stammt, der über fremde
Mutationen klagte, ist er hier nur übernommen, wenn der Code ihn selbst trägt.

## Die Befunde, um die es zuerst geht

### Eine SSO-Identität wird dem Konto mit derselben E-Mail-Adresse zugeordnet — offen

Code-Durchgang **CR-01**, gemessen. `ForwardAuthSignIn` sucht das Konto mit
`GetByEmail`; nichts bindet die Identität beim Anbieter an die lokale Zeile. Der
Kommentar „The identity is pinned to the username" und die Warnung in
`DEPLOY.md`, ein umbenannter Benutzer ergebe hier ein neues Konto, sind beide
falsch. Gemessen: `X-authentik-username: mallory`, `X-authentik-email:
boss@example.com`, leere Gruppen → Sitzung als der einzige Administrator, mit
`via_sso` und damit ohne zweiten Faktor. Die Rückfallregel für den letzten
Administrator lässt die Anmeldung dabei als `admin` weiterlaufen.

Wer das auslösen kann: jeder, der den Anbieter eine gewählte Adresse ausgeben
lassen kann. Ob Authentik in der Auslieferung Benutzern erlaubt, die eigene
Adresse zu ändern, ist **nicht gemessen**.

Die Roadmap hatte die Richtung schon vorgegeben — *„Pin to username instead if
unsure"* —, umgesetzt wurde sie nie. Keine Registerzeile spricht davon.

### Ein Identitätsheader mit zwei Werten — geschlossen (0c7b15b)

Keine Registerzeile sprach von Vielfachheit; T-10-08/09 sprechen von
Schreibweisen, also von verschiedenen Schlüsseln. `Header.Get` nimmt den ersten
Wert, und bei `X-Authentik-Groups` bestimmte, wer zuerst schreibt, die Gruppen.
Rotbeweis `5a5e753`, Flick `0c7b15b`, Mutation: 8 rote Unterfälle.

### Drei Wege zu „ein begrenzter Redakteur erreicht jede Website" — geschlossen

Drei Prüfer fanden den ersten unabhängig, zwei Gegenproben bestätigten ihn über
den echten Lösch-Handler mit lebender Sitzung (403 vorher, 200 nachher,
dasselbe Cookie):

| Weg | Rotbeweis | geschlossen durch |
|---|---|---|
| Website löschen → `ON DELETE CASCADE` räumt die einzige Zeile weg | `0536f96` | `e724cdd` |
| `SetRights` scheitert zwischen `DELETE` und `INSERT` | `88bab2e` | `e724cdd` |
| Anmeldung stuft einen Administrator ohne Website-Gruppe herab | `88bab2e` | `e391023` |

Alle drei münden in dieselbe Kodierung — *keine Zeile heisst jede Website* —,
und die ist das, was geschlossen wurde (Migration `00052`,
`users.websites_limited`). Einzeln geschlossene Wege wären die Fehlerfamilie
„an jeder bekannten Stelle richtig, an einer übersehenen still falsch" zum
siebten Mal gewesen. Der Weg ist älter als Phase 10 (Migration `00033`, auch
von Hand angelegte Redakteure waren betroffen); Phase 10 hat ihn automatisch
gemacht.

**Eine Nachwirkung des Flicks ist offen:** das Nutzerformular kann „begrenzt auf
keine Website" nicht darstellen — kein Häkchen heisst dort „alle". Wer das
Formular eines solchen Redakteurs unverändert speichert, hebt die Begrenzung
auf. Vor `e724cdd` war der Zustand nicht erreichbar; jetzt ist er es.

### Weitere Befunde ohne Registerzeile, offen

- **Eine Herabstufung wirkt nicht auf eine laufende Sitzung** (Kriterium 1,
  Code-Durchgang WR-08, gemessen). Die Abgleichung läuft nur für Anfragen ohne
  angemeldete Sitzung; ein Administrator, dem Authentik die Gruppe nimmt, bleibt
  es bis zu 24 Stunden. `DEPLOY.md` behauptet das Gegenteil.
- **`via_sso` überlebt das Abschalten von SSO** (Kriterium 5, WR-04, gemessen):
  mit `SSOEnabled=false` wird ein Administrator mit dieser Marke nicht zur
  Einrichtung geschickt und darf seinen zweiten Faktor dauerhaft entfernen.
- **`HOLZCLOUD_TRUSTED_PROXIES=0.0.0.0/0` bei eingeschaltetem SSO** wird
  ohne Wort angenommen (WR-05, gemessen); danach steht nur noch das Geheimnis,
  für das es keine Mindestlänge gibt (WR-06, gemessen: ein Zeichen lädt).

## Register

| ID | Bedrohung | Urteil auf 480f21b | Schwere | Beleg (gekürzt) |
|---|---|---|---|---|
| T-10-01 | SSOProvision ohne Standard-Website — Rechteausweitung auf jede Website | geschlossen | hoch | Beide Verweigerungen im Code gelesen. Umgebungshaelfte: internal/config/config.go:322-327 (`if cfg.SSOProvision && cfg.SSODefaultWebsite <= 0`). Datenbankhaelfte: cmd/holzcloud/main.go:654-669 (checkDefaultWebsite), aufgerufen an… |
| T-10-02 | Das geteilte Geheimnis im Startprotokoll | geschlossen | mittel | internal/config/config.go:439 `slog.Bool("sso_configured", c.SSOEnabled && c.SSOSecret != "")` — ein Boolean, kein Wert, auch nicht gekuerzt. Gefahren: `go test ./internal/config/ -run TestConfigLogValueCarriesTheSSOBlockButNotTh… |
| T-10-03 | Das geteilte Geheimnis in einer Sicherung | geschlossen | mittel | Der Wert wird an genau einer Stelle gelesen — internal/config/config.go:292 aus der Umgebung — und nirgends geschrieben. Gefahrene Abfrage: `grep -rn "sso" internal/db/migrations/*.sql` -> kein Treffer, es gibt keine Spalte, kein… |
| T-10-04 | Der Verwaltungsport aus dem Netz erreichbar — HOLZCLOUD_LISTEN | geschlossen | mittel | internal/config/config.go:174 `const defaultListen = "127.0.0.1"`, gelesen an config.go:286. Gefahren: `go test ./internal/config/ -run 'TestSSOAndListenDefaults\|TestListenAcceptsAnyAddressTheOperatorNames' -count=1` -> PASS. De… |
| T-10-05 | SSOSignOutPath als offene Weiterleitung | **teilweise** | niedrig | internal/config/config.go:328-332 ruft isLocalPath (config.go:344-350), Verbrauchsstelle internal/admin/login.go:159-175, ausgeliefert ueber h.redirect (internal/admin/page.go:1003-1010, also Location bzw. HX-Redirect). Gefahren:… |
| T-10-06 | Eine vertippte Einstellung still durch eine Vorgabe ersetzt (akzeptiert) | geschlossen | niedrig | Als "accept" gefuehrt mit der Begruendung, das bestehende Design decke es bereits. Das habe ich nachgelesen und es stimmt: Load sammelt in `errs` und gibt errors.Join zurueck (internal/config/config.go:307-336), envSize und envBo… |
| T-10-07 | Ein Client sendet X-authentik-email direkt | geschlossen | hoch | internal/web/forwardauth.go:135 — eine einzige kurzschliessende &&-Kette, in der resolver.IsTrustedPeer(r) links von secretMatches und links von jedem Header-Lesen steht (Lesestellen erst :139-147). IsTrustedPeer selbst internal/… |
| T-10-08 | Die eigene Kopfzeile eines Clients ueberlebt forward_auth auf Caddy 2.10.0-2.11.1 | **teilweise** | hoch | Der bedingungslose Strip ist echt und sitzt an internal/web/forwardauth.go:152-153, ausserhalb jedes if; stripIdentityHeaders (:181-193) laeuft ueber die Schluessel der Map statt ueber eine Namensliste und benutzt `delete` statt… |
| T-10-09 | Der Unterstrich-Alias X_authentik_email | geschlossen | hoch | internal/web/forwardauth.go:195-199 isIdentityHeader normalisiert mit `strings.ToLower(strings.ReplaceAll(name, "_", "-"))` vor dem Praefixvergleich, und stripIdentityHeaders benutzt bewusst `delete` statt Header.Del (Kommentar :… |
| T-10-10 | Zeitverhalten beim Vergleich des geteilten Geheimnisses | **teilweise** | niedrig | Die Minderung IST da: internal/web/forwardauth.go:211-213 `subtle.ConstantTimeCompare(...) == 1`, genau eine Aufrufstelle (gefahren: `grep -rn subtle --include='*.go' .` -> forwardauth.go:5 und :212 als einzige Treffer in diesem… |
| T-10-11 | Ein Handler liest das geteilte Geheimnis der Installation aus der Anfrage | geschlossen | mittel | internal/web/forwardauth.go:197-198 — isIdentityHeader trifft neben dem Praefix ausdruecklich auch `normalised == strings.ToLower(ProxySecretHeader)`, faellt also demselben Scan zum Opfer wie die Identitaetskopfzeilen. Die Behaup… |
| T-10-12 | Ein ungeprueftes JWT als vermeintliche Verteidigung geparst | geschlossen | mittel | Gefahrene Abfrage: `grep -rn 'X-authentik' --include='*.go' . \| grep -v _test.go` — jeder Treffer ausserhalb der Tests liegt in internal/web/forwardauth.go, und die einzigen ausgelesenen Namen sind username/email/name/groups (:1… |
| T-10-13 | Eine Verweigerung, wo ein Durchfallen hingehoert | geschlossen | mittel | internal/web/forwardauth.go:131-160 enthaelt keinen einzigen Schreibzugriff auf w — kein WriteHeader, kein Write — und ruft next.ServeHTTP genau einmal an :158 auf jedem Pfad. Der Zaehler steht in der gemeinsamen Testhilfe (forwa… |
| T-10-14 | Ein leer konfiguriertes Geheimnis passt auf eine leere Kopfzeile | geschlossen | hoch | Zwei unabhaengige Waechter, beide gelesen. Erstens internal/web/forwardauth.go:135, `opts.Secret != ""` als Term derselben kurzschliessenden Kette, links von secretMatches — noetig, weil ConstantTimeCompare zweier leerer Slices 1… |
| T-10-15 | Session-Fixierung ueber die Forward-Auth-Anmeldung | geschlossen | hoch | Minderung: internal/admin/forwardauth.go:267 `h.sm.RenewToken(r.Context())`, vor jedem Put (:276) und vor completeLogin (:277); ein Fehlschlag der Rotation ist ein verweigerter Login (:268-270), keine Anmeldung auf dem alten Toke… |
| T-10-16 | Case-Folding-Fehlgriff trifft das falsche Konto | geschlossen | hoch | Minderung: internal/admin/forwardauth.go:154 (strings.ToLower/TrimSpace), :160-164 (Verweigerung bei !isASCII), :635-642 (isASCII, byteweise < 0x80). Schema-Grundlage geprueft: internal/db/migrations/00001_initial.sql:5 `email TE… |
| T-10-17 | SSO-Anmeldung unsichtbar im Aktivitaetsprotokoll | geschlossen | mittel | Minderung: internal/admin/forwardauth.go:277 ruft h.completeLogin; die Funktion selbst steht in internal/admin/login.go:106-125 und schreibt Sitzungsschluessel, RecordLogin und die auth.login_success-Zeile. Aufrufstellen mit grep… |
| T-10-18 | Verweigerter SSO-Versuch ohne Spur | geschlossen | mittel | Minderung: internal/admin/forwardauth.go:615-628 (refuseSSO): ein slog.Warn mit Kennung, Grundcode und Client-Adresse (:620-621) und eine activity.ActionAuthLoginFail-Zeile mit der versuchten Adresse (:623-627) — dieselbe Form wi… |
| T-10-19 | Verweigerung antwortet 403 statt durchzufallen | **teilweise** | mittel | Minderung: internal/admin/forwardauth.go schreibt in ForwardAuthSignIn auf keinem Pfad einen Status; das Plan-Tor `sed -n '/func (h \*Handler) ForwardAuthSignIn/,/^}/p' internal/admin/forwardauth.go \| grep -c 'http.Error\\|Statu… |
| T-10-20 | Forward-Auth fuettert die Anmeldebremse | **teilweise** | mittel | Minderung ist eine Abwesenheit und sie ist real: `grep -c 'loginThrottle' internal/admin/forwardauth.go` -> 0. Regel-3-Zaehlung ueber alle Orte, an denen die Bremse steht: internal/admin/login.go:33,34,52,71,77 und internal/admin… |
| T-10-21 | Bestehende Sitzung von einer Identitaet ueberschrieben | geschlossen | hoch | Minderung: internal/admin/forwardauth.go:133-136 — eine Anfrage, deren Sitzung bereits auth.SessionKeyUserID traegt, wird unveraendert durchgereicht. Gefahren: `go test ./internal/admin/ -run TestForwardAuthNeverOverwritesASigned… |
| T-10-22 | Bereitgestelltes Konto mit null Zeilen in user_websites | geschlossen | hoch | Minderung dreiteilig, alle drei im Code nachgewiesen: (1) internal/admin/forwardauth.go:546-547 schreibt SetRights mit genau einer Website in derselben Funktion, die Create aufruft; (2) :553-559 der kompensierende Delete, wenn Se… |
| T-10-23 | Bereitstellung erzeugt einen Administrator | geschlossen | hoch | Minderung: internal/admin/forwardauth.go:525 `h.users.Create(ctx, ident.Name, email, secret, user.RoleEditor)`. Gefahren: `go test ./internal/admin/ -run 'TestForwardAuthProvisionsAnAccount\|TestProvisionedAccountCannotReachASeco… |
| T-10-24 | Bereitgestelltes Konto per Passwort erreichbar | **teilweise** | hoch | Minderung im Code vorhanden: internal/admin/forwardauth.go:512 `secret, err := randomSecret()`, :595-601 (32 Byte aus crypto/rand, base64 raw-URL, 43 Zeichen), an Create uebergeben und nirgends behalten. Gefahren: `go test ./inte… |
| T-10-25 | Das Zufallsgeheimnis in einer Logzeile | geschlossen | hoch | Minderung: internal/admin/forwardauth.go:569-571 — die slog.Info-Zeile der Bereitstellung nennt user_id, email, website_id und username, nie das Geheimnis; :512 haelt es in einer lokalen Variablen, die nur an Create geht. Gefahre… |
| T-10-26 | Wettlauf erzeugt zwei Konten fuer eine Identitaet | geschlossen | niedrig | Minderung: internal/admin/forwardauth.go:526-538 — user.ErrDuplicateEmail wird gefangen und die Zeile neu gelesen; ein Nachlesen ohne Treffer ist ein Fehler (:534-536). Die Datenbank ist der Schiedsrichter: internal/db/migrations… |
| T-10-27 | Unkontrollierte Kontoerzeugung durch den Ausweisdienst | geschlossen | mittel | Disposition ist 'accept' und die Annahme ist tatsaechlich belegt: die Bereitstellung ist standardmaessig aus (internal/config/config.go:294 `envBool(envSSOProvision, false, ...)`) und der Zustand ist getestet — `go test ./interna… |
| T-10-28 | Ein Teilstring-Treffer auf dem Gruppen-Header | geschlossen | hoch | Minderung: internal/admin/forwardauth.go:323 `if h.cfg.SSOAdminGroup != "" && ident.HasGroup(h.cfg.SSOAdminGroup)`; HasGroup vergleicht ganze Elemente, internal/web/forwardauth.go:90-97. Gefahren auf einer Kopie von HEAD: `go tes… |
| T-10-29 | Ein Redakteur verliert jede Website-Gruppe und gewinnt alle | **teilweise** | hoch | Minderung: internal/admin/forwardauth.go:413-431, `if len(ids) == 0 { return "", errSSONoWebsiteGroup }` — es wird nichts geschrieben, der Aufrufer verweigert bei :246-262. Gefahren: `go test -count=1 -run 'TestSyncRightsRefusesA… |
| T-10-30 | Eine leer konfigurierte Administrationsgruppe trifft ein leeres Header-Element | geschlossen | hoch | Drei Schichten, alle im Code nachgewiesen: (1) der Term `h.cfg.SSOAdminGroup != ""` in internal/admin/forwardauth.go:323; (2) splitGroups verwirft leere Elemente, internal/web/forwardauth.go:221-229; (3) kein Default — internal/c… |
| T-10-31 | Eine veraltete Rolle ueberlebt eine Degradierung | geschlossen | hoch | Zwei Haelften, beide nachgemessen. (a) Die Synchronisation laeuft bei JEDER Anmeldung und VOR completeLogin: internal/admin/forwardauth.go:246 steht vor :277, SetRights ersetzt vollstaendig (internal/user/rights.go:101-119), also… |
| T-10-32 | Eine Rechteaenderung ohne Aufzeichnung | **teilweise** | mittel | Die Zeile wird geschrieben: internal/admin/forwardauth.go:348-356 (Rolle) und :451-459 (Websites), je nur bei einer echten Aenderung, mit from/to/via:sso. Gefahren: `go test -count=1 -run 'TestSyncRightsPromotionWritesOneProtocol… |
| T-10-33 | Der letzte Administrator wird durch eine Gruppenaenderung degradiert | geschlossen | mittel | Minderung: internal/admin/forwardauth.go:330-340 — `case errors.Is(err, user.ErrLastAdmin)` protokolliert auf WARN mit user_id, email, username, role, requested_role und setzt `want = u.Role`, die Anmeldung laeuft weiter. Gefahre… |
| T-10-34 | may_publish wird bei jeder Anmeldung zurueckgesetzt | geschlossen | mittel | Minderung: internal/admin/forwardauth.go:444-445, `user.Rights{MayPublish: current.MayPublish, Websites: ids}` — der Wert wird bei :434 gelesen und durchgereicht. Gefahren: `go test -count=1 -run TestSyncRightsCarriesThePublishin… |
| T-10-35 | Ein Administrator ganz ohne zweiten Faktor (Transfer an den Identity Provider) | geschlossen | hoch | Ein Transfer hat keine Minderung im Code; pruefbar sind nur die Bedingungen, die die Zeile an ihn haengt. Alle drei existieren: deploy/DEPLOY.md:323-335, Abschnitt "The second factor is whatever your Authentik enforces"; cmd/holz… |
| T-10-36 | Der via_sso-Merker wird von etwas anderem als der Anmeldung gesetzt | **teilweise** | hoch | Der Zustand stimmt: `grep -rn "SessionKeyViaSSO" --include="*.go"` ueber den ganzen Baum findet genau einen Schreiber, internal/admin/forwardauth.go:271, und drei Leser (internal/auth/twofactor.go:90, internal/admin/twofactor.go:… |
| T-10-37 | Ein per Passwort angemeldeter Administrator entkommt der Pflicht | geschlossen | hoch | Praedikat: internal/auth/twofactor.go:60-62, `return role == "admin" && !viaSSO`. Fuenf Aufrufstellen, per grep gemessen (die Plaene nannten vier bzw. eine): internal/auth/twofactor.go:91 und internal/admin/twofactor.go:174, :204… |
| T-10-38 | Die Abhaengigkeit lebt nur in einem Quelltextkommentar | geschlossen | niedrig | Beide Bildschirme tragen den Satz und beide Saetze stehen in {{t}}: cmd/holzcloud/templates/admin/account.html:36-40 hinter {{if .ViaSSO}} (gespeist aus internal/admin/twofactor.go:416 h.viaSSO(r), also derselbe Sitzungsschluesse… |
| T-10-39 | Abmelden lässt das Keks des Identitätsanbieters stehen (Spoofing) | geschlossen | hoch | internal/admin/login.go:159-175 — `target := "/admin/login"`, danach der Zweig `if h.cfg != nil && h.cfg.SSOEnabled && h.sm.GetBool(r.Context(), auth.SessionKeyViaSSO) { target = h.cfg.SSOSignOutPath }`. Gefahren: `go test -count… |
| T-10-40 | Offene Weiterleitung aus der Verwaltung heraus (Tampering) | geschlossen | hoch | Zwei Ebenen, beide gelesen. internal/admin/login.go:174 setzt das Ziel ausschliesslich aus `h.cfg.SSOSignOutPath`; das Tor des Plans, gefahren auf dem Baum: `sed -n '/func (h \*Handler) HandleLogout/,/^}/p' internal/admin/login.g… |
| T-10-41 | Stilles Scheitern an `form-action 'self'` (Denial of Service) | geschlossen | mittel | internal/web/headers.go:17 trägt `form-action 'self'` in adminCSP; `git diff -- internal/web/headers.go \| wc -l` → 0, die Datei ist wie versprochen unangetastet. Die Prämisse hält ein Test, nicht ein Kommentar: internal/admin/lo… |
| T-10-42 | Abmeldung ohne Eintrag im Protokoll (Repudiation) | geschlossen | niedrig | internal/admin/login.go:145-146 — die Protokollzeile steht unverändert vor `sm.Destroy` (login.go:177). Test TestLogoutWritesTheProtocolRowOnBothPaths (login_test.go:272) PASS. |
| T-10-43 | Sitzung überlebt eine gescheiterte Weiterleitung (Information Disclosure) | geschlossen | mittel | internal/admin/login.go:177-179 — `if err := h.sm.Destroy(r.Context()); err != nil { return err }` steht auf beiden Wegen vor `h.redirect` (login.go:190). Reihenfolge gemessen mit dem Tor des Plans: im Funktionsausschnitt steht S… |
| T-10-44 | CVE-2026-30851 auf dem Caddy des Betreibers (Spoofing) | **teilweise** | hoch | Drei Schichten, eine davon gemessen. Schicht 3 im CMS: internal/web/forwardauth.go:180-193 `stripIdentityHeaders` scannt die Schlüssel der Kopfzeilentabelle statt einer Namensliste, isIdentityHeader (forwardauth.go:194-198) falte… |
| T-10-45 | Das gemeinsame Geheimnis im Caddyfile (Information Disclosure) | **teilweise** | hoch | deploy/Caddyfile.example:142 — `header_up X-Holzcloud-Proxy-Secret {env.HOLZCLOUD_SSO_SECRET}`, `grep -c HOLZCLOUD_SSO_SECRET deploy/Caddyfile.example` → 1, eine Referenz und kein Wert, wie behauptet. Ergänzend gelesen: internal/… |
| T-10-46 | Abnahmetest, der aus dem falschen Grund besteht (Repudiation) | geschlossen | hoch | deploy/DEPLOY.md:296-312 gelesen: die eine curl-Zeile mit `-H 'X-authentik-email: stranger@example.com'`, die erwartete Antwort 303 auf …/admin/login, und in Zeile 305 fett '`Connection refused` is not a pass' samt der Anleitung,… |
| T-10-47 | Ein Container, der nach dem Aktualisieren nichts mehr beantwortet (Denial of Service) | geschlossen | mittel | deploy/DEPLOY.md:53 und :61 nennen `HOLZCLOUD_LISTEN=0.0.0.0` für den Containerfall, CHANGELOG.md:38 nennt es ebenfalls. Der Standard ist im Quelltext belegt: internal/config/config.go:175 `const defaultListen = "127.0.0.1"`, ges… |
| T-10-48 | Veralteter Querverweis auf eine Funktion, die es nicht gibt (Tampering) | geschlossen | niedrig | `grep -rn 'isTrustedProxy' --exclude-dir=.planning --exclude-dir=.git .` → 0 Treffer im ganzen Baum. An seiner Stelle steht der richtige Verweis: deploy/Caddyfile.example:167 nennt `HOLZCLOUD_TRUSTED_PROXIES, the CIDR list read b… |
| T-10-49 | Die Abhängigkeit vom zweiten Faktor nur an einer Stelle dokumentiert (Repudiation) | geschlossen | mittel | Beide geforderten Hälften gelesen. Betreiberhälfte: deploy/DEPLOY.md:335-347, Abschnitt 'The second factor is whatever your Authentik enforces', samt dem Satz, dass die Zusage damit beim Identitätsanbieter liegt. Verwaltungshälft… |
| T-10-50 | Sicherheitsaussage nur in einer Sprache lesbar | **teilweise** | mittel | Die Minderung für die zwei Sätze hält. account.html:38 und user_list.html:10 tragen je einen {{t}}-Satz. Beide Schlüssel stehen in en/es/fr/it und sind sinngemäss übersetzt, gelesen. Sie sagen 'wird dort verlangt / hier nicht noc… |
| T-10-51 | Kopierter deutscher Wert gilt als übersetzt | **teilweise** | mittel | Heute gemessen: Scan-Werte (gleich, Länge >40) en/es/fr/it = 0/0/0/0. Wert gleich Schlüssel insgesamt: en 28, es 9, fr 16, it 15, übereinstimmend mit 3548f8f. Alle acht neuen Werte gelesen, keiner ist Deutsch. Für 10-09 selbst is… |
| T-10-52 | Von Hand bearbeiteter Regionalkatalog | **teilweise** | niedrig | `git log` auf fr-CH.json und it-CH.json: letzte Änderung 7025582 (09-06), in Phase 10 unberührt. de-CH wurde in 46e0722 per -schweiz neu erzeugt, und -schweiz ist idempotent (M0). Für de-CH ist die Absicherung stehend (M5). Für f… |
| T-10-53 | Browserdurchgang vor der Review-Fix-Runde abgezeichnet | **offen** | hoch | Die Vorbedingung verlangt committete Review-Fix-Commits. Gemessen: `find .planning -iname '*REVIEW*'` findet für Phase 10 nichts. Die 10-10-SUMMARY (Zeile 248) sagt 'There is none'. Die Commit-Nachricht von 480f21b sagt, Phase 10… |
| T-10-54 | Stand-in als das Echte beschrieben | geschlossen | mittel | Die 10-10-SUMMARY sagt in den Zeilen 274-282, bevor sie beschreibt, was der Stand-in leistet, was er nicht abdeckt: die Anmeldung des Outposts selbst, copy_headers bzw. die request_header-Deletes, CVE-2026-30851 und das Löschen d… |
| T-10-55 | Durchgang testet nur die harmlose Richtung | geschlossen | hoch | Code: internal/admin/forwardauth.go. :246-262 syncRightsFromGroups vor RenewToken, ein Fehler ergibt refuseSSO plus Passwortformular. :327-357 Rollenänderung in beide Richtungen. :413-432 kein passender Website-Eintrag ergibt err… |
| T-10-56 | Kriterium-2-Befehl besteht auf verweigerter Verbindung | geschlossen | hoch | Heute nachgemessen: frischer Build von 480f21b, HOLZCLOUD_LISTEN=0.0.0.0, Port 18431, SSO an. Das Startlog zeigt listen 0.0.0.0 und trusted_proxies 127.0.0.1/32, das Secret steht nicht im Log. Ergebnisse: Plan-Befehl von 192.168.… |
| T-10-57 | Durchgang gegen echte Daten / Rückstände | geschlossen | mittel | Echtes Datenverzeichnis /Users/holz/Projects/holzcloud-cms/data/holzcloud.sqlite: mtime 2026-09-04 18:51, vor dem Durchgang vom 08./09.09. Read-only mit immutable=1 gelesen: 0 Zeilen in users, 0 Zeilen in activity_log, kein Konto… |
| T-10-58 | Kriterium 6 wird zur Checkliste | **teilweise** | hoch | Die Struktur ist vollständig. `git show 480f21b:…/10-10-SUMMARY.md`: Aufgabe 1 hat die Schritte 1-11, Aufgabe 2 die Schritte 1-10, Aufgabe 3 die Schritte 1-18 (4/5/6 unter einer Überschrift), alle 39 benannt, mit dem Gesehenen in… |
| T-10-SC | Installationen über npm/pip/cargo/go (Tampering) — geprüft aus Plan 10-08 heraus | geschlossen | niedrig | Geprüft aus 10-08, weil dort die stärkere Behauptung steht ('no Go file is touched and go.mod is untouched'), und sie deckt 10-07 mit ab. Gemessen: `git log -1 --format='%h %ad %s' --date=short -- go.mod` → 0e6d7af 2026-09-03 'Ho… |

## Unregistrierte Flächen

39 Einträge aus fünf Stapeln, nach Schwere sortiert. Überschneidungen
zwischen Stapeln sind stehen gelassen: dass zwei Prüfer unabhängig dieselbe
Fläche fanden, ist selbst ein Beleg. Die mit „geschlossen" markierten
Hauptbefunde oben stammen zum Teil von hier.

| Stapel | Fläche | Schwere | Datei |
|---|---|---|---|
| T-1-14 | Eine Identitaets-Kopfzeile mit MEHREREN Werten unter demselben kanonischen Schluessel. Keine Zeile des Registers spricht ueber Vielfachheit — T-10-08 und T-10-09 sprechen nur ueber Schreibweisen (Bin… | hoch | internal/web/forwardauth.go:139-153 |
| T-1-14 | HOLZCLOUD_TRUSTED_PROXIES hat keine Plausibilitaetspruefung und keine Wechselwirkung mit HOLZCLOUD_SSO_ENABLED. parsePrefixes (internal/config/config.go:453-470) akzeptiert jedes gueltige CIDR, einsc… | hoch | internal/config/config.go:259 und :453-470 |
| T-15-27 | Das Loeschen der Standard-Website macht jeden bereitgestellten Redakteur zum Redakteur ALLER Websites | hoch | internal/domain/store.go:236 (Loeschung ohne Pruefung), internal/db/migrations/… |
| T-28-38 | SetRights ist nicht transaktional: ein Fehler zwischen DELETE und INSERT hinterlaesst eine LEERE Zuordnung — und leer heisst "jede Website" | hoch | internal/user/rights.go:87-121 zusammen mit internal/admin/forwardauth.go:439-4… |
| T-28-38 | Ein degradierter Administrator endet als Redakteur MIT Zugriff auf jede Website — die Rollenschreibung geschieht vor der Verweigerung und wird nicht zurueckgerollt | hoch | internal/admin/forwardauth.go:327-358 vs. :413-431 |
| T-28-38 | VORBESTEHEND, nicht von diesen Plaenen erzeugt: user_websites.website_id ist ON DELETE CASCADE, das Loeschen einer Website leert Zuordnungen — und leer heisst jede Website | hoch | internal/db/migrations/00033_user_rights.sql:29-30 |
| T-1-14 | Keine Mindestlaenge, keine Entropieforderung und keine Ratenbegrenzung fuer HOLZCLOUD_SSO_SECRET. config.go:292 nimmt nach TrimSpace jede nichtleere Zeichenkette; die einzige Pruefung ist != "" (conf… | mittel | internal/config/config.go:292 und :307; internal/web/forwardauth.go:135, :211-2… |
| T-15-27 | Eine Anmeldung ueber den Proxy uebergeht einen bestaetigten zweiten Faktor des CMS-Kontos | mittel | internal/admin/forwardauth.go:277 gegen internal/admin/login.go:87-95; internal… |
| T-15-27 | Ungebremste Verweigerungen lassen das Aktivitaetsprotokoll unbegrenzt wachsen | mittel | internal/admin/forwardauth.go:623-627 (eine Zeile pro Verweigerung), internal/a… |
| T-28-38 | Die Website-IDs in HOLZCLOUD_SSO_WEBSITE_GROUPS werden nie gegen die Tabelle websites geprueft — anders als HOLZCLOUD_SSO_DEFAULT_WEBSITE | mittel | internal/config/config.go:370-395 und cmd/holzcloud/main.go:658-667 |
| T-28-38 | via_sso wird von den Zwei-Faktor-Lesern nie gegen cfg.SSOEnabled geprueft — der Abmelde-Leser tut es | mittel | internal/auth/twofactor.go:90 und internal/admin/twofactor.go:61, gegen interna… |
| T-28-38 | Ein im CMS eingerichteter zweiter Faktor wird auf dem SSO-Weg nie abgefragt | mittel | internal/admin/forwardauth.go:277 gegen internal/admin/login.go:87-92 |
| T-28-38 | Eine Zeile auth.login_fail pro verweigerter ANFRAGE, auf einer Middleware, die unter /admin/ auf allem sitzt, absichtlich ohne Drossel | mittel | internal/admin/forwardauth.go:615-628, montiert in cmd/holzcloud/main.go:1127 |
| T-28-38 | Die SSO-Rechtezeilen tragen user_id NULL und sind ueber den Benutzerfilter des Protokolls nicht auffindbar | mittel | internal/admin/activity.go:219-223 und internal/activity/store.go:186-191 |
| T-28-38 | Ein im Admin von Hand entzogener Website-Zugang wird bei der naechsten SSO-Anmeldung wieder hergestellt, und kein Bildschirm sagt das | mittel | internal/admin/forwardauth.go:439-447 und cmd/holzcloud/templates/admin/user_li… |
| T-39-49 | Der ausgelieferte `(holzcloud-sso)`-Block stellt die GESAMTE Domain hinter `forward_auth`, nicht nur `/admin` | mittel | deploy/Caddyfile.example:70-143 |
| T-39-49 | `{env.HOLZCLOUD_SSO_SECRET}` wird referenziert, aber nirgends steht, wie die Variable in Caddys Umgebung kommt | mittel | deploy/holzcloud.service:19-48 und deploy/DEPLOY.md:200-212 |
| T-39-49 | Keine einzige Absicherung hält den Inhalt von `deploy/` und `docs/` fest | mittel | deploy/Caddyfile.example, deploy/DEPLOY.md (kein Test referenziert sie) |
| T-39-49 | `HOLZCLOUD_SSO_SECRET` kennt nur 'leer' und 'nicht leer' — keine Mindestlänge, keine Erzeugungsanleitung | mittel | internal/config/config.go:307-311 |
| T-39-49 | `form-action 'self'` in der Verwaltungsrichtlinie hat im ganzen Baum genau eine Absicherung — und die gehört einem anderen Thema | mittel | internal/web/headers.go:17 |
| T-39-49 | Die Ablehnung eines nicht-lokalen Abmeldeziels wird im eigenen Paket von nichts geprüft | mittel | internal/config/config_test.go (keine Ablehnungsprüfung für HOLZCLOUD_SSO_SIGN_… |
| T-39-49 | Kein Startprüfer und kein Dokumentationssatz für den Fall 'Einmalanmeldung im CMS an, Outpost-Block im Caddyfile nicht' | mittel | deploy/Caddyfile.example:60-152, deploy/DEPLOY.md:296-312 |
| T-50-58 | Sicherheitsrelevante Flash-Meldung an einer Phase-10-Aufrufstelle, für den i18n-Sammler unsichtbar | mittel | internal/admin/twofactor.go:282 |
| T-50-58 | Sinn der SSO-07-Übersetzungen hat keine Absicherung | mittel | internal/admin/twofactor_sso_test.go:247 |
| T-50-58 | Nach der Abzeichnung geänderter, gefahrener Bildschirm ohne erneuten Durchgang, dazu veraltete Zahlen in der Abschluss-Summary | mittel | internal/admin/field.go |
| T-1-14 | HOLZCLOUD_SSO_WEBSITE_GROUPS wird beim Start NICHT gegen die Datenbank geprueft, obwohl HOLZCLOUD_SSO_DEFAULT_WEBSITE genau dafuer eine eigene Startverweigerung bekommen hat. config.go:388 verlangt n… | niedrig | internal/config/config.go:377-395; cmd/holzcloud/main.go:654-669 |
| T-1-14 | ForwardAuth, RequestID und AccessLog liegen AUSSERHALB des Recoverers. cmd/holzcloud/main.go:1238-1241: handler = Recoverer(handler), danach AccessLog, danach RequestID, danach ForwardAuth ganz ausse… | niedrig | cmd/holzcloud/main.go:1238-1241 |
| T-1-14 | Der dritte Weg in den Zustand "null Zeilen in user_websites", den T-10-01 als die eigentliche Gefahr benennt und fuer den es zwei Verweigerungen gibt: das LOESCHEN einer Website. user_websites.websit… | niedrig | internal/db/migrations/00033_user_rights.sql:28-32; internal/admin/handler.go:1… |
| T-15-27 | Ein misslungenes RenewToken verweigert stumm — ohne refuseSSO, ohne Protokollzeile | niedrig | internal/admin/forwardauth.go:267-271 |
| T-15-27 | Die Protokollzeile einer Verweigerung traegt den Grund nicht | niedrig | internal/admin/forwardauth.go:623-627 |
| T-15-27 | Kein Abbau: ein durch SSO erzeugtes Konto ueberlebt die Identitaet, die es erzeugt hat | niedrig | internal/admin/forwardauth.go:488-588 (kein Gegenstueck), deploy/DEPLOY.md:217-… |
| T-15-27 | METHODENWARNUNG, keine Codeluecke: in diesem Arbeitsverzeichnis lief waehrend der Pruefung ein zweiter Agent und setzte den Baum zurueck | niedrig | (Arbeitsverzeichnis /Users/holz/Projects/holzcloud-cms) |
| T-39-49 | `isLocalPath` prüft nur das Präfix — CRLF, Tabulator und ein `?rd=`-Ziel kommen durch | niedrig | internal/config/config.go:346-352 |
| T-39-49 | Die Protokollzeile der Abmeldung unterscheidet nicht, welcher Weg gegangen wurde | niedrig | internal/admin/login.go:146 |
| T-50-58 | Kopie-Scan nur einmalig im Plan, nicht in Test oder CI | niedrig | internal/i18n/catalog_test.go:28 |
| T-50-58 | fr-CH.json und it-CH.json: formgerechte Handänderung unentdeckbar | niedrig | internal/i18n/locales/fr-CH.json |
| T-50-58 | Netzwerkadressen des Entwicklungsrechners in committeter Planung | niedrig | .planning/phases/10-authentik/10-10-SUMMARY.md:453 |
| T-50-58 | DEPLOY-Abnahmetest verlangt HOLZCLOUD_LISTEN=0.0.0.0 'vorübergehend', ohne Prüfung des Zurückstellens | niedrig | deploy/DEPLOY.md:305 |
| T-50-58 | Browserbelege nicht mehr nachmessbar | niedrig | .planning/phases/10-authentik/10-10-SUMMARY.md:1250 |

## Widerlegt

Gegenproben haben vier Befunde des ersten Laufs zu Fall gebracht; sie stehen
hier, damit sie nicht wieder auftauchen:

- **Deutsche Musterinhalte in `internal/template/sample.go` erreichen den
  Betreiber** — nein: `check.go:277` rendert nach `io.Discard`.
- **`shop.ErrNotFound` gelangt über `err.Error()` auf eine Oberfläche** —
  nein: alle drei Erzeugungsstellen und alle Aufrufer aufgezählt, keiner druckt
  den Text.
- **Für den Browserdurchgang von Kriterium 6 gibt es kein Protokoll** — doch,
  in `10-10-SUMMARY.md`; der Prüfer hat es übersehen.
- **Ein Verweigerungszweig sei ungemessen** — ein Werkzeugbefund über den
  verdorbenen ersten Lauf, kein Codebefund; die Gegenprobe hat die Messung in
  einem eigenen Worktree nachgeholt.
