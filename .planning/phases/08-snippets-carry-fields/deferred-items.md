# Zurückgestellt — Phase 8

## Aus Plan 08-04

**`07-SECURITY.md:183` nennt „T-07-26 (Plan 05)", die Nummer gehört aber Plan 06.**

Gefunden beim Schliessen von T-07-26. Die Stelle steht in **W-4** („Die
Bezeichnungsmenge ist von 12 auf 32 MiB gewachsen") und redet über die
Massnahme zur Prägung von Bezeichnungen aus Plan 05 — eine andere Bedrohung als
T-07-26, die Information Disclosure aus Plan 06 über `field.Hidden` ist. Die
Nummer dort ist also eine Verwechslung, und sie stand schon vor diesem Plan da.

Nicht angefasst, aus zwei Gründen: Plan 08-04 verlangt ausdrücklich, keinen
anderen Eintrag jenes Dokuments zu ändern, und welche Nummer dort richtig wäre,
liesse sich nur raten — das Register führt für Plan 05 mehrere Kandidaten.

**Folge:** das Zähltor `grep -c 'T-07-26' 07-SECURITY.md` misst nach dem
Schliessen **2** statt der vom Plan erwarteten 1. Eine zu lang, und die
Ursache ist diese Fremdnennung, nicht ein unvollständiges Schliessen.

**Zu tun:** beim nächsten `/gsd-secure-phase 7` die Nummer in W-4 gegen das
Register prüfen und richtigstellen.
