-- +goose Up

-- Die neueste Fassung, deren "Neuerungen" dieser Person gezeigt wurden.
--
-- Je Zugang und nicht je Browser oder Sitzung: wer nach einer Aktualisierung
-- einmal gelesen hat, was sich geändert hat, soll es nicht auf dem Handy und
-- dem Laptop und nach jeder neuen Anmeldung ein weiteres Mal vorgesetzt
-- bekommen.
--
-- Leer heisst: noch nichts festgehalten. Das ist ein neuer Zugang, und der hat
-- nichts nachzuholen — er hat die Fassung, die läuft, nie anders gekannt. Der
-- erste Besuch hält deshalb nur die laufende Fassung fest und zeigt nichts.
--
-- Zugänge, die es beim Ausführen dieser Migration schon gibt, haben das
-- Programm benutzt, bevor es die Spalte gab. 0.1.2 ist die letzte Fassung ohne
-- sie, also bekommen diese Zugänge die nächste Fassung einmal gezeigt. Ehrlich
-- dazu: war eine Installation noch weiter zurück, stehen alle Fassungen
-- trotzdem in den Reitern, nur die Markierung "neu" beginnt erst nach 0.1.2.
-- Auf einer frischen Installation ist die Tabelle beim Ausführen leer, und
-- niemand ist betroffen.
ALTER TABLE users ADD COLUMN seen_version TEXT NOT NULL DEFAULT '';
UPDATE users SET seen_version = '0.1.2';

-- +goose Down
ALTER TABLE users DROP COLUMN seen_version;
