## Server-Protokoll

Beim Aufruf einer Seite schreibt der Server eine Zeile in sein Protokoll: Zeitpunkt, aufgerufene Adresse sowie Status und Größe der Antwort. Deine IP-Adresse wird vor dem Schreiben gekürzt, bei IPv4 auf das Netz /{ipv4} und bei IPv6 auf das Netz /{ipv6}: Der Rest der Adresse wird auf null gesetzt und nicht gespeichert. Das Protokoll wird {days} Tage aufbewahrt. Gelöscht wird tageweise, weil das System-Journal täglich eine neue Datei beginnt und nur ganze Dateien entfernt. Zweck ist der Betrieb und die Sicherheit der Website.

Meldet sich jemand im Verwaltungsbereich unter /admin falsch an oder wird eine Anmeldung über Single Sign-on abgelehnt, schreibt der Server die vollständige IP-Adresse in das System-Protokoll. Sie wird ebenso lange aufbewahrt und betrifft nur Personen, die an dieser Website arbeiten.
