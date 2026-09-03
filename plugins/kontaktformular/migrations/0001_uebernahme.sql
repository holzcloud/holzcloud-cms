-- Übernimmt die Nachrichten, die der Kern gesammelt hat, bevor das
-- Kontaktformular ein Plugin wurde.
--
-- Der Kern schreibt nicht mehr in form_messages und liest nicht mehr daraus.
-- Ohne diesen Schritt wären die vorhandenen Anfragen nach dem Umstieg zwar noch
-- in der Datenbank, aber nirgends mehr zu sehen — und eine Anfrage, die jemand
-- gestellt hat und die niemand liest, ist eine verlorene Anfrage.
--
-- Kopiert wird, nicht verschoben: die alte Tabelle bleibt unangetastet stehen.
-- Damit läuft dieser Schritt gefahrlos ein zweites Mal, wenn das Plugin einmal
-- entfernt und wieder eingespielt wird, und wer die alten Zeilen noch braucht,
-- findet sie da, wo sie immer waren. Weggeräumt wird die Tabelle erst in einer
-- späteren Fassung des Kerns.
--
-- Der Schlüssel ist derselbe, den das Plugin selbst vergibt: Zeitstempel, dann
-- ein Trennzeichen, dann etwas Eindeutiges. Hier ist das die alte Zeilennummer,
-- die es kein zweites Mal gibt.
INSERT OR IGNORE INTO plugin_store (plugin_id, website_id, key, value, updated_at)
SELECT
    'kontaktformular',
    m.website_id,
    'nachricht:' || m.created_at || '-alt' || m.id,
    json_object(
        'kennung', m.created_at || '-alt' || m.id,
        'name',    m.name,
        'email',   m.email,
        'betreff', m.subject,
        'text',    m.body,
        'seite',   COALESCE((SELECT p.slug FROM pages p WHERE p.id = m.page_id), ''),
        'zeit',    m.created_at,
        'gelesen', json(CASE WHEN m.read_at IS NULL THEN 'false' ELSE 'true' END)
    ),
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM form_messages m;
