-- +goose Up

-- Der Postausgang.
--
-- Eine Nachricht wird in einer Anfrage eingereiht und später von einem Auftrag
-- verschickt. Nicht direkt aus der Anfrage heraus: ein Mailserver, der zehn
-- Sekunden zum Antworten braucht, wären zehn Sekunden, die ein Besucher auf
-- seine Bestätigung wartet — und ein Mailserver, der gar nicht antwortet, wäre
-- eine Anfrage, die scheitert, obwohl die Nachricht längst gespeichert ist.
--
-- In der Datenbank und nicht im Arbeitsspeicher: ein Neustart mitten in einer
-- Zustellung darf keine Einladung verschlucken. Der Preis ist eine Zeile pro
-- Nachricht, die nach der Zustellung eine Weile stehen bleibt.
CREATE TABLE mail_outbox (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    -- website_id ist NULL für alles, was nicht zu einer Website gehört:
    -- Einladungen und Passwort-Links betreffen den Server, nicht eine Seite.
    website_id INTEGER REFERENCES websites(id) ON DELETE CASCADE,
    recipient  TEXT NOT NULL,
    subject    TEXT NOT NULL,
    body       TEXT NOT NULL,
    reply_to   TEXT NOT NULL DEFAULT '',

    -- versuche zählt, wie oft es schon schiefging. Danach richtet sich, wann
    -- der nächste Versuch fällig ist, und ab wann aufgegeben wird.
    attempts   INTEGER NOT NULL DEFAULT 0,
    -- next_try ist der frühestmögliche nächste Versuch. Beim Einreihen ist das
    -- jetzt; nach einem Fehlschlag rückt er nach hinten.
    next_try   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_error TEXT NOT NULL DEFAULT '',
    -- sent_at bleibt NULL, bis es geklappt hat. Zugestellte Zeilen bleiben eine
    -- Weile stehen: "ist die Einladung rausgegangen?" ist die erste Frage, die
    -- jemand stellt, und ohne diese Spur gibt es darauf keine Antwort.
    sent_at    TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
) STRICT;

-- Der Auftrag fragt immer dasselbe: was ist offen und fällig?
CREATE INDEX idx_mail_outbox_due ON mail_outbox(next_try) WHERE sent_at IS NULL;

-- Wohin Benachrichtigungen dieser Website gehen.
--
-- Getrennt von contact_email: die steht öffentlich neben dem Formular, damit
-- ein Besucher auch direkt schreiben kann. Wer die Anfragen liest, ist nicht
-- zwingend dieselbe Adresse — und eine Adresse, die auf der Website steht,
-- bekommt Spam, den man nicht auch noch weitergeleitet haben will.
ALTER TABLE websites ADD COLUMN notify_email TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE websites DROP COLUMN notify_email;
DROP INDEX IF EXISTS idx_mail_outbox_due;
DROP TABLE IF EXISTS mail_outbox;
