-- +goose Up

-- Der Postausgang.
--
-- E-Mail wird nicht im Anfrage-Zyklus verschickt. Ein Mailserver, der drei
-- Sekunden für die Begrüssung braucht, hängt sonst an der Bestellung der
-- Kundin, und einer, der gerade nicht erreichbar ist, verliert die Bestätigung
-- ersatzlos — die Bestellung ist dann in der Welt und niemand weiss davon.
--
-- Stattdessen wird die Nachricht in derselben Sekunde abgelegt, in der die
-- Bestellung entsteht, und ein Hintergrundlauf trägt sie aus. Fällt der Server
-- dazwischen aus, steht sie beim nächsten Start noch da.
CREATE TABLE outbox (
    id INTEGER PRIMARY KEY,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,

    -- Wofür die Nachricht steht, damit der Admin sie einer Bestellung zuordnen
    -- kann: 'order_customer', 'order_operator', 'order_shipped'.
    kind TEXT NOT NULL,

    -- Die Bestellung dazu. Darf leer sein — nicht jede Nachricht gehört zu
    -- einer. Wird die Bestellung gelöscht, geht die Nachricht mit: sie hätte
    -- danach keinen Gegenstand mehr.
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,

    recipient TEXT NOT NULL,
    -- Der Anzeigename des Absenders: der Name des Betriebs, nicht der des
    -- Servers. Eine Installation bedient mehrere Websites, und eine Bestätigung
    -- von "Holzcloud" statt von "Holzbau Schmidt" ist für die Kundin eine
    -- Nachricht von jemandem, bei dem sie nichts bestellt hat.
    from_name TEXT NOT NULL DEFAULT '',
    -- reply_to ist die Adresse des Betriebs, damit eine Antwort der Kundin dort
    -- ankommt und nicht beim Absenderpostfach des Servers.
    reply_to TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL,
    -- Der fertige Text, nicht die Zutaten. Was hier steht, geht raus — und was
    -- rausgegangen ist, kann der Betrieb später nachlesen. Eine Nachricht beim
    -- Versand neu zu bauen hiesse, dass sich der Inhalt zwischen Auslösen und
    -- Zustellen noch ändern kann.
    body TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed')),
    -- attempts zählt die Versuche. Nach genug davon wird aufgegeben, sonst
    -- klopft eine Nachricht an eine falsch geschriebene Adresse bis in alle
    -- Ewigkeit.
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    -- Der frühestmögliche nächste Versuch. Trägt die Wartezeit zwischen den
    -- Versuchen, die sich nach jedem Fehlschlag verdoppelt.
    next_attempt_at TEXT NOT NULL,

    created_at TEXT NOT NULL,
    sent_at TEXT
) STRICT;

-- Der Hintergrundlauf fragt genau danach: was ist offen und schon fällig.
CREATE INDEX idx_outbox_due ON outbox(status, next_attempt_at)
    WHERE status = 'pending';

-- Und die Bestellseite im Admin fragt danach.
CREATE INDEX idx_outbox_order ON outbox(order_id);

-- Wohin die Meldung über eine neue Bestellung geht. Leer heisst: es geht keine
-- raus. Bewusst eine eigene Adresse und nicht die des angemeldeten Kontos —
-- Bestellungen liest oft jemand anderes als der, der die Website pflegt.
ALTER TABLE websites ADD COLUMN order_email TEXT NOT NULL DEFAULT '';

-- Die Zahlungsangaben für Vorauskasse: Kontoinhaber, IBAN, allenfalls ein
-- Hinweis auf die Bestellnummer als Zahlungszweck.
--
-- Bis hierher stand in der Kasse "wir schicken Ihnen die Zahlungsangaben per
-- E-Mail", und es gab weder die Angaben noch die E-Mail. Ein Versprechen, das
-- die Software nicht halten konnte.
ALTER TABLE websites ADD COLUMN payment_details TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE websites DROP COLUMN payment_details;
ALTER TABLE websites DROP COLUMN order_email;
DROP TABLE outbox;
