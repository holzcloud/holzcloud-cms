# Zurückgestellt

> **Erledigt am 2026-09-03 — Schnellaufgabe `260903-zwei-abgelegte-fehler`:** `tools/mkbundle` prüft die Schlagwörter jetzt gegen ihre Namen statt gegen ihre Kennungen (`tools/mkbundle/main.go:184`).

## mkbundle prüft `page.terms` gegen die Slugs statt gegen die Namen

Gefunden beim Lesen von `checkReferences`, während für das Beispiel entschieden
wurde, ob es ein `terms` bekommt. Vorbestehend, nicht von dieser Aufgabe
verursacht, deshalb hier statt im Diff.

- `internal/bundle/export.go:256` schreibt in `page.terms` den **Namen**
  (`out.Terms = append(out.Terms, t.Name)`).
- `internal/bundle/import.go:547` liest sie wieder als Namen
  (`term.Parse(strings.Join(p.Terms, ", "))`).
- `tools/mkbundle/main.go:177` baut die Menge aber aus `t.Slug` und prüft
  `page.terms` dagegen.

Folge: ein exportiertes Bundle mit einem Schlagwort, dessen Name nicht schon
sein eigener Slug ist, lässt sich mit mkbundle nicht bauen — es meldet
`term "Möbel" has no entry in terms`, obwohl der Eintrag da ist. Die Prüfung
müsste über die Namen laufen, oder beide Seiten müssten sich auf den Slug
einigen.

Deshalb kommt im Beispiel-Bundle kein `terms` vor: ein Beispiel wird kopiert,
und eine der beiden möglichen Schreibweisen wäre in jedem Fall die falsche
gewesen. Sobald das entschieden ist, gehört ein Schlagwort hinein.
