v1.10 — Inhaltsmodell und Zugang

Eine Website beschreibt ihr Inhaltsmodell vollstaendig, Inhalte koennen als
Tabelle ankommen, Alben leben einmal und erscheinen auf vielen Seiten — und wer
die Verwaltung betritt, darf ueber die Anmeldung des Betriebs kommen.

- Feldarten: Auswahl als Knopfreihe, echte Mehrfachauswahl, Schlagwortfeld,
  zeit, bereich und code — in Seiten, Textbausteinen und eigenen Bausteinarten
- Textbausteine tragen jede Feldart
- CSV-Import mit Spaltenzuordnung, Probelauf und Bericht je Zeile
- Anmeldung ueber Authentik als Forward-Auth-Kopfzeile vom Proxy, neben dem
  unveraenderten Passwortweg
- Galerie: Lichtkasten ohne JavaScript, Alben, Diashow

Zwei Hinweise fuer den Betrieb: der Dienst horcht standardmaessig nur noch auf
127.0.0.1 (das Container-Abbild setzt 0.0.0.0 selbst), und eine Sitzung des
Ausweisdienstes erfuellt die Bestaetigung in zwei Schritten.

Der Meilenstein wurde als v1.6 geplant und traegt seit dem Abschluss die
Nummer des Releases. Einzelheiten in CHANGELOG.md und .planning/MILESTONES.md.
