# Sicherheitslücken melden

Bitte melden Sie eine Lücke **nicht** über einen öffentlichen Issue.

Nutzen Sie stattdessen den privaten Kanal von GitHub:
[**Report a vulnerability**](https://github.com/holzcloud/holzcloud-cms/security/advisories/new).
Nur der Betreuer sieht die Meldung, und Sie können darin mitschreiben, bis die
Sache behoben ist.

Wenn Ihnen das nicht möglich ist, öffnen Sie einen Issue mit dem Titel
„Sicherheit — bitte um Kontakt" und **ohne Einzelheiten**; wir vereinbaren dann
einen anderen Weg.

## Was passiert dann

Dies ist ein Projekt, das eine einzelne Person in ihrer Freizeit betreut. Ich
bemühe mich um eine erste Antwort innert einer Woche und um eine Behebung, sobald
ich verstanden habe, was zu tun ist. Ich kann keine Fristen zusichern und zahle
keine Prämien.

Wenn Sie eine Frist für die Veröffentlichung setzen möchten: 90 Tage sind
üblich und für mich in Ordnung. Sagen Sie es einfach in der Meldung.

Sie werden in der Behebung genannt, wenn Sie das möchten — sagen Sie mir, unter
welchem Namen.

## Was zählt als Lücke

Interessant ist alles, was einer Person mehr erlaubt, als ihr zusteht:

- Zugriff auf fremde Websites in einer Installation mit mehreren
- Inhalte, die ohne Anmeldung erreichbar sind, obwohl sie es nicht sein sollten
  — Entwürfe, geschützte Seiten, Bestellungen, hochgeladene Dateien
- Ausführen von Code über eine hochgeladene Vorlage oder eine hochgeladene Datei
- Umgehen der Anmeldung, des zweiten Faktors oder der Rollentrennung
- Skripte, die im Admin-Bereich zur Ausführung kommen
- Manipulation von Preisen, Beständen oder Zahlungsvorgängen

Nicht interessant, auch wenn ein Scanner es anmerkt:

- Fehlende Sicherheits-Kopfzeilen auf `/healthz` oder `/readyz`
- Nutzeraufzählung über die Anmeldemaske — es gibt keine; alle Fehlversuche
  antworten gleich. Wenn Sie das Gegenteil finden, ist es eine Lücke.
- Angaben zur eingesetzten Fassung. Die stehen absichtlich im Startprotokoll.
- Berichte, die nur aus der Ausgabe eines Werkzeugs bestehen, ohne dass jemand
  nachgesehen hat, ob sich damit tatsächlich etwas erreichen lässt

## Was das Projekt von sich aus tut

Damit Sie einschätzen können, was schon geprüft ist:

- Vorlagenarchive werden beim Hochladen abgewiesen, wenn sie JavaScript oder
  Verweise auf fremde Server enthalten, und werden vor der Annahme gerendert
- Die Inhaltsrichtlinie (`Content-Security-Policy`) verbietet auf jeder Antwort
  fremde Quellen; im Admin-Bereich zusätzlich jedes Skript im Dokument
- Passwörter mit Argon2id, zweiter Faktor für Verwaltungskonten zwingend
- CSRF auf allen ändernden Anfragen, auch den über htmx
- Zip-Slip beim Vorlagen-Upload wird verhindert
- Zahlungen gelten erst als eingegangen, wenn der Anbieter das auf Rückfrage
  bestätigt — der Inhalt einer Benachrichtigung wird nie geglaubt
- Mail wird nur über eine verschlüsselte Verbindung verschickt, und
  Kopfzeilenwerte werden von Zeilenumbrüchen befreit

Ein Weg um eines dieser Dinge herum ist genau das, was ich hören möchte.
